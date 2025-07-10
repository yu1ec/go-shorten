package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	DataDir    = "data"
	RecordFile = "shorten_records.json"
	BackupDir  = "backups"
)

// URLRecord 表示一个短链接记录
type URLRecord struct {
	ShortCode  string    `json:"short_code"`
	TargetURL  string    `json:"target_url"`
	Remark     string    `json:"remark"`
	CreateTime time.Time `json:"create_time"`
}

// URLStorage 处理短链接的存储
type URLStorage struct {
	mutex      sync.RWMutex
	recordPath string
	backupPath string
	cache      map[string]*URLRecord
	lastBackup time.Time
	isDirty    bool
}

// NewURLStorage 创建一个新的URL存储实例
func NewURLStorage() (*URLStorage, error) {
	// 确保数据目录存在
	if err := os.MkdirAll(DataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 确保备份目录存在
	backupPath := filepath.Join(DataDir, BackupDir)
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return nil, fmt.Errorf("创建备份目录失败: %w", err)
	}

	storage := &URLStorage{
		recordPath: filepath.Join(DataDir, RecordFile),
		backupPath: backupPath,
		cache:      make(map[string]*URLRecord),
		lastBackup: time.Now(),
		isDirty:    false,
	}

	// 加载现有数据到缓存
	if err := storage.loadFromFile(); err != nil {
		return nil, fmt.Errorf("加载数据失败: %w", err)
	}

	// 启动定时备份
	go storage.startBackupScheduler()

	return storage, nil
}

// loadFromFile 从文件加载数据到缓存
func (s *URLStorage) loadFromFile() error {
	file, err := os.Open(s.recordPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	var records []URLRecord
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&records); err != nil {
		return err
	}

	s.cache = make(map[string]*URLRecord)
	for _, record := range records {
		recordCopy := record
		s.cache[record.ShortCode] = &recordCopy
	}

	return nil
}

// saveToFile 将缓存数据保存到文件
func (s *URLStorage) saveToFile() error {
	file, err := os.Create(s.recordPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var records []URLRecord
	for _, record := range s.cache {
		records = append(records, *record)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(records)
}

// startBackupScheduler 启动定时备份任务
func (s *URLStorage) startBackupScheduler() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mutex.RLock()
		needsBackup := s.isDirty
		s.mutex.RUnlock()

		if needsBackup {
			if err := s.createBackup(); err != nil {
				fmt.Printf("备份失败: %v\n", err)
			}
		}
	}
}

// createBackup 创建备份文件
func (s *URLStorage) createBackup() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.isDirty {
		return nil
	}

	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(s.backupPath, fmt.Sprintf("shorten_records_%s.json", timestamp))

	sourceFile, err := os.Open(s.recordPath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(backupFile)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	s.lastBackup = time.Now()
	s.isDirty = false
	return nil
}

// GetAllURLs 获取所有短链接记录
func (s *URLStorage) GetAllURLs() ([]URLRecord, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var result []URLRecord
	for _, record := range s.cache {
		result = append(result, *record)
	}

	return result, nil
}

// GetURLByCode 通过短码获取URL记录
func (s *URLStorage) GetURLByCode(code string) (*URLRecord, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	record, exists := s.cache[code]
	if !exists {
		return nil, errors.New("链接不存在")
	}

	recordCopy := *record
	return &recordCopy, nil
}

// CreateURL 创建新的短链接
func (s *URLStorage) CreateURL(record URLRecord) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.cache[record.ShortCode]; exists {
		return errors.New("短链接代码已存在")
	}

	record.CreateTime = time.Now()
	recordCopy := record
	s.cache[record.ShortCode] = &recordCopy
	s.isDirty = true

	return s.saveToFile()
}

// UpdateURL 更新现有的短链接
func (s *URLStorage) UpdateURL(record URLRecord) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	existing, exists := s.cache[record.ShortCode]
	if !exists {
		return errors.New("链接不存在")
	}

	record.CreateTime = existing.CreateTime
	recordCopy := record
	s.cache[record.ShortCode] = &recordCopy
	s.isDirty = true

	return s.saveToFile()
}

// DeleteURL 删除短链接
func (s *URLStorage) DeleteURL(shortCode string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.cache[shortCode]; !exists {
		return errors.New("链接不存在")
	}

	delete(s.cache, shortCode)
	s.isDirty = true

	return s.saveToFile()
}
