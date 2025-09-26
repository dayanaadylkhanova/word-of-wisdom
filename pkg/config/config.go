package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ListenAddr    string
	PoWDifficulty int
	PoWTTL        time.Duration
	LogLevel      string
	ShutdownWait  time.Duration
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func atoi(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

func Parse() Config {
	ttl, _ := time.ParseDuration(getenv("POW_TTL", "60s"))
	wait, _ := time.ParseDuration(getenv("SHUTDOWN_WAIT", "5s"))
	return Config{
		ListenAddr:    getenv("LISTEN_ADDR", ":8080"),
		PoWDifficulty: atoi(getenv("POW_DIFFICULTY", "22"), 22),
		PoWTTL:        ttl,
		LogLevel:      getenv("LOG_LEVEL", "info"),
		ShutdownWait:  wait,
	}
}
