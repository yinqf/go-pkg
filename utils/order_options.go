package utils

import (
	"strings"

	"github.com/yinqf/go-pkg/crud"
)

// ParseOrderOptions 解析排序参数。
func ParseOrderOptions(values map[string][]string) []crud.OrderOption {
	rawOrders := make([]string, 0, len(values))
	for _, key := range []string{"order", "sort", "order_by", "orderBy"} {
		if entries, ok := values[key]; ok {
			rawOrders = append(rawOrders, entries...)
		}
	}

	options := make([]crud.OrderOption, 0, len(rawOrders))
	for _, raw := range rawOrders {
		opt, ok := parseOrderOption(raw)
		if ok {
			options = append(options, opt)
		}
	}
	return options
}

func parseOrderOption(raw string) (crud.OrderOption, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return crud.OrderOption{}, false
	}

	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ' ' || r == ':' || r == ','
	})

	if len(parts) == 0 {
		return crud.OrderOption{}, false
	}

	column := strings.TrimSpace(parts[0])
	if column == "" {
		return crud.OrderOption{}, false
	}

	desc := false
	if strings.HasPrefix(column, "-") {
		column = strings.TrimPrefix(column, "-")
		desc = true
	} else if strings.HasPrefix(column, "+") {
		column = strings.TrimPrefix(column, "+")
	}

	if column == "" {
		return crud.OrderOption{}, false
	}

	if len(parts) > 1 {
		switch strings.ToLower(strings.TrimSpace(parts[1])) {
		case "desc", "descend", "descending":
			desc = true
		default:
			desc = false
		}
	}

	return crud.OrderOption{Column: column, Desc: desc}, true
}
