package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sakurajima336/animeko-watch-together/protocol"
)

// handleEvents GET /v2/watch-together/rooms/{roomId}/events
//
// 聊天室的 SSE 事件流。只推送本扩展的事件:
//   - 注释行 ":connected" / ":ping" -> 连接与心跳
//   - "event: chat"  + data         -> 新的聊天消息
//
// 房间快照(snapshot)不在此推送 —— 房间状态与播放同步由官方服务端负责。
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request, roomID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "NO_STREAMING", "streaming unsupported")
		return
	}

	room := s.rooms.GetOrCreateChatRoom(roomID)

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// 客户端依赖这个注释行判定已连接。
	fmt.Fprint(w, ":connected\n\n")
	flusher.Flush()

	ch := room.Subscribe()
	defer room.Unsubscribe(ch)

	pingTicker := time.NewTicker(15 * time.Second)
	defer pingTicker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return

		case frame := <-ch:
			if _, err := w.Write(frame); err != nil {
				return
			}
			flusher.Flush()

		case <-pingTicker.C:
			// 心跳:防止中间代理因空闲断开连接。
			if _, err := fmt.Fprint(w, ":ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

var _ = protocol.ChatEvent{}
