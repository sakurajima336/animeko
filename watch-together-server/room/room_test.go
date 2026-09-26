package room

import (
	"sync"
	"testing"
	"time"
)

// TestRoomIsolation 不同官方 roomId 的聊天室互不可见。
func TestRoomIsolation(t *testing.T) {
	m := NewManager()

	a := m.GetOrCreateChatRoom("official_room_a")
	b := m.GetOrCreateChatRoom("official_room_b")
	if a == b {
		t.Fatal("different roomIds must map to different chat rooms")
	}

	if _, err := a.SendChatBySession("nonce-a", "u1", "Alice", nil, "hello from A"); err != nil {
		t.Fatalf("send in A: %v", err)
	}
	if _, err := b.SendChatBySession("nonce-b", "u2", "Bob", nil, "hello from B"); err != nil {
		t.Fatalf("send in B: %v", err)
	}

	for _, msg := range a.ChatHistory() {
		if msg.Content == "hello from B" {
			t.Fatal("chat room A leaked a message from B")
		}
	}
	for _, msg := range b.ChatHistory() {
		if msg.Content == "hello from A" {
			t.Fatal("chat room B leaked a message from A")
		}
	}
}

// TestGetOrCreateIsStable 同一 roomId 反复获取得到同一实例。
func TestGetOrCreateIsStable(t *testing.T) {
	m := NewManager()
	first := m.GetOrCreateChatRoom("official_room")
	second := m.GetOrCreateChatRoom("official_room")
	if first != second {
		t.Fatal("GetOrCreateChatRoom must be stable for the same roomId")
	}
	if m.Get("official_room") == nil {
		t.Fatal("Get should return the created room")
	}
}

// TestSendRequiresSession 缺少官方 sessionNonce 时必须拒绝。
func TestSendRequiresSession(t *testing.T) {
	m := NewManager()
	r := m.GetOrCreateChatRoom("official_room")
	if _, err := r.SendChatBySession("", "u1", "Alice", nil, "hi"); err != ErrMissingSession {
		t.Fatalf("err = %v, want ErrMissingSession", err)
	}
	if r.MessageCount() != 0 {
		t.Fatal("rejected message must not be stored")
	}
}

// TestIdentityFollowsOfficialMember 昵称与头像随请求透传, 且同一 nonce 缺省时沿用登记值。
func TestIdentityFollowsOfficialMember(t *testing.T) {
	m := NewManager()
	r := m.GetOrCreateChatRoom("official_room")

	avatar := "https://example.com/a.png"
	first, err := r.SendChatBySession("nonce", "u1", "Alice", &avatar, "hi")
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if first.Nickname != "Alice" || first.AvatarURL == nil || *first.AvatarURL != avatar {
		t.Fatalf("first message identity = %q / %v, want Alice / %q", first.Nickname, first.AvatarURL, avatar)
	}

	// 后续请求不带昵称与头像时, 沿用首次登记的官方成员信息。
	second, err := r.SendChatBySession("nonce", "u1", "", nil, "again")
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if second.Nickname != "Alice" {
		t.Fatalf("nickname = %q, want Alice (registered on first send)", second.Nickname)
	}
	if second.AvatarURL == nil || *second.AvatarURL != avatar {
		t.Fatalf("avatar = %v, want %q", second.AvatarURL, avatar)
	}
}

// TestIdentityUpdatesWhenOfficialMemberChanges 官方成员换了身份(同 nonce 新 userId)时以新信息为准。
func TestIdentityUpdatesWhenOfficialMemberChanges(t *testing.T) {
	m := NewManager()
	r := m.GetOrCreateChatRoom("official_room")

	if _, err := r.SendChatBySession("nonce", "u1", "Alice", nil, "hi"); err != nil {
		t.Fatalf("send: %v", err)
	}
	avatar := "https://example.com/b.png"
	msg, err := r.SendChatBySession("nonce", "u2", "Bob", &avatar, "hi again")
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if msg.UserID != "u2" || msg.Nickname != "Bob" || msg.AvatarURL == nil || *msg.AvatarURL != avatar {
		t.Fatalf("message identity = %q / %q / %v, want u2 / Bob / %q", msg.UserID, msg.Nickname, msg.AvatarURL, avatar)
	}
}

// TestChatBroadcast 聊天消息会广播给订阅者。
func TestChatBroadcast(t *testing.T) {
	m := NewManager()
	r := m.GetOrCreateChatRoom("official_room")
	ch := r.Subscribe()
	defer r.Unsubscribe(ch)

	if _, err := r.SendChatBySession("nonce", "u1", "Alice", nil, "broadcast me"); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case frame := <-ch:
		if len(frame) == 0 {
			t.Fatal("empty frame")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no broadcast received")
	}
}

// TestConcurrentSend 并发发送不应丢消息或产生数据竞争。
func TestConcurrentSend(t *testing.T) {
	m := NewManager()
	r := m.GetOrCreateChatRoom("official_room")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = r.SendChatBySession("nonce", "u1", "Alice", nil, "msg")
		}(i)
	}
	wg.Wait()

	if got := r.MessageCount(); got != 20 {
		t.Fatalf("message count = %d, want 20", got)
	}
}

// TestHistoryCap 消息数超过上限时只保留最近若干条。
func TestHistoryCap(t *testing.T) {
	m := NewManager()
	r := m.GetOrCreateChatRoom("official_room")
	for i := 0; i < maxChatHistory+50; i++ {
		if _, err := r.SendChatBySession("nonce", "u1", "Alice", nil, "msg"); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	if got := r.MessageCount(); got != maxChatHistory {
		t.Fatalf("message count = %d, want %d", got, maxChatHistory)
	}
}

// TestStats 统计聊天室数与消息数。
func TestStats(t *testing.T) {
	m := NewManager()
	a := m.GetOrCreateChatRoom("room_a")
	m.GetOrCreateChatRoom("room_b")
	_, _ = a.SendChatBySession("nonce", "u1", "Alice", nil, "hi")

	rooms, messages := m.Stats()
	if rooms != 2 || messages != 1 {
		t.Fatalf("stats = (%d, %d), want (2, 1)", rooms, messages)
	}
}
