package app

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
)

func TestAppRun_DelegatesAndPassesDifficulty_GoMock(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mr := NewMockRunner(ctrl)

	var gotCtx context.Context
	var gotDifficulty int

	mr.EXPECT().
		Run(gomock.Any(), 42).
		DoAndReturn(func(ctx context.Context, difficulty int) error {
			gotCtx = ctx
			gotDifficulty = difficulty

			// Контекст не должен быть отменён преждевременно
			select {
			case <-ctx.Done():
				t.Fatalf("ctx was canceled prematurely")
			default:
			}
			return nil
		})

	a := New(mr, 42)

	if err := a.Run(); err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if gotCtx == nil {
		t.Fatalf("Runner.Run received nil ctx")
	}
	if gotDifficulty != 42 {
		t.Fatalf("difficulty passed = %d; want 42", gotDifficulty)
	}
}

func TestAppRun_PropagatesError_GoMock(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	wantErr := errors.New("boom")
	mr := NewMockRunner(ctrl)

	mr.EXPECT().
		Run(gomock.Any(), 7).
		Return(wantErr)

	a := New(mr, 7)

	err := a.Run()
	if err == nil {
		t.Fatalf("Run() expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v; want %v", err, wantErr)
	}
}

func TestAppRun_CancelsOnSignal_GracefulExit_GoMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mr := NewMockRunner(ctrl)

	mr.EXPECT().
		Run(gomock.Any(), 1).
		DoAndReturn(func(ctx context.Context, difficulty int) error {
			<-ctx.Done()
			return nil
		})

	a := New(mr, 1)

	done := make(chan error, 1)
	go func() { done <- a.Run() }()

	time.Sleep(50 * time.Millisecond)

	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("sending SIGINT failed: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned error on graceful cancel: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return after SIGINT")
	}
}
