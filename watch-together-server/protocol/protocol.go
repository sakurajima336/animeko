// Package protocol 定义与 Animeko(Ani) 客户端兼容的一起看(Watch Together)协议数据结构。
//
// 这些字段与客户端 openapi 生成的模型一一对应:
//   - AniJoinWatchTogetherRoomRequest
//   - AniWatchTogetherJoinResponse
//   - AniReportWatchTogetherStateRequest
//   - AniWatchTogetherReportResponse
//   - AniLeaveWatchTogetherRoomRequest
//   - AniWatchTogetherRoomSnapshot / Member / Playback / WatchingInfo
//
// 注意: 客户端使用 kotlinx.serialization, 未声明的字段会被忽略, 但已声明且非空的字段必须出现,
// 因此所有 Required 字段都不能使用 omitempty。
package protocol

// MemberState 成员状态。
type MemberState string

const (
	MemberStateIdle         MemberState = "IDLE"
	MemberStateWatching     MemberState = "WATCHING"
	MemberStateDisconnected MemberState = "DISCONNECTED"
)

// RoomStatus 房间状态。
type RoomStatus string

const (
	RoomStatusOpen   RoomStatus = "OPEN"
	RoomStatusClosed RoomStatus = "CLOSED"
)

// Membership 成员资格,用于告知客户端其会话是否仍然有效。
type Membership string

const (
	MembershipOK             Membership = "OK"
	MembershipKickedTimetout Membership = "KICKED_TIMEOUT"
	MembershipReplaced       Membership = "REPLACED"
	MembershipRoomClosed     Membership = "ROOM_CLOSED"
	MembershipNotMember      Membership = "NOT_MEMBER"
)

// JoinRequest POST /v2/watch-together/join
type JoinRequest struct {
	RoomName  string `json:"roomName"`
	Password  string `json:"password"`
	Following *bool  `json:"following,omitempty"`
}

// JoinResponse POST /v2/watch-together/join
type JoinResponse struct {
	RoomID       string        `json:"roomId"`
	Created      bool          `json:"created"`
	IsHost       bool          `json:"isHost"`
	SessionNonce string        `json:"sessionNonce"`
	ServerTime   int64         `json:"serverTime"`
	Snapshot     *RoomSnapshot `json:"snapshot"`
}

// LeaveRequest POST /v2/watch-together/rooms/{roomId}/leave
type LeaveRequest struct {
	SessionNonce string `json:"sessionNonce"`
}

// ReportRequest POST /v2/watch-together/rooms/{roomId}/report
type ReportRequest struct {
	SessionNonce string        `json:"sessionNonce"`
	MemberState  MemberState   `json:"memberState"`
	Following    bool          `json:"following"`
	Watching     *WatchingInfo `json:"watching,omitempty"`
	KnownVersion *int64        `json:"knownVersion,omitempty"`
}

// ReportResponse POST /v2/watch-together/rooms/{roomId}/report
type ReportResponse struct {
	ServerTime int64         `json:"serverTime"`
	Membership Membership    `json:"membership"`
	Version    int64         `json:"version"`
	Snapshot   *RoomSnapshot `json:"snapshot,omitempty"`
}

// WatchingInfo 正在观看的条目信息。
type WatchingInfo struct {
	SubjectID        int     `json:"subjectId"`
	EpisodeID        int     `json:"episodeId"`
	SubjectName      string  `json:"subjectName"`
	EpisodeSort      string  `json:"episodeSort"`
	EpisodeName      string  `json:"episodeName"`
	PositionMillis   int64   `json:"positionMillis"`
	PositionAtMillis int64   `json:"positionAtMillis"`
	DurationMillis   int64   `json:"durationMillis"`
	Paused           bool    `json:"paused"`
	Buffering        bool    `json:"buffering"`
	Loading          bool    `json:"loading"`
	PlaybackRate     float32 `json:"playbackRate"`
}

// Playback 房间内被跟随的播放状态。
type Playback struct {
	Info       *WatchingInfo `json:"info"`
	ReportedAt int64         `json:"reportedAt"`
}

// Member 房间成员。
type Member struct {
	UserID     string        `json:"userId"`
	Nickname   string        `json:"nickname"`
	IsHost     bool          `json:"isHost"`
	Following  bool          `json:"following"`
	State      MemberState   `json:"state"`
	LastSeenAt int64         `json:"lastSeenAt"`
	AvatarURL  *string       `json:"avatarUrl,omitempty"`
	Watching   *WatchingInfo `json:"watching,omitempty"`
}

// RoomSnapshot 房间快照,通过 SSE 的 snapshot 事件与 HTTP 响应下发给客户端。
type RoomSnapshot struct {
	RoomID       string     `json:"roomId"`
	RoomName     string     `json:"roomName"`
	Version      int64      `json:"version"`
	Status       RoomStatus `json:"status"`
	HostUserID   string     `json:"hostUserId"`
	ServerTime   int64      `json:"serverTime"`
	Members      []*Member  `json:"members"`
	ClosedReason *string    `json:"closedReason,omitempty"`
	Playback     *Playback  `json:"playback,omitempty"`
}

// ---- 聊天(本服务端扩展,官方协议未定义,客户端已适配) ----

// ChatMessage 一条房间聊天消息。
type ChatMessage struct {
	ID       string `json:"id"`
	RoomID   string `json:"roomId"`
	UserID   string `json:"userId"`
	Nickname string `json:"nickname"`
	Content  string `json:"content"`
	SentAt   int64  `json:"sentAt"`
	System   bool   `json:"system"`
}

// SendChatRequest POST /v2/watch-together/rooms/{roomId}/chat
type SendChatRequest struct {
	SessionNonce string `json:"sessionNonce"`
	Content      string `json:"content"`
}

// ChatHistoryResponse GET /v2/watch-together/rooms/{roomId}/chat
type ChatHistoryResponse struct {
	ServerTime int64          `json:"serverTime"`
	Messages   []*ChatMessage `json:"messages"`
}

// ChatEvent 通过 SSE 事件 "chat" 下发。
type ChatEvent struct {
	Message *ChatMessage `json:"message"`
}

// ByeEvent SSE 事件 "bye" 的负载。
type ByeEvent struct {
	Reason string `json:"reason"`
}

// ErrorResponse 错误响应体。
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
