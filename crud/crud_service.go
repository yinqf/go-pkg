package crud

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (s *Service[T]) Paginate(ctx context.Context, page, size int, filters map[string][]string) ([]T, int64, error) {
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
	model := new(T)

	query := session.Model(model)

	if len(filters) > 0 {
		allowed := columnAllowlist(query, model)
		for column, vals := range filters {
			if len(vals) == 0 {
				continue
			}
			value := vals[0]
			if value == "" {
				continue
			}
			if !allowed[column] {
				continue
			}
			query = query.Where(clause.Eq{Column: clause.Column{Name: column}, Value: value})
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("id").Limit(size).Offset(offset).Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

var columnNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

func columnAllowlist(tx *gorm.DB, model interface{}) map[string]bool {
	columns := make(map[string]bool)
	if tx == nil {
		return columns
	}
	if err := tx.Statement.Parse(model); err == nil && tx.Statement.Schema != nil {
		for _, field := range tx.Statement.Schema.Fields {
			name := field.DBName
			if name == "" {
				name = field.Name
			}
			if columnNamePattern.MatchString(name) {
				columns[name] = true
			}
		}
		return columns
	}

	if tx.Migrator() == nil {
		return columns
	}

	if cols, err := tx.Migrator().ColumnTypes(model); err == nil {
		for _, col := range cols {
			if name := col.Name(); columnNamePattern.MatchString(name) {
				columns[name] = true
			}
		}
	}
	return columns
}
