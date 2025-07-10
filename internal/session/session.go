package session

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"sync"
	"time"
)

// Session 代表一个用户会话
type Session struct {
	ID        string
	Values    map[string]interface{}
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Manager 会话管理器
type Manager struct {
	sessions      map[string]*Session
	mutex         sync.RWMutex
	cookieName    string
	maxLifetime   time.Duration
	cookieOptions http.Cookie
}

// NewManager 创建一个新的会话管理器
func NewManager(cookieName string, maxLifetime time.Duration) *Manager {
	cookieOptions := http.Cookie{
		Name:     cookieName,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false, // 开发环境可设为false，生产环境应设为true
	}

	return &Manager{
		sessions:      make(map[string]*Session),
		cookieName:    cookieName,
		maxLifetime:   maxLifetime,
		cookieOptions: cookieOptions,
	}
}

// 生成随机会话ID
func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// Start 开始一个新会话或获取现有会话
func (m *Manager) Start(w http.ResponseWriter, r *http.Request) (*Session, error) {
	// 尝试从cookie获取会话ID
	cookie, err := r.Cookie(m.cookieName)
	if err == nil && cookie.Value != "" {
		// 尝试获取现有会话
		m.mutex.RLock()
		session, exists := m.sessions[cookie.Value]
		m.mutex.RUnlock()

		if exists && time.Now().Before(session.ExpiresAt) {
			// 更新过期时间
			session.ExpiresAt = time.Now().Add(m.maxLifetime)
			return session, nil
		}
	}

	// 创建新会话
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	session := &Session{
		ID:        sessionID,
		Values:    make(map[string]interface{}),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(m.maxLifetime),
	}

	// 保存会话
	m.mutex.Lock()
	m.sessions[sessionID] = session
	m.mutex.Unlock()

	// 设置cookie
	newCookie := m.cookieOptions
	newCookie.Value = sessionID
	newCookie.Expires = session.ExpiresAt
	newCookie.MaxAge = int(m.maxLifetime.Seconds())
	http.SetCookie(w, &newCookie)

	return session, nil
}

// Get 获取现有会话
func (m *Manager) Get(r *http.Request) (*Session, error) {
	cookie, err := r.Cookie(m.cookieName)
	if err != nil {
		return nil, err
	}

	m.mutex.RLock()
	session, exists := m.sessions[cookie.Value]
	m.mutex.RUnlock()

	if !exists {
		return nil, errors.New("会话不存在")
	}

	if time.Now().After(session.ExpiresAt) {
		// 删除过期会话
		m.mutex.Lock()
		delete(m.sessions, cookie.Value)
		m.mutex.Unlock()
		return nil, errors.New("会话已过期")
	}

	return session, nil
}

// Destroy 销毁会话
func (m *Manager) Destroy(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(m.cookieName)
	if err != nil {
		return
	}

	m.mutex.Lock()
	delete(m.sessions, cookie.Value)
	m.mutex.Unlock()

	// 删除cookie
	expiredCookie := m.cookieOptions
	expiredCookie.Value = ""
	expiredCookie.Expires = time.Unix(0, 0)
	expiredCookie.MaxAge = -1
	http.SetCookie(w, &expiredCookie)
}

// GC 进行垃圾回收，清理过期会话
func (m *Manager) GC() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for id, session := range m.sessions {
		if time.Now().After(session.ExpiresAt) {
			delete(m.sessions, id)
		}
	}
}

// StartGCTimer 启动垃圾回收计时器
func (m *Manager) StartGCTimer() {
	go func() {
		ticker := time.NewTicker(time.Minute * 10) // 每10分钟清理一次
		for {
			<-ticker.C
			m.GC()
		}
	}()
}
