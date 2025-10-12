package crud

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// Service 用于封装带主键实体的通用增删改查能力。
type Service[T any] struct {
	db *gorm.DB
}

func NewService[T any](db *gorm.DB) *Service[T] {
	return &Service[T]{db: db}
}

func (s *Service[T]) SaveOrUpdate(ctx context.Context, entity *T) error {
	return s.db.WithContext(ctx).Save(entity).Error
}

func (s *Service[T]) DeleteByID(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("id is required")
	}

	session := s.db.WithContext(ctx)
	var result *gorm.DB

	if numericID, err := strconv.ParseUint(id, 10, 64); err == nil {
		result = session.Delete(new(T), numericID)
	} else {
		result = session.Where("id = ?", id).Delete(new(T))
	}
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (s *Service[T]) Paginate(ctx context.Context, page, size int) ([]T, int64, error) {
	if page < 1 {
		page = 1
	}

	if size <= 0 {
		size = 10
	}

	offset := (page - 1) * size

	var (
		list  []T
		total int64
	)

	session := s.db.WithContext(ctx)

	if err := session.Model(new(T)).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := session.Model(new(T)).Order("id").Limit(size).Offset(offset).Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}
