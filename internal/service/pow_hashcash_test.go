package service

import (
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dayanaadylkhanova/word-of-wisdom/internal/entity"
)

func TestLeadingZeroBits_Table(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   []byte
		want int
	}{
		{"all_zero_1byte", []byte{0x00}, 8},
		{"all_zero_2bytes", []byte{0x00, 0x00}, 16},
		{"0x0f", []byte{0x0f}, 4},
		{"0xf0", []byte{0xf0}, 0},
		{"0x00_0x1f", []byte{0x00, 0x1f}, 8 + 3},
		{"0x7f", []byte{0x7f}, 1},
		{"0x01", []byte{0x01}, 7},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := leadingZeroBits(tc.in)
			if got != tc.want {
				t.Fatalf("leadingZeroBits(% X) = %d; want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestPowMessage_Format(t *testing.T) {
	t.Parallel()

	salt := []byte("salt")
	ex := int64(123)
	nonce := "0abc" // Verify приводит nonce к lower-case

	got := powMessage(salt, ex, nonce)
	want := []byte("salt:123:0abc")
	if string(got) != string(want) {
		t.Fatalf("powMessage() = %q; want %q", string(got), string(want))
	}
}

func TestVerify_Table(t *testing.T) {
	t.Parallel()

	h := NewHashcash()

	// Детерминированный challenge (фиксированная соль и время)
	salt := []byte("fixed-salt")
	saltB64 := base64.StdEncoding.EncodeToString(salt)
	now := time.Now()
	baseCh := entity.Challenge{
		Version:    1,
		Algo:       "sha256-leading-zero-bits",
		Difficulty: 12, // низкая сложность для быстрого подбора
		SaltB64:    saltB64,
		Expires:    now.Add(1 * time.Minute).Unix(),
	}

	// Подбираем валидный nonce для baseCh
	findNonce := func(ch entity.Challenge) string {
		expStr := strconv.FormatInt(ch.Expires, 10)
		for i := 0; ; i++ {
			n := strings.ToLower(strconv.FormatInt(int64(i), 16))
			// payload = salt : expires : nonce_lower
			payload := make([]byte, 0, len(salt)+1+len(expStr)+1+len(n))
			payload = append(payload, salt...)
			payload = append(payload, ':')
			payload = append(payload, []byte(expStr)...)
			payload = append(payload, ':')
			payload = append(payload, []byte(n)...)
			sum := sha256.Sum256(payload)
			if leadingZeroBits(sum[:]) >= ch.Difficulty {
				return n
			}
			// защитный порог
			if i > 1<<22 {
				t.Fatal("failed to find nonce in reasonable time; reduce difficulty")
			}
		}
	}

	validNonce := findNonce(baseCh)

	// Считаем реальное количество ведущих нулевых бит для найденного nonce,
	// чтобы корректно проверить границу и +1.
	expStr := strconv.FormatInt(baseCh.Expires, 10)
	{
		payload := make([]byte, 0, len(salt)+1+len(expStr)+1+len(validNonce))
		payload = append(payload, salt...)
		payload = append(payload, ':')
		payload = append(payload, []byte(expStr)...)
		payload = append(payload, ':')
		payload = append(payload, []byte(validNonce)...)
		sum := sha256.Sum256(payload)
		actualBits := leadingZeroBits(sum[:])

		cases := []struct {
			name    string
			ch      entity.Challenge
			nonce   string
			wantErr string // подстрока ошибки; пусто = nil
		}{
			{"ok_valid_nonce", baseCh, validNonce, ""},
			{"bad_nonce", baseCh, "deadbeef", "pow invalid"},
			{
				"expired",
				func() entity.Challenge {
					c := baseCh
					c.Expires = now.Add(-1 * time.Second).Unix()
					return c
				}(),
				validNonce,
				"challenge expired",
			},
			{
				"unsupported_algo",
				func() entity.Challenge { c := baseCh; c.Algo = "md5"; return c }(),
				validNonce,
				"unsupported challenge",
			},
			{
				"unsupported_version",
				func() entity.Challenge { c := baseCh; c.Version = 2; return c }(),
				validNonce,
				"unsupported challenge",
			},
			{
				"bad_salt",
				func() entity.Challenge { c := baseCh; c.SaltB64 = "!!!not-base64!!!"; return c }(),
				validNonce,
				"bad salt",
			},
			{
				"boundary_pass_exact_bits",
				func() entity.Challenge { c := baseCh; c.Difficulty = actualBits; return c }(),
				validNonce,
				"",
			},
			{
				"boundary_fail_plus_one",
				func() entity.Challenge { c := baseCh; c.Difficulty = actualBits + 1; return c }(),
				validNonce,
				"pow invalid",
			},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				err := h.Verify(tc.ch, entity.Solution{Nonce: tc.nonce})
				if tc.wantErr == "" && err != nil {
					t.Fatalf("Verify() unexpected error: %v", err)
				}
				if tc.wantErr != "" {
					if err == nil {
						t.Fatalf("Verify() expected error containing %q, got nil", tc.wantErr)
					}
					if !strings.Contains(err.Error(), tc.wantErr) {
						t.Fatalf("Verify() error %q; want contains %q", err.Error(), tc.wantErr)
					}
				}
			})
		}
	}
}

func TestNewChallenge_BasicFields(t *testing.T) {
	t.Parallel()

	h := NewHashcash()
	const ttl = int64(90)
	const diff = 10

	before := time.Now()
	ch, err := h.NewChallenge(diff, ttl)
	after := time.Now()
	if err != nil {
		t.Fatalf("NewChallenge() error: %v", err)
	}

	// Алгоритм/версия/сложность
	if ch.Algo != "sha256-leading-zero-bits" || ch.Version != 1 {
		t.Fatalf("unexpected algo/version: %s/%d", ch.Algo, ch.Version)
	}
	if ch.Difficulty != diff {
		t.Fatalf("difficulty = %d; want %d", ch.Difficulty, diff)
	}

	// Соль — валидная base64
	if _, err := base64.StdEncoding.DecodeString(ch.SaltB64); err != nil {
		t.Fatalf("SaltB64 not base64: %v", err)
	}

	// Expires ≈ (время вызова) + ttl с допуском
	ttlDur := time.Duration(ttl) * time.Second
	exp := time.Unix(ch.Expires, 0)

	// Допуск ±1s относительно before/after
	min := before.Add(ttlDur).Add(-1 * time.Second)
	max := after.Add(ttlDur).Add(1 * time.Second)
	if exp.Before(min) || exp.After(max) {
		t.Fatalf("expires out of expected range: got %v; want [%v, %v]", exp, min, max)
	}
}
