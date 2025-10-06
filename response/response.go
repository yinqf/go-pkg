package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinqf/go-pkg/logger"
	"go.uber.org/zap"
)

const SuccessCode = 0

type Body struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func write(c *gin.Context, status, code int, msg string, data interface{}) {
	c.JSON(status, Body{
		Code:    code,
		Message: msg,
		Data:    data,
	})
}

func Success(c *gin.Context, data interface{}) {
	if data == nil {
		data = gin.H{}
	}

	write(c, http.StatusOK, SuccessCode, "OK", data)
}

func Error(c *gin.Context, msg string) {
	ErrorWithStatus(c, http.StatusInternalServerError, msg)
}

func ErrorWithStatus(c *gin.Context, status int, msg string) {
	if status < http.StatusBadRequest {
		status = http.StatusInternalServerError
	}

	if msg == "" {
		msg = http.StatusText(status)
	}

	method := ""
	requestURI := ""
	if c.Request != nil {
		method = c.Request.Method
		requestURI = c.Request.RequestURI
	}

	logger.Error(
		"请求处理失败",
		zap.Int("status", status),
		zap.String("message", msg),
		zap.String("method", method),
		zap.String("path", c.FullPath()),
		zap.String("uri", requestURI),
		zap.String("client_ip", c.ClientIP()),
	)

	write(c, status, status, msg, gin.H{})
}
