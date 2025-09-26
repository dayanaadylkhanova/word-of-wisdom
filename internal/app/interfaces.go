package app

import (
	"context"
)

//go:generate mockgen -source=interfaces.go -destination=./app_mock.go -package=app

type Runner interface {
	Run(ctx context.Context, difficulty int) error
}
