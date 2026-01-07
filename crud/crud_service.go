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
	query = ApplyFilters(query, filters, allowed)

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

type filterOp string

const (
	filterEq      filterOp = "eq"
	filterNe      filterOp = "ne"
	filterGt      filterOp = "gt"
	filterGte     filterOp = "gte"
	filterLt      filterOp = "lt"
	filterLte     filterOp = "lte"
	filterLike    filterOp = "like"
	filterIn      filterOp = "in"
	filterNotIn   filterOp = "nin"
	filterBetween filterOp = "between"
	filterIsNull  filterOp = "isnull"
	filterNotNull filterOp = "notnull"
)

// ApplyFilters 根据通用筛选语法构建查询条件。
func ApplyFilters(query *gorm.DB, filters map[string][]string, allowed map[string]bool) *gorm.DB {
	if query == nil || len(filters) == 0 {
		return query
	}

	for key, vals := range filters {
		column, op := parseFilterKey(key)
		if column == "" || !allowed[column] {
			continue
		}

		values := normalizeFilterValues(vals)
		if len(values) == 0 && op != filterIsNull && op != filterNotNull {
			continue
		}

		columnExpr := clause.Column{Name: column}
		switch op {
		case filterEq:
			query = query.Where(clause.Eq{Column: columnExpr, Value: values[0]})
		case filterNe:
			query = query.Where(clause.Neq{Column: columnExpr, Value: values[0]})
		case filterGt:
			query = query.Where(clause.Gt{Column: columnExpr, Value: values[0]})
		case filterGte:
			query = query.Where(clause.Gte{Column: columnExpr, Value: values[0]})
		case filterLt:
			query = query.Where(clause.Lt{Column: columnExpr, Value: values[0]})
		case filterLte:
			query = query.Where(clause.Lte{Column: columnExpr, Value: values[0]})
		case filterLike:
			value := values[0]
			if value != "" && !strings.ContainsAny(value, "%_") {
				value = "%" + value + "%"
			}
			if value != "" {
				query = query.Where(clause.Expr{SQL: "? LIKE ?", Vars: []interface{}{columnExpr, value}})
			}
		case filterIn:
			list := splitCommaValues(values)
			if len(list) > 0 {
				query = query.Where(clause.IN{Column: columnExpr, Values: toInterfaceSlice(list)})
			}
		case filterNotIn:
			list := splitCommaValues(values)
			if len(list) > 0 {
				query = query.Not(clause.IN{Column: columnExpr, Values: toInterfaceSlice(list)})
			}
		case filterBetween:
			parts := splitCommaValues(values)
			if len(parts) >= 2 {
				start := strings.TrimSpace(parts[0])
				end := strings.TrimSpace(parts[1])
				if start != "" && end != "" {
					query = query.Where(clause.Gte{Column: columnExpr, Value: start}).
						Where(clause.Lte{Column: columnExpr, Value: end})
				}
			}
		case filterIsNull:
			if isNull, ok := parseBoolValue(firstValue(values)); !ok || isNull {
				query = query.Where(clause.Expr{SQL: "? IS NULL", Vars: []interface{}{columnExpr}})
			} else {
				query = query.Where(clause.Expr{SQL: "? IS NOT NULL", Vars: []interface{}{columnExpr}})
			}
		case filterNotNull:
			query = query.Where(clause.Expr{SQL: "? IS NOT NULL", Vars: []interface{}{columnExpr}})
		}
	}

	return query
}

func parseFilterKey(key string) (string, filterOp) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return "", filterEq
	}

	if idx := strings.LastIndex(trimmed, "__"); idx > 0 && idx < len(trimmed)-2 {
		column := trimmed[:idx]
		rawOp := trimmed[idx+2:]
		if op, ok := normalizeFilterOp(rawOp); ok {
			return column, op
		}
	}

	return trimmed, filterEq
}

func normalizeFilterOp(raw string) (filterOp, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "eq", "=":
		return filterEq, true
	case "ne", "neq", "!=", "<>":
		return filterNe, true
	case "gt", ">":
		return filterGt, true
	case "gte", "ge", ">=":
		return filterGte, true
	case "lt", "<":
		return filterLt, true
	case "lte", "le", "<=":
		return filterLte, true
	case "like", "contains", "contain":
		return filterLike, true
	case "in":
		return filterIn, true
	case "nin", "notin", "not_in":
		return filterNotIn, true
	case "between", "range":
		return filterBetween, true
	case "isnull", "null":
		return filterIsNull, true
	case "notnull", "not_null", "isnotnull":
		return filterNotNull, true
	default:
		return "", false
	}
}

func normalizeFilterValues(vals []string) []string {
	if len(vals) == 0 {
		return nil
	}
	result := make([]string, 0, len(vals))
	for _, raw := range vals {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func splitFilterValues(vals []string) []string {
	if len(vals) == 0 {
		return nil
	}
	result := make([]string, 0, len(vals))
	for _, raw := range vals {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func splitCommaValues(vals []string) []string {
	if len(vals) == 0 {
		return nil
	}
	result := make([]string, 0, len(vals))
	for _, raw := range vals {
		for _, part := range strings.Split(raw, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			result = append(result, trimmed)
		}
	}
	return result
}

func toInterfaceSlice(vals []string) []interface{} {
	result := make([]interface{}, 0, len(vals))
	for _, val := range vals {
		result = append(result, val)
	}
	return result
}

func parseBoolValue(raw string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "y", "on":
		return true, true
	case "0", "false", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

func firstValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
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
