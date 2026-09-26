package room

import (
	"sync"
	"time"

	"github.com/sakurajima336/animeko-watch-together/protocol"
)

const (
	// maxChatHistory 每个聊天室保留的消息条数。
	maxChatHistory = 200
	// chatRoomTTL 长期无人访问的聊天室自动回收。
	chatRoomTTL = 24 * time.Hour
)

// ChatRoom 一个官方一起看房间对应的聊天室。
//
// 聊天室只以官方下发的 roomId 为键, 不保存"房间名/密码"之类概念 ——
// 房间的权威来源只有官方服务端。
type ChatRoom struct {
	ID string

	mu       sync.RWMutex
	messages []*protocol.ChatMessage
	subs     map[chan []byte]struct{}
	// sessions 记录在该聊天室发言过的参与者会话。
	// 值为展示信息, 用于让同一 sessionNonce 持续显示同一昵称。
	sessions   map[string]participant
	lastActive time.Time
}

type participant struct {
	UserID   string
	Nickname string
	Avatar   *string
}

func NewChatRoom(id string) *ChatRoom {
	return &ChatRoom{
		ID:         id,
		subs:       make(map[chan []byte]struct{}),
		sessions:   make(map[string]participant),
		lastActive: time.Now(),
	}
}

// Sender 一条聊天消息的发言者身份。
//
// Explicit 表示 UserID 是否由客户端**显式**提供(官方成员信息或 token)。
// 客户端未提供任何身份时会回落到按 IP + User-Agent 生成的匿名 ID, 那种情况下
// 不能把它当作"换了个人" —— 否则每次省略都会把该 sessionNonce 已登记的官方成员
// 信息覆盖成匿名值。
type Sender struct {
	UserID   string
	Nickname string
	Avatar   *string
	Explicit bool
}

// SendChatBySession 以官方 sessionNonce 发送一条消息。
//
// sessionNonce 由官方服务端签发, 这里只把它当作参与者标识。
// 昵称与头像取自官方房间成员信息, 由客户端随请求透传:
// 同一 nonce 首次登记后即固定下来, 后续请求缺省时沿用登记值,
// 保证聊天室里的昵称/头像与官方房间成员一致且稳定。
func (r *ChatRoom) SendChatBySession(
	sessionNonce string,
	sender Sender,
	content string,
) (*protocol.ChatMessage, error) {
	if sessionNonce == "" {
		return nil, ErrMissingSession
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastActive = time.Now()

	registered, ok := r.sessions[sessionNonce]
	switch {
	case !ok:
		// 首次发言: 按本次带来的信息登记(此时即便是匿名身份也没有更好的来源)。
		registered = participant{UserID: sender.UserID, Nickname: sender.Nickname, Avatar: sender.Avatar}

	case sender.Explicit && sender.UserID != "" && sender.UserID != registered.UserID:
		// 官方房间重新分配了身份: 以本次请求为准。
		registered = participant{UserID: sender.UserID, Nickname: sender.Nickname, Avatar: sender.Avatar}

	case sender.Explicit:
		// 同一身份, 仅补齐本次带来的昵称/头像。
		if sender.Nickname != "" {
			registered.Nickname = sender.Nickname
		}
		if sender.Avatar != nil {
			registered.Avatar = sender.Avatar
		}

	default:
		// 匿名兜底(客户端没提供任何身份): 完全忽略, 沿用已登记身份。
		// 否则每次省略都会把官方成员信息覆盖成"游客xxxxxx"。
	}
	r.sessions[sessionNonce] = registered

	msg := &protocol.ChatMessage{
		ID:        newID("m"),
		RoomID:    r.ID,
		UserID:    registered.UserID,
		Nickname:  registered.Nickname,
		AvatarURL: registered.Avatar,
		Content:   content,
		SentAt:    time.Now().UnixMilli(),
	}
	r.messages = append(r.messages, msg)
	if len(r.messages) > maxChatHistory {
		r.messages = r.messages[len(r.messages)-maxChatHistory:]
	}
	r.broadcastChatLocked(msg)
	return msg, nil
}

// AddSystemMessage 追加一条系统提示(例如参与者加入)。
func (r *ChatRoom) AddSystemMessage(content string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastActive = time.Now()
	msg := &protocol.ChatMessage{
		ID:       newID("m"),
		RoomID:   r.ID,
		UserID:   "system",
		Nickname: "系统",
		Content:  content,
		SentAt:   time.Now().UnixMilli(),
		System:   true,
	}
	r.messages = append(r.messages, msg)
	if len(r.messages) > maxChatHistory {
		r.messages = r.messages[len(r.messages)-maxChatHistory:]
	}
	r.broadcastChatLocked(msg)
}

// ChatHistory 返回聊天记录副本。
func (r *ChatRoom) ChatHistory() []*protocol.ChatMessage {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*protocol.ChatMessage, len(r.messages))
	copy(out, r.messages)
	return out
}

// MessageCount 当前消息数。
func (r *ChatRoom) MessageCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.messages)
}

// IdleSince 距上次活动的时间。
func (r *ChatRoom) IdleSince() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return time.Since(r.lastActive)
}

// purgeMessages 清空消息但保留聊天室条目(用于回收后仍被引用的场景)。
func (r *ChatRoom) purgeMessages() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = nil
	r.sessions = make(map[string]participant)
}
