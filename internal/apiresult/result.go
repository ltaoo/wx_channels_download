// Package apiresult provides the application's shared JSON response envelope.
package apiresult

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	success_code    = 0
	success_message = "成功"
)

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func Ok(ctx *gin.Context, data interface{}) {
	ctx.JSON(http.StatusOK, Response{
		Code: success_code,
		Msg:  success_message,
		Data: data,
	})
}

func Err(ctx *gin.Context, code int, msg string) {
	ctx.JSON(http.StatusOK, Response{
		Code: code,
		Msg:  msg,
	})
}

func ErrWithData(ctx *gin.Context, code int, msg string, data interface{}) {
	ctx.JSON(http.StatusOK, Response{
		Code: code,
		Msg:  msg,
		Data: data,
	})
}
