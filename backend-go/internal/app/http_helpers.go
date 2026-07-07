package app

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		badRequest(c, "请求参数无效")
		return false
	}
	return true
}

func businessConflict(message string) error {
	return BusinessConflict(message)
}

func (e BusinessConflict) Error() string {
	return string(e)
}

func isBusinessConflict(err error) bool {
	var target BusinessConflict
	return errors.As(err, &target)
}

func badRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, APIError{Message: message})
}

func conflict(c *gin.Context, message string) {
	c.JSON(http.StatusConflict, APIError{Message: message})
}

func serverError(c *gin.Context, err error) {
	log.Printf("server error: %v", err)
	c.JSON(http.StatusInternalServerError, APIError{Message: "服务器内部错误"})
}

func handleDBError(c *gin.Context, err error) {
	if isDuplicateEntry(err) {
		conflict(c, "数据冲突：相同记录可能已存在")
		return
	}
	serverError(c, err)
}

func queryInt(c *gin.Context, key string, fallback int) int {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
