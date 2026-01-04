package crud

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/yinqf/go-pkg/response"
	"github.com/yinqf/go-pkg/utils"
)

// ServiceContract 描述了泛型 CRUD 处理器所依赖的服务能力。
type ServiceContract[T any] interface {
	SaveOrUpdate(ctx context.Context, entity *T) error
	DeleteByID(ctx context.Context, id string) error
	Paginate(ctx context.Context, page, size int, filters map[string][]string, orders []OrderOption) ([]T, int64, error)
}

type Handler[T any] struct {
	service ServiceContract[T]
}

func NewHandler[T any](svc ServiceContract[T]) *Handler[T] {
	return &Handler[T]{service: svc}
}

func (h *Handler[T]) SaveOrUpdate(c *gin.Context) {
	var payload T
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.SaveOrUpdate(c.Request.Context(), &payload); err != nil {
		response.Error(c, err.Error())
		return
	}

	response.Success(c, payload)
}

func (h *Handler[T]) List(c *gin.Context) {
	page, size, err := utils.ParsePageAndSize(c)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, err.Error())
		return
	}

	rawQuery := c.Request.URL.Query()
	orders := utils.ParseOrderOptions(rawQuery)
	filters := make(map[string][]string, len(rawQuery))
	for key, values := range rawQuery {
		if key == "page" || key == "size" || key == "order" || key == "sort" || key == "order_by" || key == "orderBy" {
			continue
		}
		cleaned := make([]string, 0, len(values))
		for _, v := range values {
			if strings.TrimSpace(v) != "" {
				cleaned = append(cleaned, v)
			}
		}
		if len(cleaned) > 0 {
			filters[key] = cleaned
		}
	}

	items, total, svcErr := h.service.Paginate(c.Request.Context(), page, size, filters, orders)
	if svcErr != nil {
		response.Error(c, svcErr.Error())
		return
	}

	response.Success(c, gin.H{
		"list":  items,
		"page":  page,
		"size":  size,
		"total": total,
	})
}

func (h *Handler[T]) Delete(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		response.ErrorWithStatus(c, http.StatusBadRequest, "id is required")
		return
	}

	if err := h.service.DeleteByID(c.Request.Context(), id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.ErrorWithStatus(c, http.StatusNotFound, "记录不存在")
			return
		}

		response.Error(c, err.Error())
		return
	}

	response.Success(c, gin.H{"id": id})
}
