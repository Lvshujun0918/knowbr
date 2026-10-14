package store

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"knowbr/server/internal/models"
)

// ErrNotFound 表示记录不存在。
var ErrNotFound = errors.New("学习记录不存在")

// Store 封装 GORM 数据访问。
type Store struct {
	db *gorm.DB
}

// Open 打开（或创建）SQLite 数据库并完成迁移。
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&models.Session{}); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// DB 暴露底层连接，便于测试与扩展。
func (s *Store) DB() *gorm.DB { return s.db }

// Create 保存一条新记录。
func (s *Store) Create(m *models.Session) error { return s.db.Create(m).Error }

// Save 更新一条记录。
func (s *Store) Save(m *models.Session) error { return s.db.Save(m).Error }

// Get 按 ID 读取记录。
func (s *Store) Get(id string) (*models.Session, error) {
	var m models.Session
	err := s.db.First(&m, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// List 按创建时间倒序返回最近的学习记录。
func (s *Store) List(limit int) ([]models.Session, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var list []models.Session
	if err := s.db.Order("created_at DESC").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Delete 删除一条记录。
func (s *Store) Delete(id string) error {
	res := s.db.Delete(&models.Session{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAll 清空全部记录，返回删除条数。
func (s *Store) DeleteAll() (int64, error) {
	res := s.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.Session{})
	return res.RowsAffected, res.Error
}

// Count 返回记录总数。
func (s *Store) Count() (int64, error) {
	var n int64
	err := s.db.Model(&models.Session{}).Count(&n).Error
	return n, err
}

// Touch 更新记录时间戳。
func (s *Store) Touch(m *models.Session) {
	m.UpdatedAt = time.Now().UTC()
}
