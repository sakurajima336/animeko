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

// TestJoinThenChat 端到端:加入房间 -> 发言 -> 拉取历史。
func TestJoinThenChat(t *testing.T) {
	srv := newTestServer()

	rec := postJSON(t, srv, "/v2/watch-together/join?userId=u1&nickname=Alice",
		`{"roomName":"test","password":"pw","following":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("join status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var join protocol.JoinResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &join); err != nil {
		t.Fatalf("decode join: %v", err)
	}
	if join.RoomID == "" || join.SessionNonce == "" {
		t.Fatalf("join response missing roomId/nonce: %+v", join)
	}
	if !join.IsHost {
		t.Fatal("first joiner should be host")
	}
	if join.Snapshot == nil || len(join.Snapshot.Members) != 1 {
		t.Fatal("join response should carry a snapshot with 1 member")
	}

	// 发言
	chatPath := "/v2/watch-together/rooms/" + join.RoomID + "/chat"
	rec = postJSON(t, srv, chatPath,
		`{"sessionNonce":"`+join.SessionNonce+`","content":"hello everyone"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("chat status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// 历史
	rec = getJSON(t, srv, chatPath)
	if rec.Code != http.StatusOK {
		t.Fatalf("history status = %d", rec.Code)
	}
	var hist protocol.ChatHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &hist); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	found := false
	for _, m := range hist.Messages {
		if m.Content == "hello everyone" {
			found = true
			if m.Nickname != "Alice" {
				t.Fatalf("nickname = %q, want Alice", m.Nickname)
			}
		}
	}
	if !found {
		t.Fatalf("message not found in history: %+v", hist.Messages)
	}
}

// TestChatRequiresMembership 陌生人不能往房间发言。
func TestChatRequiresMembership(t *testing.T) {
	srv := newTestServer()
	rec := postJSON(t, srv, "/v2/watch-together/join?userId=u1&nickname=Alice",
		`{"roomName":"test","password":"pw"}`)
	var join protocol.JoinResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &join)

	chatPath := "/v2/watch-together/rooms/" + join.RoomID + "/chat"
	rec = postJSON(t, srv, chatPath, `{"sessionNonce":"bogus","content":"let me in"}`)
	if rec.Code == http.StatusOK {
		t.Fatal("non-member should not be able to post")
	}
}

// TestRoomIsolationOverHTTP 两个房间的聊天记录互不可见。
func TestRoomIsolationOverHTTP(t *testing.T) {
	srv := newTestServer()

	var a, b protocol.JoinResponse
	rec := postJSON(t, srv, "/v2/watch-together/join?userId=u1&nickname=Alice",
		`{"roomName":"room-a","password":"pw"}`)
	_ = json.Unmarshal(rec.Body.Bytes(), &a)
	rec = postJSON(t, srv, "/v2/watch-together/join?userId=u2&nickname=Bob",
		`{"roomName":"room-b","password":"pw"}`)
	_ = json.Unmarshal(rec.Body.Bytes(), &b)

	if a.RoomID == b.RoomID {
		t.Fatal("room ids must differ")
	}

	_ = postJSON(t, srv, "/v2/watch-together/rooms/"+a.RoomID+"/chat",
		`{"sessionNonce":"`+a.SessionNonce+`","content":"secret-A"}`)
	_ = postJSON(t, srv, "/v2/watch-together/rooms/"+b.RoomID+"/chat",
		`{"sessionNonce":"`+b.SessionNonce+`","content":"secret-B"}`)

	rec = getJSON(t, srv, "/v2/watch-together/rooms/"+a.RoomID+"/chat")
	var histA protocol.ChatHistoryResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &histA)
	for _, m := range histA.Messages {
		if m.Content == "secret-B" {
			t.Fatal("room-a leaked room-b's message")
		}
	}

	rec = getJSON(t, srv, "/v2/watch-together/rooms/"+b.RoomID+"/chat")
	var histB protocol.ChatHistoryResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &histB)
	for _, m := range histB.Messages {
		if m.Content == "secret-A" {
			t.Fatal("room-b leaked room-a's message")
		}
	}
}

// TestWrongPasswordOverHTTP 密码错误返回 WRONG_PASSWORD。
func TestWrongPasswordOverHTTP(t *testing.T) {
	srv := newTestServer()
	_ = postJSON(t, srv, "/v2/watch-together/join?userId=u1&nickname=Alice",
		`{"roomName":"locked","password":"right"}`)
	rec := postJSON(t, srv, "/v2/watch-together/join?userId=u2&nickname=Bob",
		`{"roomName":"locked","password":"bad"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "WRONG_PASSWORD") {
		t.Fatalf("body = %s, want WRONG_PASSWORD", rec.Body.String())
	}
}

// TestReportUpdatesPlayback 房主上报播放状态后房间快照带上 playback。
func TestReportUpdatesPlayback(t *testing.T) {
	srv := newTestServer()
	rec := postJSON(t, srv, "/v2/watch-together/join?userId=u1&nickname=Alice",
		`{"roomName":"sync","password":"pw"}`)
	var join protocol.JoinResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &join)

	body := `{"sessionNonce":"` + join.SessionNonce + `","memberState":"WATCHING","following":true,` +
		`"watching":{"subjectId":1,"episodeId":11,"subjectName":"番剧","episodeSort":"1","episodeName":"第一话",` +
		`"positionMillis":1000,"positionAtMillis":1000,"durationMillis":100000,"paused":false,` +
		`"buffering":false,"loading":false,"playbackRate":1.0}}`

	rec = postJSON(t, srv, "/v2/watch-together/rooms/"+join.RoomID+"/report", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("report status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var rep protocol.ReportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &rep); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if rep.Membership != protocol.MembershipOK {
		t.Fatalf("membership = %v, want OK", rep.Membership)
	}
	if rep.Snapshot == nil || rep.Snapshot.Playback == nil {
		t.Fatal("host report should produce playback in snapshot")
	}
	if rep.Snapshot.Playback.Info.SubjectID != 1 {
		t.Fatalf("subjectId = %d, want 1", rep.Snapshot.Playback.Info.SubjectID)
	}
}

// TestHealthz 健康检查可用。
func TestHealthz(t *testing.T) {
	srv := newTestServer()
	rec := getJSON(t, srv, "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ok") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

// TestCORS 预检请求被正确处理。
func TestCORS(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodOptions, "/v2/watch-together/join", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("CORS header = %q", got)
	}
}
