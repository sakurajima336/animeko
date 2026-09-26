package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sakurajima336/animeko-watch-together/protocol"
	"github.com/sakurajima336/animeko-watch-together/room"
)

func newTestServer() *Server {
	return NewServer(room.NewManager())
}

func postJSON(t *testing.T, srv *Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func getJSON(t *testing.T, srv *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

// TestHealthIdentifiesAsChatExtension 健康检查应表明自己是聊天扩展。
func TestHealthIdentifiesAsChatExtension(t *testing.T) {
	srv := newTestServer()
	rec := getJSON(t, srv, "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["role"] != "chat-extension" {
		t.Fatalf("role = %v, want chat-extension", body["role"])
	}
}

// TestJoinIsNotProvided 房间生命周期接口必须不存在。
func TestJoinIsNotProvided(t *testing.T) {
	srv := newTestServer()
	rec := postJSON(t, srv, "/v2/watch-together/join",
		`{"roomName":"x","password":"y"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("join status = %d, want 404 (rooms come from the official server)", rec.Code)
	}
}

// TestReportAndLeaveAreNotProvided 房间状态接口也必须不存在。
func TestReportAndLeaveAreNotProvided(t *testing.T) {
	srv := newTestServer()
	rec := postJSON(t, srv, "/v2/watch-together/rooms/r1/report", `{}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("report status = %d, want 404", rec.Code)
	}
	rec = postJSON(t, srv, "/v2/watch-together/rooms/r1/leave", `{}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("leave status = %d, want 404", rec.Code)
	}
}

// TestChatWithOfficialRoomId 用官方 roomId 收发消息。
func TestChatWithOfficialRoomId(t *testing.T) {
	srv := newTestServer()
	// 模拟官方服务端下发的 roomId。
	officialRoomID := "r_official_abc123"
	chatPath := "/v2/watch-together/rooms/" + officialRoomID + "/chat"

	rec := postJSON(t, srv, chatPath+"?userId=u1&nickname=Alice",
		`{"sessionNonce":"official-nonce-1","content":"hello everyone"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("chat status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = getJSON(t, srv, chatPath)
	if rec.Code != http.StatusOK {
		t.Fatalf("history status = %d", rec.Code)
	}
	var hist protocol.ChatHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &hist); err != nil {
		t.Fatalf("decode: %v", err)
	}
	found := false
	for _, m := range hist.Messages {
		if m.Content == "hello everyone" {
			found = true
			if m.Nickname != "Alice" {
				t.Fatalf("nickname = %q, want Alice", m.Nickname)
			}
			if m.RoomID != officialRoomID {
				t.Fatalf("roomId = %q, want %q", m.RoomID, officialRoomID)
			}
		}
	}
	if !found {
		t.Fatalf("message not found: %+v", hist.Messages)
	}
}

// TestChatRequiresSessionNonce 没有官方 sessionNonce 不能发言。
func TestChatRequiresSessionNonce(t *testing.T) {
	srv := newTestServer()
	chatPath := "/v2/watch-together/rooms/r_official/chat"
	rec := postJSON(t, srv, chatPath, `{"sessionNonce":"","content":"let me in"}`)
	if rec.Code == http.StatusOK {
		t.Fatal("empty sessionNonce must be rejected")
	}
}

// TestRoomIsolationOverHTTP 两个官方 roomId 的聊天记录互不可见。
func TestRoomIsolationOverHTTP(t *testing.T) {
	srv := newTestServer()
	roomA := "r_official_a"
	roomB := "r_official_b"

	_ = postJSON(t, srv, "/v2/watch-together/rooms/"+roomA+"/chat?userId=u1&nickname=Alice",
		`{"sessionNonce":"n1","content":"secret-A"}`)
	_ = postJSON(t, srv, "/v2/watch-together/rooms/"+roomB+"/chat?userId=u2&nickname=Bob",
		`{"sessionNonce":"n2","content":"secret-B"}`)

	for _, tc := range []struct {
		room      string
		forbidden string
	}{{roomA, "secret-B"}, {roomB, "secret-A"}} {
		rec := getJSON(t, srv, "/v2/watch-together/rooms/"+tc.room+"/chat")
		var hist protocol.ChatHistoryResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &hist)
		for _, m := range hist.Messages {
			if m.Content == tc.forbidden {
				t.Fatalf("room %s leaked message %q", tc.room, tc.forbidden)
			}
		}
	}
}

// TestEmptyMessageRejected 空消息被拒绝。
func TestEmptyMessageRejected(t *testing.T) {
	srv := newTestServer()
	rec := postJSON(t, srv, "/v2/watch-together/rooms/r_official/chat",
		`{"sessionNonce":"n1","content":"   "}`)
	if rec.Code == http.StatusOK {
		t.Fatal("blank message must be rejected")
	}
}

// TestCORS 预检请求被正确处理。
func TestCORS(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodOptions, "/v2/watch-together/rooms/r1/chat", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("CORS header = %q", got)
	}
}
