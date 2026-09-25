package api

import (
	"crypto/rand"
	"encoding/hex"
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

// Server 一起看 HTTP 服务。
type Server struct {
	rooms *room.Manager
}

func NewServer(rooms *room.Manager) *Server {
	return &Server{rooms: rooms}
}

// Handler 注册路由。路径前缀与客户端约定的 /v2/watch-together 一致。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v2/watch-together/join", s.handleJoin)

	// /v2/watch-together/rooms/{roomId}/...
	mux.HandleFunc("/v2/watch-together/rooms/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/v2/watch-together/rooms/")
		parts := strings.Split(rest, "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, http.StatusNotFound, "NOT_FOUND", " missing roomId")
			return
		}
		roomID := parts[0]

		var action string
		if len(parts) > 1 {
			action = parts[1]
		}

		switch action {
		case "report":
			s.handleReport(w, r, roomID)
		case "leave":
			s.handleLeave(w, r, roomID)
		case "events":
			s.handleEvents(w, r, roomID)
		case "chat":
			s.handleChat(w, r, roomID)
		default:
			writeError(w, http.StatusNotFound, "NOT_FOUND", "unknown action")
		}
	})

	return corsMiddleware(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	roomCount, memberCount := s.rooms.Stats()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"serverAt": time.Now().UnixMilli(),
		"rooms":    roomCount,
		"members":  memberCount,
	})
}

// handleJoin POST /v2/watch-together/join
func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "use POST")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "unreadable body")
		return
	}
	var req protocol.JoinRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_NAME", "malformed json")
		return
	}

	user := identifyUser(r, req.RoomName)
	member := &room.Member{
		UserID:       user.id,
		Nickname:     user.nickname,
		Following:    req.Following == nil || *req.Following,
		State:        protocol.MemberStateIdle,
		SessionNonce: newNonce(),
		LastSeenAt:   time.Now().UnixMilli(),
		AvatarURL:    user.avatar,
	}

	r0, created, err := s.rooms.JoinOrCreate(req.RoomName, req.Password, member)
	if err != nil {
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, room.ErrWrongPassword):
			status = http.StatusForbidden
		case errors.Is(err, room.ErrRoomFull):
			status = http.StatusForbidden
		case errors.Is(err, room.ErrRoomClosed):
			status = http.StatusForbidden
		}
		writeError(w, status, err.Error(), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, protocol.JoinResponse{
		RoomID:       r0.ID,
		Created:      created,
		IsHost:       member.IsHost,
		SessionNonce: member.SessionNonce,
		ServerTime:   time.Now().UnixMilli(),
		Snapshot:     r0.Snapshot(),
	})
}

// handleReport POST /v2/watch-together/rooms/{roomId}/report
func (s *Server) handleReport(w http.ResponseWriter, r *http.Request, roomID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "use POST")
		return
	}
	r0, ok := s.rooms.Get(roomID)
	if !ok {
		writeError(w, http.StatusForbidden, string(protocol.MembershipRoomClosed), "room not found")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "unreadable body")
		return
	}
	var req protocol.ReportRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "malformed json")
		return
	}

	membership, snapshot := r0.Report(req.SessionNonce, req.MemberState, req.Following, req.Watching)
	writeJSON(w, http.StatusOK, protocol.ReportResponse{
		ServerTime: time.Now().UnixMilli(),
		Membership: membership,
		Version:    r0.Version(),
		Snapshot:   snapshot,
	})
}

// handleLeave POST /v2/watch-together/rooms/{roomId}/leave
func (s *Server) handleLeave(w http.ResponseWriter, r *http.Request, roomID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "use POST")
		return
	}
	r0, ok := s.rooms.Get(roomID)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err == nil {
		var req protocol.LeaveRequest
		if json.Unmarshal(body, &req) == nil && req.SessionNonce != "" {
			r0.Leave(req.SessionNonce)
			if r0.IsEmpty() {
				s.rooms.Remove(roomID)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleChat 房间聊天:GET 取历史,POST 发言。
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request, roomID string) {
	r0, ok := s.rooms.Get(roomID)
	if !ok {
		writeError(w, http.StatusNotFound, string(protocol.MembershipRoomClosed), "room not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, protocol.ChatHistoryResponse{
			ServerTime: time.Now().UnixMilli(),
			Messages:   r0.ChatHistory(),
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
		msg, err := r0.SendChat(req.SessionNonce, content)
		if err != nil {
			code := "NOT_MEMBER"
			status := http.StatusForbidden
			if errors.Is(err, room.ErrRoomClosed) {
				code = string(protocol.MembershipRoomClosed)
			}
			writeError(w, status, code, err.Error())
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

func newNonce() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
