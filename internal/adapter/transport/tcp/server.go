package tcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dayanaadylkhanova/word-of-wisdom/internal/entity"
)

type Server struct {
	log       *slog.Logger
	addr      string
	ttl       time.Duration
	pow       PoW
	quotes    Quote
	ln        net.Listener
	wg        sync.WaitGroup
	connsMu   sync.Mutex
	active    map[net.Conn]struct{}
	shutdownT time.Duration
}

func NewServer(log *slog.Logger, addr string, ttl time.Duration, shutdown time.Duration, pow PoW, quotes Quote) *Server {
	return &Server{
		log:       log,
		addr:      addr,
		ttl:       ttl,
		shutdownT: shutdown,
		pow:       pow,
		quotes:    quotes,
		active:    make(map[net.Conn]struct{}),
	}
}

func (s *Server) Run(ctx context.Context, difficulty int) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.ln = ln
	s.log.Info("server started", "addr", s.addr, "difficulty", difficulty, "ttl", s.ttl.String())

	errCh := make(chan error, 1)
	go func() { errCh <- s.acceptLoop(difficulty) }()

	select {
	case <-ctx.Done():

		s.log.Info("shutdown: closing listener")
		_ = s.ln.Close()

		s.connsMu.Lock()
		for c := range s.active {
			_ = c.SetDeadline(time.Now().Add(200 * time.Millisecond))
			if tc, ok := c.(*net.TCPConn); ok {
				_ = tc.CloseWrite()
			}
		}
		s.connsMu.Unlock()

		done := make(chan struct{})
		go func() { s.wg.Wait(); close(done) }()
		select {
		case <-done:
			s.log.Info("shutdown: all connections drained")
		case <-time.After(s.shutdownT):
			s.log.Warn("shutdown: force-close remaining connections")
			s.connsMu.Lock()
			for c := range s.active {
				_ = c.Close()
			}
			s.connsMu.Unlock()
		}
		return nil

	case err := <-errCh:
		return err
	}
}

func (s *Server) acceptLoop(difficulty int) error {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Temporary() {
				s.log.Warn("temporary accept error", "err", err)
				time.Sleep(50 * time.Millisecond)
				continue
			}
			return fmt.Errorf("accept: %w", err)
		}
		s.track(conn, true)
		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			defer s.track(c, false)
			s.handle(c, difficulty)
		}(conn)
	}
}

func (s *Server) track(c net.Conn, add bool) {
	s.connsMu.Lock()
	if add {
		s.active[c] = struct{}{}
	} else {
		delete(s.active, c)
	}
	s.connsMu.Unlock()
}

func (s *Server) handle(conn net.Conn, difficulty int) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * s.ttl))

	ch, err := s.pow.NewChallenge(difficulty, int64(s.ttl.Seconds()))
	if err != nil {
		s.log.Error("challenge create failed", "err", err)
		return
	}
	bw := bufio.NewWriter(conn)
	br := bufio.NewReader(conn)

	if payload, err := json.Marshal(ch); err == nil {
		_, _ = bw.Write(append(payload, '\n'))
		_ = bw.Flush()
	} else {
		s.log.Error("challenge marshal failed", "err", err)
		return
	}
	s.log.Debug("challenge issued",
		"remote", conn.RemoteAddr().String(),
		"expires", ch.Expires,
		"difficulty", ch.Difficulty,
	)

	line, err := br.ReadString('\n')
	if err != nil {
		s.log.Debug("read solution failed", "err", err)
		return
	}
	line = strings.TrimSpace(line)
	var sol entity.Solution
	if err := json.Unmarshal([]byte(line), &sol); err != nil || sol.Nonce == "" {
		_, _ = bw.WriteString("invalid solution json\n")
		_ = bw.Flush()
		s.log.Debug("bad solution", "err", err)
		return
	}

	if err := s.pow.Verify(ch, sol); err != nil {
		_, _ = bw.WriteString("pow verification failed\n")
		_ = bw.Flush()
		s.log.Debug("pow failed", "reason", err.Error())
		return
	}

	_, _ = bw.WriteString(s.quotes.Random() + "\n")
	_ = bw.Flush()
	s.log.Info("success", "remote", conn.RemoteAddr().String())
}
