package tcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dayanaadylkhanova/word-of-wisdom/internal/entity"
	"go.uber.org/mock/gomock"
)

func loggerSilent() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func mustPipe(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	c1, c2 := net.Pipe()
	// дедлайны, чтобы не повиснуть при падении теста
	_ = c1.SetDeadline(time.Now().Add(2 * time.Second))
	_ = c2.SetDeadline(time.Now().Add(2 * time.Second))
	return c1, c2
}

func TestHandle_PowExpired_TreatedAsPowFail(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPow := NewMockPoW(ctrl)
	mockQuote := NewMockQuote(ctrl)

	ch := entity.Challenge{
		Version:    1,
		Algo:       "sha256-leading-zero-bits",
		Difficulty: 22,
		SaltB64:    "c2FsdA==",                              // "salt"
		Expires:    time.Now().Add(-1 * time.Second).Unix(), // просрочен для реализма (но главное — Verify вернёт ошибку)
	}

	mockPow.EXPECT().NewChallenge(gomock.Any(), gomock.Any()).Return(ch, nil)
	mockPow.EXPECT().Verify(ch, gomock.Any()).Return(errors.New("challenge expired"))
	// Quote не должен вызываться

	srv := NewServer(loggerSilent(), "ignored:0", time.Minute, 200*time.Millisecond, mockPow, mockQuote)

	cli, srvSide := mustPipe(t)
	defer cli.Close()
	defer srvSide.Close()

	go srv.handle(srvSide, 22)

	br := bufio.NewReader(cli)
	bw := bufio.NewWriter(cli)

	// challenge
	if _, err := br.ReadString('\n'); err != nil {
		t.Fatalf("read challenge: %v", err)
	}

	// отправляем любое решение
	if _, err := bw.WriteString(`{"nonce":"deadbeef"}` + "\n"); err != nil {
		t.Fatalf("write solution: %v", err)
	}
	_ = bw.Flush()

	// ожидаем унифицированный ответ про провал PoW
	reply, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read pow-fail reply: %v", err)
	}
	if want := "pow verification failed\n"; reply != want {
		t.Fatalf("reply = %q; want %q", reply, want)
	}
}

func TestHandle_ParallelMany(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPow := NewMockPoW(ctrl)
	mockQuote := NewMockQuote(ctrl)

	ch := entity.Challenge{
		Version:    1,
		Algo:       "sha256-leading-zero-bits",
		Difficulty: 1,
		SaltB64:    "c2FsdA==",
		Expires:    time.Now().Add(1 * time.Minute).Unix(),
	}

	const N = 10

	// ожидаем по N обращений
	mockPow.EXPECT().NewChallenge(gomock.Any(), gomock.Any()).Times(N).Return(ch, nil)
	mockPow.EXPECT().Verify(ch, gomock.Any()).Times(N).Return(nil)
	mockQuote.EXPECT().Random().Times(N).Return("ok")

	srv := NewServer(loggerSilent(), "ignored:0", time.Minute, 200*time.Millisecond, mockPow, mockQuote)

	var wg sync.WaitGroup
	wg.Add(N)

	for i := 0; i < N; i++ {
		cli, srvSide := mustPipe(t)

		go func(c1, c2 net.Conn) {
			defer wg.Done()
			defer c1.Close()
			defer c2.Close()

			// серверная сторона
			go srv.handle(c2, 10)

			// клиентская сторона
			br := bufio.NewReader(c1)
			bw := bufio.NewWriter(c1)

			// challenge
			if _, err := br.ReadString('\n'); err != nil {
				t.Errorf("read challenge: %v", err)
				return
			}
			// solution
			if _, err := bw.WriteString(`{"nonce":"00"}` + "\n"); err != nil {
				t.Errorf("write solution: %v", err)
				return
			}
			_ = bw.Flush()

			// quote
			if reply, err := br.ReadString('\n'); err != nil {
				t.Errorf("read quote: %v", err)
			} else if reply != "ok\n" {
				t.Errorf("quote = %q; want %q", reply, "ok\n")
			}
		}(cli, srvSide)
	}

	wg.Wait()
}

func TestHandle_OK(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPow := NewMockPoW(ctrl)
	mockQuote := NewMockQuote(ctrl)

	ch := entity.Challenge{
		Version:    1,
		Algo:       "sha256-leading-zero-bits",
		Difficulty: 1,
		SaltB64:    "c2FsdA==", // "salt"
		Expires:    time.Now().Add(1 * time.Minute).Unix(),
	}

	mockPow.EXPECT().NewChallenge(gomock.Any(), gomock.Any()).Return(ch, nil)
	mockPow.EXPECT().Verify(ch, gomock.Any()).Return(nil)
	mockQuote.EXPECT().Random().Return("hello, world")

	srv := NewServer(loggerSilent(), "ignored:0", time.Minute, 200*time.Millisecond, mockPow, mockQuote)

	cli, srvSide := mustPipe(t)
	defer cli.Close()
	defer srvSide.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.handle(srvSide, 10)
	}()

	br := bufio.NewReader(cli)
	bw := bufio.NewWriter(cli)

	// 1) читаем challenge
	line, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read challenge: %v", err)
	}
	var gotCh entity.Challenge
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &gotCh); err != nil {
		t.Fatalf("unmarshal challenge: %v", err)
	}

	// 2) отправляем решение (валидность проверит мок Verify)
	if _, err := bw.WriteString(`{"nonce":"00"}` + "\n"); err != nil {
		t.Fatalf("write solution: %v", err)
	}
	_ = bw.Flush()

	// 3) читаем цитату
	reply, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read quote: %v", err)
	}
	if want := "hello, world\n"; reply != want {
		t.Fatalf("quote = %q; want %q", reply, want)
	}

	<-done
}

func TestHandle_InvalidJSON(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPow := NewMockPoW(ctrl)
	mockQuote := NewMockQuote(ctrl)

	ch := entity.Challenge{
		Version:    1,
		Algo:       "sha256-leading-zero-bits",
		Difficulty: 1,
		SaltB64:    "c2FsdA==",
		Expires:    time.Now().Add(1 * time.Minute).Unix(),
	}

	mockPow.EXPECT().NewChallenge(gomock.Any(), gomock.Any()).Return(ch, nil)

	srv := NewServer(loggerSilent(), "ignored:0", time.Minute, 200*time.Millisecond, mockPow, mockQuote)

	cli, srvSide := mustPipe(t)
	defer cli.Close()
	defer srvSide.Close()

	go srv.handle(srvSide, 10)

	br := bufio.NewReader(cli)
	bw := bufio.NewWriter(cli)

	// challenge
	if _, err := br.ReadString('\n'); err != nil {
		t.Fatalf("read challenge: %v", err)
	}

	// отправляем некорректный JSON
	if _, err := bw.WriteString("not a json\n"); err != nil {
		t.Fatalf("write bad json: %v", err)
	}
	_ = bw.Flush()

	// ждём ответ об ошибке
	reply, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read error reply: %v", err)
	}
	if want := "invalid solution json\n"; reply != want {
		t.Fatalf("reply = %q; want %q", reply, want)
	}
}

func TestHandle_PowFail(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPow := NewMockPoW(ctrl)
	mockQuote := NewMockQuote(ctrl)

	ch := entity.Challenge{
		Version:    1,
		Algo:       "sha256-leading-zero-bits",
		Difficulty: 22,
		SaltB64:    "c2FsdA==",
		Expires:    time.Now().Add(1 * time.Minute).Unix(),
	}

	mockPow.EXPECT().NewChallenge(gomock.Any(), gomock.Any()).Return(ch, nil)
	mockPow.EXPECT().Verify(ch, gomock.Any()).Return(errors.New("pow invalid"))
	// Quote не вызывается

	srv := NewServer(loggerSilent(), "ignored:0", time.Minute, 200*time.Millisecond, mockPow, mockQuote)

	cli, srvSide := mustPipe(t)
	defer cli.Close()
	defer srvSide.Close()

	go srv.handle(srvSide, 22)

	br := bufio.NewReader(cli)
	bw := bufio.NewWriter(cli)

	// challenge
	if _, err := br.ReadString('\n'); err != nil {
		t.Fatalf("read challenge: %v", err)
	}

	// отправляем любое решение
	if _, err := bw.WriteString(`{"nonce":"deadbeef"}` + "\n"); err != nil {
		t.Fatalf("write solution: %v", err)
	}
	_ = bw.Flush()

	// ожидаем ответ о провале pow
	reply, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read pow-fail reply: %v", err)
	}
	if want := "pow verification failed\n"; reply != want {
		t.Fatalf("reply = %q; want %q", reply, want)
	}
}

func TestRun_GracefulShutdown(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPow := NewMockPoW(ctrl)
	mockQuote := NewMockQuote(ctrl)

	ch := entity.Challenge{Version: 1, Algo: "sha256-leading-zero-bits", Difficulty: 1, SaltB64: "c2FsdA==", Expires: time.Now().Add(1 * time.Minute).Unix()}
	mockPow.EXPECT().NewChallenge(gomock.Any(), gomock.Any()).AnyTimes().Return(ch, nil)
	mockPow.EXPECT().Verify(ch, gomock.Any()).AnyTimes().Return(nil)
	mockQuote.EXPECT().Random().AnyTimes().Return("ok")

	addr := freeTCPAddr(t) // <— вот тут
	srv := NewServer(loggerSilent(), addr, 500*time.Millisecond, 200*time.Millisecond, mockPow, mockQuote)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(ctx, 10) }()

	// ждём, пока порт начнёт слушаться
	deadline := time.Now().Add(2 * time.Second)
	var conn net.Conn
	var err error
	for time.Now().Before(deadline) {
		conn, err = net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("server did not start listening on %s: %v", addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	// читаем challenge
	br := bufio.NewReader(conn)
	if _, err := br.ReadString('\n'); err != nil {
		t.Fatalf("read challenge: %v", err)
	}
	_ = conn.(*net.TCPConn).CloseWrite()

	// триггернем shutdown
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned error on shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}

func freeTCPAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen temp: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}
