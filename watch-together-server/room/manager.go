package room

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Manager 管理各官方房间对应的聊天室。
//
// 键是**官方服务端下发的 roomId**。Manager 不做房间名校验, 也无法创建房间 ——
// 房间的权威来源只有官方服务端, 这里只负责在给定 roomId 上维护聊天室。
type Manager struct {
	mu    sync.RWMutex
	rooms map[string]*ChatRoom
}

func NewManager() *Manager {
	m := &Manager{rooms: make(map[string]*ChatRoom)}
	go m.reapLoop()
	return m
}

// GetOrCreateChatRoom 取得 roomId 对应的聊天室, 不存在则按需建立。
//
// 按需建立是刻意的: 官方房间可能先于本扩展存在, 不应因此拒绝聊天。
func (m *Manager) GetOrCreateChatRoom(roomID string) *ChatRoom {
	m.mu.RLock()
	if room, ok := m.rooms[roomID]; ok {
		m.mu.RUnlock()
		return room
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if room, ok := m.rooms[roomID]; ok {
		return room
	}
	room := NewChatRoom(roomID)
	m.rooms[roomID] = room
	return room
}

// Get 按 roomId 获取聊天室。不存在时返回 nil。
func (m *Manager) Get(roomID string) *ChatRoom {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rooms[roomID]
}

// Stats 返回聊天室数量与消息总数。
func (m *Manager) Stats() (int, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	messages := 0
	for _, room := range m.rooms {
		messages += room.MessageCount()
	}
	return len(m.rooms), messages
}

// reapLoop 回收长期无人访问的聊天室, 避免内存无限增长。
func (m *Manager) reapLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		for id, room := range m.rooms {
			if room.IdleSince() > chatRoomTTL {
				delete(m.rooms, id)
			}
		}
		m.mu.Unlock()
	}
}

func newID(prefix string) string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return prefix + "_" + hex.EncodeToString(buf)
}
