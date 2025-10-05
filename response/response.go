package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
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

	write(c, status, status, msg, gin.H{})
}
