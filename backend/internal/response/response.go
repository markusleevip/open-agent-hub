package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Unified response format
// Business error codes
const (
	CodeOK            = 0
	CodeBadRequest    = 40000
	CodeUnauthorized  = 40100
	CodeForbidden     = 40300
	CodeNotFound      = 40400
	CodeConflict      = 40900
	CodeInternal      = 50000
	CodeBusinessError = 50001
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeOK,
		Message: "success",
		Data:    data,
	})
}

func OKWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeOK,
		Message: message,
		Data:    data,
	})
}

func Fail(c *gin.Context, httpStatus, code int, message string) {
	c.AbortWithStatusJSON(httpStatus, Response{
		Code:    code,
		Message: message,
	})
}

func BadRequest(c *gin.Context, message string) {
	Fail(c, http.StatusBadRequest, CodeBadRequest, message)
}

func Unauthorized(c *gin.Context, message string) {
	Fail(c, http.StatusUnauthorized, CodeUnauthorized, message)
}

func Forbidden(c *gin.Context, message string) {
	Fail(c, http.StatusForbidden, CodeForbidden, message)
}

func NotFound(c *gin.Context, message string) {
	Fail(c, http.StatusNotFound, CodeNotFound, message)
}

func InternalError(c *gin.Context, message string) {
	Fail(c, http.StatusInternalServerError, CodeInternal, message)
}

func Error(c *gin.Context, httpStatus int, message string) {
	Fail(c, httpStatus, CodeBusinessError, message)
}
