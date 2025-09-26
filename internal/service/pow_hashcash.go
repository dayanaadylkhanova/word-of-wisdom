package service

import (
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"math/bits"
	"strconv"
	"strings"
	"time"

	"github.com/dayanaadylkhanova/word-of-wisdom/internal/entity"
)

type Hashcash struct{}

func NewHashcash() *Hashcash { return &Hashcash{} }

func (h *Hashcash) NewChallenge(difficulty int, ttlSeconds int64) (entity.Challenge, error) {
	salt := make([]byte, 16)
	_, _ = crand.Read(salt)
	return entity.Challenge{
		Version:    1,
		Algo:       "sha256-leading-zero-bits",
		Difficulty: difficulty,
		SaltB64:    base64.StdEncoding.EncodeToString(salt),
		Expires:    time.Now().Add(time.Duration(ttlSeconds) * time.Second).Unix(),
	}, nil
}

func leadingZeroBits(b []byte) int {
	total := 0
	for _, by := range b {
		if by == 0 {
			total += 8
			continue
		}
		total += bits.LeadingZeros8(by)
		break
	}
	return total
}

func powMessage(salt []byte, expires int64, nonceHex string) []byte {
	payload := make([]byte, 0, len(salt)+1+20+1+len(nonceHex))
	payload = append(payload, salt...)
	payload = append(payload, ':')
	payload = append(payload, []byte(strconv.FormatInt(expires, 10))...)
	payload = append(payload, ':')
	payload = append(payload, []byte(nonceHex)...)
	return payload
}

func (h *Hashcash) Verify(ch entity.Challenge, sol entity.Solution) error {
	if ch.Algo != "sha256-leading-zero-bits" || ch.Version != 1 {
		return errors.New("unsupported challenge")
	}
	if time.Now().Unix() > ch.Expires {
		return errors.New("challenge expired")
	}
	salt, err := base64.StdEncoding.DecodeString(ch.SaltB64)
	if err != nil {
		return fmt.Errorf("bad salt: %w", err)
	}
	sum := sha256.Sum256(powMessage(salt, ch.Expires, strings.ToLower(sol.Nonce)))
	if leadingZeroBits(sum[:]) < ch.Difficulty {
		return errors.New("pow invalid")
	}
	return nil
}
