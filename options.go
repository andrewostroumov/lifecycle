package lifecycle

import (
	"context"
	"os"
	"os/signal"
	"time"

	"go.uber.org/zap"
)

type options struct {
	ctx             context.Context
	logger          *zap.Logger
	startTimeout    time.Duration
	shutdownTimeout time.Duration
}

var defaultOptions = options{
	ctx:             context.Background(),
	logger:          zap.NewNop(),
	shutdownTimeout: 30 * time.Second,
}

type Option func(opts *options)

func WithLogger(logger *zap.Logger) Option {
	return func(opts *options) {
		opts.logger = logger
	}
}

func WithShutdownTimeout(timeout time.Duration) Option {
	return func(opts *options) {
		opts.shutdownTimeout = timeout
	}
}

func WithContext(ctx context.Context) Option {
	return func(opts *options) {
		opts.ctx = ctx
	}
}

func WithSignalContext(ctx context.Context, sig ...os.Signal) Option {
	return func(opts *options) {
		ctx, _ = signal.NotifyContext(ctx, sig...)
		opts.ctx = ctx
	}
}
