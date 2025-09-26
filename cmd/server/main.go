package main

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/dayanaadylkhanova/word-of-wisdom/internal/adapter/quote"
	"github.com/dayanaadylkhanova/word-of-wisdom/internal/adapter/transport/tcp"
	"github.com/dayanaadylkhanova/word-of-wisdom/internal/service"
	"github.com/dayanaadylkhanova/word-of-wisdom/pkg/config"
	"github.com/dayanaadylkhanova/word-of-wisdom/pkg/logger"
)

func main() {
	cfg := config.Parse()
	
	log := logger.NewJSON(logger.LevelFromEnv(cfg.LogLevel))

	pow := service.NewHashcash()
	qt := quote.NewStatic()
	srv := tcp.NewServer(log, cfg.ListenAddr, cfg.PoWTTL, cfg.ShutdownWait, pow, qt)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := srv.Run(ctx, cfg.PoWDifficulty); err != nil {
		log.Error("server stopped with error", slog.Any("err", err))
	}
}
