package room

import (
	"encoding/json"

	"github.com/sakurajima336/animeko-watch-together/protocol"
)

// subscribe 注册一个 SSE 订阅通道。
func (r *Room) subscribe() chan []byte {
	ch := make(chan []byte, 64)
	r.mu.Lock()
	r.subs[ch] = struct{}{}
	r.mu.Unlock()
	return ch
}

func (r *Room) unsubscribe(ch chan []byte) {
	r.mu.Lock()
	delete(r.subs, ch)
	r.mu.Unlock()
}

// broadcastLocked 向所有订阅者推送当前快照。调用方必须持有写锁。
func (r *Room) broadcastLocked() {
	if len(r.subs) == 0 {
		return
	}
	payload, err := json.Marshal(r.snapshotLocked())
	if err != nil {
		return
	}
	frame := "event: snapshot\ndata: " + string(payload) + "\n\n"
	for ch := range r.subs {
		select {
		case ch <- []byte(frame):
		default:
			// 订阅者消费太慢,丢弃这一帧避免阻塞整个房间。
		}
	}
}

// broadcastChatLocked 向所有订阅者推送一条聊天消息。调用方必须持有写锁。
func (r *Room) broadcastChatLocked(msg *protocol.ChatMessage) {
	if len(r.subs) == 0 {
		return
	}
	payload, err := json.Marshal(protocol.ChatEvent{Message: msg})
	if err != nil {
		return
	}
	frame := "event: chat\ndata: " + string(payload) + "\n\n"
	for ch := range r.subs {
		select {
		case ch <- []byte(frame):
		default:
		}
	}
}

// Subscribe 公开的订阅入口,供 HTTP 层调用。
func (r *Room) Subscribe() chan []byte { return r.subscribe() }

// Unsubscribe 取消订阅。
func (r *Room) Unsubscribe(ch chan []byte) { r.unsubscribe(ch) }
