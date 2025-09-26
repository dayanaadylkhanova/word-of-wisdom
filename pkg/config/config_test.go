package config

import (
	"os"
	"testing"
	"time"
)

func TestParse_Defaults_WhenEnvMissing(t *testing.T) {
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("POW_DIFFICULTY", "")
	t.Setenv("POW_TTL", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("SHUTDOWN_WAIT", "")

	cfg := Parse()

	if cfg.ListenAddr != ":8080" {
		t.Fatalf("ListenAddr=%q; want :8080", cfg.ListenAddr)
	}
	// atoi(...) с пустым => дефолт 22
	if cfg.PoWDifficulty != 22 {
		t.Fatalf("PoWDifficulty=%d; want 22", cfg.PoWDifficulty)
	}
	// при пустом POW_TTL используется дефолт "60s"
	if cfg.PoWTTL != 60*time.Second {
		t.Fatalf("PoWTTL=%v; want 60s", cfg.PoWTTL)
	}
	// дефолт LOG_LEVEL
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel=%q; want info", cfg.LogLevel)
	}
	// дефолт SHUTDOWN_WAIT = 5s
	if cfg.ShutdownWait != 5*time.Second {
		t.Fatalf("ShutdownWait=%v; want 5s", cfg.ShutdownWait)
	}
}

func TestParse_ValidDurations(t *testing.T) {
	t.Setenv("POW_TTL", "90s")
	t.Setenv("SHUTDOWN_WAIT", "1500ms")
	t.Setenv("POW_DIFFICULTY", "17")

	cfg := Parse()

	if cfg.PoWTTL != 90*time.Second {
		t.Fatalf("PoWTTL=%v; want 90s", cfg.PoWTTL)
	}
	if cfg.ShutdownWait != 1500*time.Millisecond {
		t.Fatalf("ShutdownWait=%v; want 1500ms", cfg.ShutdownWait)
	}
	if cfg.PoWDifficulty != 17 {
		t.Fatalf("PoWDifficulty=%d; want 17", cfg.PoWDifficulty)
	}
}

func TestParse_InvalidValues_CurrentBehavior(t *testing.T) {
	// Невалидные строки: ParseDuration ошибки игнорит -> ноль.
	t.Setenv("POW_TTL", "oops")
	t.Setenv("SHUTDOWN_WAIT", "nope")
	// Невалидная сложность -> atoi вернёт дефолт 22
	t.Setenv("POW_DIFFICULTY", "abc")

	// Остальные пустые
	os.Unsetenv("LISTEN_ADDR")
	os.Unsetenv("LOG_LEVEL")

	cfg := Parse()

	if cfg.PoWTTL != 0 {
		t.Fatalf("PoWTTL=%v; want 0 (текущее поведение при невалидном значении)", cfg.PoWTTL)
	}
	if cfg.ShutdownWait != 0 {
		t.Fatalf("ShutdownWait=%v; want 0 (текущее поведение при невалидном значении)", cfg.ShutdownWait)
	}
	if cfg.PoWDifficulty != 22 {
		t.Fatalf("PoWDifficulty=%d; want дефолт 22 при невалидной строке", cfg.PoWDifficulty)
	}
}
