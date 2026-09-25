package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sakurajima336/animeko-watch-together/protocol"
)

// handleEvents GET /v2/watch-together/rooms/{roomId}/events
//
// SSE 事件流。客户端 Ktor SSE 插件会解析:
//   - 注释行 ":connected" / ":ping"  -> Connected / Ping
//   - "event: snapshot" + data       -> 房间快照
//   - "event: chat" + data           -> 聊天消息(本服务端扩展)
//   - "event: bye" + data            -> 会话结束
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request, roomID string) {
	r0, ok := s.rooms.Get(roomID)
	if !ok {
		writeError(w, http.StatusForbidden, string(protocol.MembershipRoomClosed), "room not found")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "NO_STREAMING", "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// 客户端依赖这个注释行判定已连接。
	fmt.Fprint(w, ":connected\n\n")
	flusher.Flush()

	ch := r0.Subscribe()
	defer r0.Unsubscribe(ch)

	// 首帧立刻下发当前快照,让客户端无需等待第一次状态变化。
	if snap := r0.Snapshot(); snap != nil {
		if payload, err := json.Marshal(snap); err == nil {
			fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", payload)
			flusher.Flush()
		}
	}

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
