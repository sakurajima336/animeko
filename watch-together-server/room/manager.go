package room

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

// Manager 管理所有房间。房间之间完全隔离:各自的成员列表、播放状态与聊天记录互不可见。
type Manager struct {
	mu    sync.RWMutex
	rooms map[string]*Room
	// byName 房间名 -> 房间 ID,用于按名字加入。
	byName map[string]string
}

func NewManager() *Manager {
	m := &Manager{
		rooms:  make(map[string]*Room),
		byName: make(map[string]string),
	}
	go m.reapLoop()
	return m
}

// JoinOrCreate 加入同名房间,不存在则以当前密码创建。
// 返回房间、是否为新建、以及错误。
func (m *Manager) JoinOrCreate(roomName, password string, member *Member) (*Room, bool, error) {
	if err := validateRoomName(roomName); err != nil {
		return nil, false, err
	}
	if err := validatePassword(password); err != nil {
		return nil, false, err
	}

	m.mu.Lock()
	id, ok := m.byName[roomName]
	if ok {
		if r, exists := m.rooms[id]; exists {
			m.mu.Unlock()
			if err := r.TryJoin(password, member); err != nil {
				return nil, false, err
			}
			r.AddSystemMessage(member.Nickname + " 加入了房间")
			return r, false, nil
		}
		// 索引残留,清理后当作新房间处理。
		delete(m.byName, roomName)
	}
	r := NewRoom(newID("r"), roomName, password, member)
	m.rooms[r.ID] = r
	m.byName[roomName] = r.ID
	m.mu.Unlock()
	r.AddSystemMessage(member.Nickname + " 创建了房间")
	return r, true, nil
}

// Get 按房间 ID 获取。
func (m *Manager) Get(roomID string) (*Room, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rooms[roomID]
	return r, ok
}

// Remove 销毁房间。
func (m *Manager) Remove(roomID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[roomID]
	if !ok {
		return
	}
	delete(m.rooms, roomID)
	delete(m.byName, r.Name)
}

// Stats 当前房间数。
func (m *Manager) Stats() (int, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	members := 0
	for _, r := range m.rooms {
		members += r.MemberCount()
	}
	return len(m.rooms), members
}

// reapLoop 定期清理无人房间与超时成员。
func (m *Manager) reapLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.RLock()
		ids := make([]string, 0, len(m.rooms))
		for id := range m.rooms {
			ids = append(ids, id)
		}
		m.mu.RUnlock()

		for _, id := range ids {
			r, ok := m.Get(id)
			if !ok {
				continue
			}
			if r.ReapStale() {
				m.Remove(id)
			}
		}
	}
}

func validateRoomName(name string) error {
	n := len(strings.TrimSpace(name))
	if n < 1 || n > MaxRoomNameLen {
		return ErrInvalidName
	}
	return nil
}

func validatePassword(pwd string) error {
	if len(pwd) > MaxPasswordLen {
		return ErrInvalidPassword
	}
	return nil
}

func newID(prefix string) string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return prefix + "_" + hex.EncodeToString(buf)
}
