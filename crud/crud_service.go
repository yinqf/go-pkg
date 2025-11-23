package crud

import (
	"context"
	"errors"
	"reflect"
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

// OrderOption 描述单个排序条件。
type OrderOption struct {
	Column string
	Desc   bool
}

func (s *Service[T]) SaveOrUpdate(ctx context.Context, entity *T) error {
	if entity == nil {
		return errors.New("entity is nil")
	}

	session := s.db.WithContext(ctx)
	stmt := &gorm.Statement{DB: session, Context: ctx}
	if err := stmt.Parse(entity); err != nil {
		return err
	}

	schema := stmt.Schema
	if schema == nil {
		return errors.New("failed to parse schema")
	}

	primary := schema.PrioritizedPrimaryField
	if primary == nil {
		return errors.New("primary key is not defined")
	}

	value := reflect.ValueOf(entity)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return errors.New("entity must be a non-nil pointer")
	}
	elem := value.Elem()

	_, zeroPK := primary.ValueOf(ctx, elem)
	if zeroPK {
		return session.Create(entity).Error
	}

	columns := make([]string, 0, len(schema.Fields))
	for _, field := range schema.Fields {
		if !field.Updatable || field.DBName == "" || field == primary {
			continue
		}

		if field.AutoUpdateTime > 0 {
			columns = append(columns, field.DBName)
			continue
		}

		if _, zero := field.ValueOf(ctx, elem); zero {
			continue
		}

		columns = append(columns, field.DBName)
	}

	if len(columns) == 0 {
		return nil
	}

	return session.Model(entity).Select(columns).Updates(entity).Error
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

func (s *Service[T]) Paginate(ctx context.Context, page, size int, filters map[string][]string, orders []OrderOption) ([]T, int64, error) {
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
	allowed := columnAllowlist(query, model)

	if len(filters) > 0 {
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

	orderBy := sanitizeOrders(orders, allowed)
	if len(orderBy) == 0 {
		query = query.Order("id")
	} else {
		for _, opt := range orderBy {
			query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: opt.Column}, Desc: opt.Desc})
		}
	}

	if err := query.Limit(size).Offset(offset).Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

func sanitizeOrders(orders []OrderOption, allowed map[string]bool) []OrderOption {
	if len(orders) == 0 || len(allowed) == 0 {
		return nil
	}

	result := make([]OrderOption, 0, len(orders))
	seen := make(map[string]struct{}, len(orders))
	for _, opt := range orders {
		column := strings.TrimSpace(opt.Column)
		if column == "" || !allowed[column] {
			continue
		}
		if _, ok := seen[column]; ok {
			continue
		}
		result = append(result, OrderOption{Column: column, Desc: opt.Desc})
		seen[column] = struct{}{}
	}
	return result
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
