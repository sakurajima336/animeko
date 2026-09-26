// Package protocol 定义一起看**聊天扩展**的数据结构。
//
// 本扩展不实现房间生命周期(join/report/leave)——那些由官方服务端负责。
// 这里只保留聊天室相关的类型: 消息、发送请求、历史响应、SSE 事件。
//
// 客户端使用 kotlinx.serialization, 未声明的字段会被忽略,
// 但已声明且必填的字段必须出现。
package protocol

// ChatMessage 一条房间聊天消息。
//
// UserID / Nickname / AvatarURL 都来自官方一起看房间的成员信息, 客户端据此把消息
// 归属到官方成员(昵称与头像与官方房间保持一致)。
type ChatMessage struct {
	ID        string  `json:"id"`
	RoomID    string  `json:"roomId"`
	UserID    string  `json:"userId"`
	Nickname  string  `json:"nickname"`
	AvatarURL *string `json:"avatarUrl,omitempty"`
	Content   string  `json:"content"`
	SentAt    int64   `json:"sentAt"`
	System    bool    `json:"system"`
}

// SendChatRequest POST /v2/watch-together/rooms/{roomId}/chat
//
// sessionNonce 由官方服务端签发, 扩展据此识别参与者。
// sender* 是客户端所在官方房间的成员信息(昵称/头像), 用于让聊天室里的显示与官方房间一致。
type SendChatRequest struct {
	SessionNonce    string `json:"sessionNonce"`
	Content         string `json:"content"`
	SenderUserID    string `json:"senderUserId,omitempty"`
	SenderNickname  string `json:"senderNickname,omitempty"`
	SenderAvatarURL string `json:"senderAvatarUrl,omitempty"`
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

// ErrorResponse 错误响应体。
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
