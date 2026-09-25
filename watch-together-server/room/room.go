package room

import (
	"sync"
	"time"

	"github.com/sakurajima336/animeko-watch-together/protocol"
)

const (
	// MaxMembers 每个房间的最大成员数,超过返回 ROOM_FULL。
	MaxMembers = 20

	// MaxRoomNameLen / MaxPasswordLen 与客户端校验保持一致。
	MaxRoomNameLen = 32
	MaxPasswordLen = 64

	// memberTimeout 成员心跳超时,超时后标记为 DISCONNECTED。
	memberTimeout = 60 * time.Second
	// memberReapAfter 断开后多久从房间移除。
	memberReapAfter = 5 * time.Minute

	// maxChatHistory 每个房间保留的聊天消息条数。
	maxChatHistory = 200
)

// Member 房间内的一个成员会话。
type Member struct {
	UserID       string
	Nickname     string
	IsHost       bool
	Following    bool
	State        protocol.MemberState
	SessionNonce string
	LastSeenAt   int64
	Watching     *protocol.WatchingInfo
	AvatarURL    *string
}

// Room 一个一起看房间。所有状态由 mu 保护。
type Room struct {
	ID       string
	Name     string
	Password string

	mu           sync.RWMutex
	version      int64
	status       protocol.RoomStatus
	closedReason *string
	members      []*Member
	playback     *protocol.Playback
	chat         []*protocol.ChatMessage
	// subs 订阅 SSE 推送的通道集合。
	subs map[chan []byte]struct{}
}

// NewRoom 创建一个房间,creator 为房主。
func NewRoom(id, name, password string, creator *Member) *Room {
	creator.IsHost = true
	r := &Room{
		ID:       id,
		Name:     name,
		Password: password,
		status:   protocol.RoomStatusOpen,
		members:  []*Member{creator},
		version:  1,
		subs:     make(map[chan []byte]struct{}),
	}
	return r
}

// TryJoin 加入房间。roomName 不存在或密码不符时返回具体错误码。
func (r *Room) TryJoin(password string, m *Member) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status == protocol.RoomStatusClosed {
		return ErrRoomClosed
	}
	if r.Password != password {
		return ErrWrongPassword
	}
	// 同一用户重入:替换旧会话(客户端会认为是 REPLACED)。
	for i, e := range r.members {
		if e.UserID == m.UserID {
			r.members[i] = m
			r.version++
			r.broadcastLocked()
			return nil
		}
	}
	if len(r.members) >= MaxMembers {
		return ErrRoomFull
	}
	r.members = append(r.members, m)
	r.version++
	r.broadcastLocked()
	return nil
}

// Leave 移除指定会话。
func (r *Room) Leave(sessionNonce string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := false
	for i, e := range r.members {
		if e.SessionNonce == sessionNonce {
			r.members = append(r.members[:i], r.members[i+1:]...)
			removed = true
			break
		}
	}
	if !removed {
		return
	}
	r.version++
	// 房主离开后把房主移交给剩下的人,避免房间无人主持。
	if len(r.members) > 0 && !r.hasHostLocked() {
		r.members[0].IsHost = true
	}
	if len(r.members) == 0 {
		r.closeLocked(protocol.MembershipRoomClosed)
	} else {
		r.broadcastLocked()
	}
}

// Report 上报成员状态。房主的上报会更新房间播放状态。
func (r *Room) Report(
	sessionNonce string,
	state protocol.MemberState,
	following bool,
	watching *protocol.WatchingInfo,
) (protocol.Membership, *protocol.RoomSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var self *Member
	for _, e := range r.members {
		if e.SessionNonce == sessionNonce {
			self = e
			break
		}
	}
	if self == nil {
		return protocol.MembershipNotMember, nil
	}
	if r.status == protocol.RoomStatusClosed {
		return protocol.MembershipRoomClosed, r.snapshotLocked()
	}

	self.State = state
	self.Following = following
	self.Watching = watching
	self.LastSeenAt = time.Now().UnixMilli()

	// 房主身份由会话自身决定:只有房主上报的播放状态才会成为房间 playback。
	if self.IsHost && watching != nil {
		r.playback = &protocol.Playback{
			Info:       watching,
			ReportedAt: time.Now().UnixMilli(),
		}
		r.version++
	}
	// 成员状态变化也要让其他人看到(比如谁在播/谁离开)。
	r.version++
	r.broadcastLocked()
	return protocol.MembershipOK, r.snapshotLocked()
}

// SendChat 发送一条聊天消息。非成员不能发言。
func (r *Room) SendChat(sessionNonce, content string) (*protocol.ChatMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	sender := r.findLocked(sessionNonce)
	if sender == nil {
		return nil, ErrNotMember
	}
	if r.status == protocol.RoomStatusClosed {
		return nil, ErrRoomClosed
	}

	msg := &protocol.ChatMessage{
		ID:       newID("m"),
		RoomID:   r.ID,
		UserID:   sender.UserID,
		Nickname: sender.Nickname,
		Content:  content,
		SentAt:   time.Now().UnixMilli(),
	}
	r.chat = append(r.chat, msg)
	if len(r.chat) > maxChatHistory {
		r.chat = r.chat[len(r.chat)-maxChatHistory:]
	}
	r.broadcastChatLocked(msg)
	return msg, nil
}

// AddSystemMessage 追加一条系统消息(如成员加入/离开)。
func (r *Room) AddSystemMessage(content string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	msg := &protocol.ChatMessage{
		ID:       newID("m"),
		RoomID:   r.ID,
		UserID:   "system",
		Nickname: "系统",
		Content:  content,
		SentAt:   time.Now().UnixMilli(),
		System:   true,
	}
	r.chat = append(r.chat, msg)
	if len(r.chat) > maxChatHistory {
		r.chat = r.chat[len(r.chat)-maxChatHistory:]
	}
	r.broadcastChatLocked(msg)
}

// ChatHistory 返回房间聊天历史。
func (r *Room) ChatHistory() []*protocol.ChatMessage {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*protocol.ChatMessage, len(r.chat))
	copy(out, r.chat)
	return out
}

// Snapshot 生成房间快照。
func (r *Room) Snapshot() *protocol.RoomSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.snapshotLocked()
}

// Version 当前版本号。
func (r *Room) Version() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.version
}

// MemberCount 当前成员数。
func (r *Room) MemberCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.members)
}

// IsEmpty 房间是否无人。
func (r *Room) IsEmpty() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.members) == 0
}

// ReapStale 清理超时成员,返回是否应销毁房间。
func (r *Room) ReapStale() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UnixMilli()
	changed := false
	remaining := r.members[:0]
	for _, e := range r.members {
		age := now - e.LastSeenAt
		if age > int64(memberTimeout/time.Millisecond) && e.State != protocol.MemberStateDisconnected {
			e.State = protocol.MemberStateDisconnected
			changed = true
		}
		if age > int64(memberReapAfter/time.Millisecond) {
			changed = true
			continue
		}
		remaining = append(remaining, e)
	}
	r.members = remaining
	if changed {
		r.version++
	}
	if len(r.members) == 0 {
		r.closeLocked(protocol.MembershipRoomClosed)
		return true
	}
	if changed {
		r.broadcastLocked()
	}
	return false
}

func (r *Room) findLocked(sessionNonce string) *Member {
	for _, e := range r.members {
		if e.SessionNonce == sessionNonce {
			return e
		}
	}
	return nil
}

func (r *Room) hasHostLocked() bool {
	for _, e := range r.members {
		if e.IsHost {
			return true
		}
	}
	return false
}

func (r *Room) closeLocked(reason protocol.Membership) {
	if r.status == protocol.RoomStatusClosed {
		return
	}
	r.status = protocol.RoomStatusClosed
	rsn := string(reason)
	r.closedReason = &rsn
	r.version++
	r.broadcastLocked()
}

func (r *Room) snapshotLocked() *protocol.RoomSnapshot {
	members := make([]*protocol.Member, 0, len(r.members))
	host := ""
	for _, e := range r.members {
		if e.IsHost {
			host = e.UserID
		}
		members = append(members, &protocol.Member{
			UserID:     e.UserID,
			Nickname:   e.Nickname,
			IsHost:     e.IsHost,
			Following:  e.Following,
			State:      e.State,
			LastSeenAt: e.LastSeenAt,
			AvatarURL:  e.AvatarURL,
			Watching:   e.Watching,
		})
	}
	if host == "" && len(r.members) > 0 {
		host = r.members[0].UserID
	}
	snap := &protocol.RoomSnapshot{
		RoomID:       r.ID,
		RoomName:     r.Name,
		Version:      r.version,
		Status:       r.status,
		HostUserID:   host,
		ServerTime:   time.Now().UnixMilli(),
		Members:      members,
		ClosedReason: r.closedReason,
	}
	if r.playback != nil {
		cp := *r.playback
		if cp.Info != nil {
			info := *cp.Info
			cp.Info = &info
		}
		snap.Playback = &cp
	}
	return snap
}
