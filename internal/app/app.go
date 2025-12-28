package app

import "go.uber.org/zap"

type App struct {
	Logger       *zap.Logger
	Key          []byte
	TokenExpTime int
}

func NewApp(l *zap.Logger, key []byte, t int) *App {
	return &App{Logger: l, Key: key, TokenExpTime: t}
}
