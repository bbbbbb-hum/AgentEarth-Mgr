package mcpclient

import (
	"context"
	"sync"
	"time"
)

// SessionEntry 会话条目
type SessionEntry struct {
	Client     *Client
	ConfigID   int64
	WemcpName  string
	ServiceURL string
	ServerInfo *ServerInfo
	CreatedAt  time.Time
	LastUsedAt time.Time
}

// SessionManager MCP会话管理器
// 用于缓存已初始化的MCP客户端连接，避免每次调用都重新初始化
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[int64]*SessionEntry // key: configId
	ttl      time.Duration           // 会话过期时间
	maxSize  int                     // 最大缓存数量
}

var (
	globalManager *SessionManager
	managerOnce   sync.Once
)

// GetSessionManager 获取全局会话管理器实例
func GetSessionManager() *SessionManager {
	managerOnce.Do(func() {
		globalManager = NewSessionManager(5*time.Minute, 100)
		// 启动后台清理协程
		go globalManager.startCleanupRoutine()
	})
	return globalManager
}

// NewSessionManager 创建新的会话管理器
func NewSessionManager(ttl time.Duration, maxSize int) *SessionManager {
	return &SessionManager{
		sessions: make(map[int64]*SessionEntry),
		ttl:      ttl,
		maxSize:  maxSize,
	}
}

// GetOrCreate 获取或创建会话
// 如果存在有效的缓存会话则返回，否则创建新会话
func (m *SessionManager) GetOrCreate(ctx context.Context, configID int64, wemcpName, serviceURL string, timeout time.Duration) (*SessionEntry, bool, error) {
	// 先尝试从缓存获取
	m.mu.RLock()
	entry, exists := m.sessions[configID]
	m.mu.RUnlock()

	// 检查缓存是否有效
	if exists && entry.ServiceURL == serviceURL && time.Since(entry.LastUsedAt) < m.ttl {
		// 更新最后使用时间
		m.mu.Lock()
		entry.LastUsedAt = time.Now()
		m.mu.Unlock()
		return entry, true, nil // true 表示命中缓存
	}

	// 创建新客户端
	client := NewClient(serviceURL, timeout)

	// 初始化连接
	initResult, err := client.Initialize(ctx)
	if err != nil {
		return nil, false, err
	}

	// 创建会话条目
	newEntry := &SessionEntry{
		Client:     client,
		ConfigID:   configID,
		WemcpName:  wemcpName,
		ServiceURL: serviceURL,
		ServerInfo: &initResult.ServerInfo,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
	}

	// 保存到缓存
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否需要清理旧会话
	if len(m.sessions) >= m.maxSize {
		m.evictOldest()
	}

	m.sessions[configID] = newEntry
	return newEntry, false, nil // false 表示新创建
}

// Get 获取已存在的会话
func (m *SessionManager) Get(configID int64) (*SessionEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.sessions[configID]
	if !exists || time.Since(entry.LastUsedAt) >= m.ttl {
		return nil, false
	}

	return entry, true
}

// Remove 移除会话
func (m *SessionManager) Remove(configID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, configID)
}

// Clear 清除所有会话
func (m *SessionManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = make(map[int64]*SessionEntry)
}

// UpdateLastUsed 更新最后使用时间
func (m *SessionManager) UpdateLastUsed(configID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry, exists := m.sessions[configID]; exists {
		entry.LastUsedAt = time.Now()
	}
}

// evictOldest 驱逐最旧的会话（必须在持有锁的情况下调用）
func (m *SessionManager) evictOldest() {
	var oldestID int64
	var oldestTime time.Time

	for id, entry := range m.sessions {
		if oldestTime.IsZero() || entry.LastUsedAt.Before(oldestTime) {
			oldestID = id
			oldestTime = entry.LastUsedAt
		}
	}

	if oldestID != 0 {
		delete(m.sessions, oldestID)
	}
}

// startCleanupRoutine 启动后台清理协程
func (m *SessionManager) startCleanupRoutine() {
	ticker := time.NewTicker(m.ttl / 2)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanup()
	}
}

// cleanup 清理过期会话
func (m *SessionManager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, entry := range m.sessions {
		if now.Sub(entry.LastUsedAt) >= m.ttl {
			delete(m.sessions, id)
		}
	}
}

// Stats 返回会话统计信息
func (m *SessionManager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_sessions": len(m.sessions),
		"max_size":       m.maxSize,
		"ttl_seconds":    m.ttl.Seconds(),
	}
}
