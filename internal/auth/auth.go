package auth

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

const (
	DataDir  = "data"
	UserFile = "users.json"
)

// User 表示系统用户
type User struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
	IsAdmin      bool   `json:"is_admin"`
}

// UserManager 管理用户认证
type UserManager struct {
	mutex    sync.RWMutex
	users    map[string]User
	userFile string
}

// NewUserManager 创建新的用户管理器
func NewUserManager() (*UserManager, error) {
	// 确保数据目录存在
	if err := os.MkdirAll(DataDir, 0755); err != nil {
		return nil, err
	}

	manager := &UserManager{
		users:    make(map[string]User),
		userFile: filepath.Join(DataDir, UserFile),
	}

	// 尝试加载用户数据
	if err := manager.loadUsers(); err != nil {
		// 如果是因为文件不存在，创建管理员账户
		if os.IsNotExist(err) {
			// 从环境变量获取默认用户名和密码
			username := os.Getenv("SHORTEN_AUTH_USER")
			password := os.Getenv("SHORTEN_AUTH_PASS")

			// 如果环境变量未设置，使用默认值
			if username == "" {
				username = "admin"
			}
			if password == "" {
				password = "admin"
			}

			// 创建管理员用户
			if err := manager.CreateUser(username, password, true); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return manager, nil
}

// 加载用户数据
func (m *UserManager) loadUsers() error {
	data, err := os.ReadFile(m.userFile)
	if err != nil {
		return err
	}

	var users []User
	if err := json.Unmarshal(data, &users); err != nil {
		return err
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 重新构建用户映射
	m.users = make(map[string]User)
	for _, user := range users {
		m.users[user.Username] = user
	}

	return nil
}

// CreateUser 创建新用户
func (m *UserManager) CreateUser(username, password string, isAdmin bool) error {
	if username == "" || password == "" {
		return errors.New("用户名和密码不能为空")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查用户是否已存在
	if _, exists := m.users[username]; exists {
		return errors.New("用户已存在")
	}

	// 生成密码哈希
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 添加用户
	m.users[username] = User{
		Username:     username,
		PasswordHash: string(hash),
		IsAdmin:      isAdmin,
	}

	return m.saveUsersUnlocked()
}

// saveUsersUnlocked 无需加锁的保存用户数据方法（假设调用者已经获取了锁）
func (m *UserManager) saveUsersUnlocked() error {
	// 转换为切片
	users := make([]User, 0, len(m.users))
	for _, user := range m.users {
		users = append(users, user)
	}

	// 序列化
	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}

	// 写入文件
	return os.WriteFile(m.userFile, data, 0644)
}

// Authenticate 验证用户凭据
func (m *UserManager) Authenticate(username, password string) (bool, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	user, exists := m.users[username]
	if !exists {
		return false, errors.New("用户不存在")
	}

	// 验证密码
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil, nil
}

// GetUser 获取用户信息
func (m *UserManager) GetUser(username string) (User, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	user, exists := m.users[username]
	if !exists {
		return User{}, errors.New("用户不存在")
	}

	return user, nil
}

// UpdatePassword 更新用户密码
func (m *UserManager) UpdatePassword(username, newPassword string) error {
	if newPassword == "" {
		return errors.New("密码不能为空")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	user, exists := m.users[username]
	if !exists {
		return errors.New("用户不存在")
	}

	// 生成新的密码哈希
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 更新用户
	user.PasswordHash = string(hash)
	m.users[username] = user

	return m.saveUsersUnlocked()
}

// DeleteUser 删除用户
func (m *UserManager) DeleteUser(username string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.users[username]; !exists {
		return errors.New("用户不存在")
	}

	delete(m.users, username)
	return m.saveUsersUnlocked()
}

// ListUsers 列出所有用户
func (m *UserManager) ListUsers() []User {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	users := make([]User, 0, len(m.users))
	for _, user := range m.users {
		// 不返回密码哈希
		users = append(users, User{
			Username: user.Username,
			IsAdmin:  user.IsAdmin,
		})
	}

	return users
}

// AuthenticateAPIKey 使用API密钥进行验证
func (m *UserManager) AuthenticateBasic(username, password string) bool {
	// 从环境变量获取
	envUser := os.Getenv("SHORTEN_AUTH_USER")
	envPass := os.Getenv("SHORTEN_AUTH_PASS")

	// 如果环境变量未设置，检查用户数据库
	if envUser == "" || envPass == "" {
		auth, err := m.Authenticate(username, password)
		return err == nil && auth
	}

	// 使用常量时间比较，防止计时攻击
	userMatch := subtle.ConstantTimeCompare([]byte(username), []byte(envUser)) == 1
	passMatch := subtle.ConstantTimeCompare([]byte(password), []byte(envPass)) == 1
	return userMatch && passMatch
}
