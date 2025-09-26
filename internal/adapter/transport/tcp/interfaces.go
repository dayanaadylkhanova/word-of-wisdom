package tcp

import "github.com/dayanaadylkhanova/word-of-wisdom/internal/entity"

//go:generate mockgen -source=interfaces.go -destination=./server_mock.go -package=tcp

type PoW interface {
	NewChallenge(difficulty int, ttlSeconds int64) (entity.Challenge, error)
	Verify(ch entity.Challenge, sol entity.Solution) error
}

type Quote interface {
	Random() string
}
