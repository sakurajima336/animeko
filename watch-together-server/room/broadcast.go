package room

import (
	"encoding/json"

	"github.com/sakurajima336/animeko-watch-together/protocol"
)

// Subscribe 注册一个 SSE 订阅通道。
func (r *ChatRoom) Subscribe() chan []byte {
	ch := make(chan []byte, 64)
	r.mu.Lock()
	r.subs[ch] = struct{}{}
	r.mu.Unlock()
	return ch
}

// Unsubscribe 取消订阅。
func (r *ChatRoom) Unsubscribe(ch chan []byte) {
	r.mu.Lock()
	delete(r.subs, ch)
	r.mu.Unlock()
}

// broadcastChatLocked 向所有订阅者推送一条聊天消息。调用方必须持有写锁。
func (r *ChatRoom) broadcastChatLocked(msg *protocol.ChatMessage) {
	if len(r.subs) == 0 {
		return
	}
	payload, err := json.Marshal(protocol.ChatEvent{Message: msg})
	if err != nil {
		return
	}
	frame := []byte("event: chat\ndata: " + string(payload) + "\n\n")
	for ch := range r.subs {
		select {
		case ch <- frame:
		default:
			// 订阅者消费太慢,丢弃这一帧避免阻塞整个聊天室。
			// 历史消息仍可通过 REST 拉取。
		}
	}
}
