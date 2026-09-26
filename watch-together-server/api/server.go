package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/sakurajima336/animeko-watch-together/protocol"
	"github.com/sakurajima336/animeko-watch-together/room"
)

// Server 官方一起看房间的聊天扩展 HTTP 服务。
//
// 房间生命周期(join/report/leave)不在此实现: 房间由官方服务端负责,
// 这里只在官方下发的 roomId 上挂一个聊天室。
type Server struct {
	rooms *room.Manager
}

func NewServer(rooms *room.Manager) *Server {
	return &Server{rooms: rooms}
}

// Handler 注册路由。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", s.handleHealth)

	// /v2/watch-together/rooms/{roomId}/...
	// roomId 必须是官方服务端下发的 id, 扩展不解析房间名。
	mux.HandleFunc("/v2/watch-together/rooms/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/v2/watch-together/rooms/")
		parts := strings.Split(rest, "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "missing roomId")
			return
		}
		roomID := parts[0]

		var action string
		if len(parts) > 1 {
			action = parts[1]
		}

		switch action {
		case "chat":
			s.handleChat(w, r, roomID)
		case "events":
			s.handleEvents(w, r, roomID)
		default:
			// 房间生命周期接口一律不提供。
			writeError(w, http.StatusNotFound, "NOT_FOUND", "unknown action")
		}
	})

	return corsMiddleware(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	roomCount, messageCount := s.rooms.Stats()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"role":     "chat-extension",
		"serverAt": time.Now().UnixMilli(),
		"rooms":    roomCount,
		"messages": messageCount,
	})
}

// handleChat 房间聊天。GET 取历史, POST 发言。
//
// 房间可能只存在于官方服务端: 首次收到某 roomId 的消息时会按需建立聊天室,
// 因此不会因为"扩展里还没有该房间"而拒绝官方房间的聊天。
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request, roomID string) {
	switch r.Method {
	case http.MethodGet:
		room := s.rooms.GetOrCreateChatRoom(roomID)
		writeJSON(w, http.StatusOK, protocol.ChatHistoryResponse{
			ServerTime: time.Now().UnixMilli(),
			Messages:   room.ChatHistory(),
		})

	case http.MethodPost:
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "unreadable body")
			return
		}
		var req protocol.SendChatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "malformed json")
			return
		}
		content := strings.TrimSpace(req.Content)
		if content == "" {
			writeError(w, http.StatusBadRequest, "EMPTY_MESSAGE", "empty content")
			return
		}
		if len(content) > 500 {
			content = content[:500]
		}
		if req.SessionNonce == "" {
			writeError(w, http.StatusBadRequest, "MISSING_SESSION", "sessionNonce is required")
			return
		}

		user := identifyUser(r, roomID)
		room := s.rooms.GetOrCreateChatRoom(roomID)
		msg, err := room.SendChatBySession(req.SessionNonce, user.id, user.nickname, content)
		if err != nil {
			writeError(w, http.StatusBadRequest, "CHAT_FAILED", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, msg)

	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "use GET or POST")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, protocol.ErrorResponse{Code: code, Message: message})
}

var _ = errors.Is
