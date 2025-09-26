package app

import (
	"context"
	"os/signal"
	"syscall"
)

type App struct {
	srv        Runner
	difficulty int
}

func New(srv Runner, difficulty int) *App {
	return &App{srv: srv, difficulty: difficulty}
}

func (a *App) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return a.srv.Run(ctx, a.difficulty)
}
