package api

import "errors"

var (
	ErrDBNotInitialized = errors.New("数据库未初始化")
	ErrInvalidInput     = errors.New("invalid input")
)
