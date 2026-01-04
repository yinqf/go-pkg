package utils

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ParsePageAndSize 解析分页参数，返回 page/size 或错误。
func ParsePageAndSize(c *gin.Context) (int, int, error) {
	if c == nil {
		return 0, 0, errors.New("invalid context")
	}

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		return 0, 0, errors.New("page 必须为正整数")
	}

	size, err := strconv.Atoi(c.DefaultQuery("size", "10"))
	if err != nil || size <= 0 {
		return 0, 0, errors.New("size 必须为正整数")
	}

	return page, size, nil
}
