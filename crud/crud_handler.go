package crud

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yinqf/go-pkg/response"
	"gorm.io/gorm"
)

// ServiceContract 描述了泛型 CRUD 处理器所依赖的服务能力。
type ServiceContract[T any] interface {
	SaveOrUpdate(ctx context.Context, entity *T) error
	DeleteByID(ctx context.Context, id string) error
	Paginate(ctx context.Context, page, size int) ([]T, int64, error)
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
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		response.ErrorWithStatus(c, http.StatusBadRequest, "page 必须为正整数")
		return
	}

	size, err := strconv.Atoi(c.DefaultQuery("size", "10"))
	if err != nil || size <= 0 {
		response.ErrorWithStatus(c, http.StatusBadRequest, "size 必须为正整数")
		return
	}

	items, total, svcErr := h.service.Paginate(c.Request.Context(), page, size)
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

// Bind 将通用 CRUD 路由绑定到指定的路由组。
