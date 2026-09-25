package room

import (
	"sync"
	"testing"
	"time"

	"github.com/sakurajima336/animeko-watch-together/protocol"
)

func newMember(id, nick string) *Member {
	return &Member{
		UserID:       id,
		Nickname:     nick,
		State:        protocol.MemberStateIdle,
		SessionNonce: "nonce_" + id,
		LastSeenAt:   time.Now().UnixMilli(),
	}
}

// TestRoomIsolation 验证两个房间的成员、播放状态与聊天记录互不可见。
func TestRoomIsolation(t *testing.T) {
	m := NewManager()

	alice := newMember("u1", "Alice")
	bob := newMember("u2", "Bob")

	roomA, createdA, err := m.JoinOrCreate("room-a", "pw", alice)
	if err != nil {
		t.Fatalf("create room-a: %v", err)
	}
	if !createdA {
		t.Fatal("room-a should be newly created")
	}

	roomB, createdB, err := m.JoinOrCreate("room-b", "pw", bob)
	if err != nil {
		t.Fatalf("create room-b: %v", err)
	}
	if !createdB {
		t.Fatal("room-b should be newly created")
	}
	if roomA.ID == roomB.ID {
		t.Fatal("different rooms must have different IDs")
	}

	// 成员隔离
	if got := len(roomA.Snapshot().Members); got != 1 {
		t.Fatalf("room-a members = %d, want 1", got)
	}
	if got := len(roomB.Snapshot().Members); got != 1 {
		t.Fatalf("room-b members = %d, want 1", got)
	}
	if roomA.Snapshot().Members[0].UserID != "u1" {
		t.Fatal("room-a should contain alice only")
	}
	if roomB.Snapshot().Members[0].UserID != "u2" {
		t.Fatal("room-b should contain bob only")
	}

	// 聊天隔离
	if _, err := roomA.SendChat(alice.SessionNonce, "hello from A"); err != nil {
		t.Fatalf("send in room-a: %v", err)
	}
	if _, err := roomB.SendChat(bob.SessionNonce, "hello from B"); err != nil {
		t.Fatalf("send in room-b: %v", err)
	}
	histA := roomA.ChatHistory()
	histB := roomB.ChatHistory()
	if len(histA) == 0 || len(histB) == 0 {
		t.Fatal("both rooms should have chat history")
	}
	for _, msg := range histA {
		if msg.Content == "hello from B" {
			t.Fatal("room-a leaked a message from room-b")
		}
	}
	for _, msg := range histB {
		if msg.Content == "hello from A" {
			t.Fatal("room-b leaked a message from room-a")
		}
	}

	// 播放状态隔离
	infoA := &protocol.WatchingInfo{SubjectID: 1, EpisodeID: 11, SubjectName: "A"}
	roomA.Report(alice.SessionNonce, protocol.MemberStateWatching, true, infoA)
	if roomA.Snapshot().Playback == nil {
		t.Fatal("room-a should have playback (alice is host)")
	}
	if roomB.Snapshot().Playback != nil {
		t.Fatal("room-b must not see room-a playback")
	}
}

// TestJoinWrongPassword 密码错误必须被拒绝。
func TestJoinWrongPassword(t *testing.T) {
	m := NewManager()
	alice := newMember("u1", "Alice")
	if _, _, err := m.JoinOrCreate("secret", "correct", alice); err != nil {
		t.Fatalf("create: %v", err)
	}
	bob := newMember("u2", "Bob")
	_, _, err := m.JoinOrCreate("secret", "wrong", bob)
	if err != ErrWrongPassword {
		t.Fatalf("err = %v, want ErrWrongPassword", err)
	}
}

// TestNonMemberCannotChat 非成员不能发言。
func TestNonMemberCannotChat(t *testing.T) {
	m := NewManager()
	alice := newMember("u1", "Alice")
	r0, _, err := m.JoinOrCreate("r", "pw", alice)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := r0.SendChat("nonce_stranger", "hi"); err != ErrNotMember {
		t.Fatalf("err = %v, want ErrNotMember", err)
	}
}

// TestHostPlaybackSync 只有房主的上报会更新房间 playback。
func TestHostPlaybackSync(t *testing.T) {
	m := NewManager()
	alice := newMember("u1", "Alice")
	r0, _, err := m.JoinOrCreate("r", "pw", alice)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	bob := newMember("u2", "Bob")
	if err := r0.TryJoin("pw", bob); err != nil {
		t.Fatalf("join: %v", err)
	}

	// 普通成员上报不应影响房间 playback。
	bobInfo := &protocol.WatchingInfo{SubjectID: 999, EpisodeID: 999, SubjectName: "Bob's show"}
	r0.Report(bob.SessionNonce, protocol.MemberStateWatching, true, bobInfo)
	if r0.Snapshot().Playback != nil {
		t.Fatal("follower report must not set room playback")
	}

	// 房主上报才会。
	hostInfo := &protocol.WatchingInfo{SubjectID: 1, EpisodeID: 11, SubjectName: "Host's show"}
	r0.Report(alice.SessionNonce, protocol.MemberStateWatching, true, hostInfo)
	pb := r0.Snapshot().Playback
	if pb == nil {
		t.Fatal("host report must set room playback")
	}
	if pb.Info.SubjectID != 1 {
		t.Fatalf("playback subject = %d, want 1", pb.Info.SubjectID)
	}
}

// TestChatBroadcast 聊天消息会广播给房间订阅者。
func TestChatBroadcast(t *testing.T) {
	m := NewManager()
	alice := newMember("u1", "Alice")
	r0, _, err := m.JoinOrCreate("r", "pw", alice)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ch := r0.Subscribe()
	defer r0.Unsubscribe(ch)

	if _, err := r0.SendChat(alice.SessionNonce, "broadcast me"); err != nil {
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

// TestConcurrentJoin 并发加入房间不应产生数据竞争或超额成员。
func TestConcurrentJoin(t *testing.T) {
	m := NewManager()
	host := newMember("host", "Host")
	r0, _, err := m.JoinOrCreate("race", "pw", host)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := string(rune('a' + i))
			_ = r0.TryJoin("pw", newMember("u_"+id, "user"+id))
		}(i)
	}
	wg.Wait()

	if got := r0.MemberCount(); got != 11 {
		t.Fatalf("members = %d, want 11", got)
	}
}
