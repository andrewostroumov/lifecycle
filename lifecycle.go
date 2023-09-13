package lifecycle

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Hook struct {
	OnStart func(ctx context.Context) error
	OnStop  func(ctx context.Context) error
}

type Lifecycle struct {
	ctx             context.Context
	cancel          context.CancelFunc
	logger          *zap.Logger
	hooks           []Hook
	closed          chan struct{}
	invokeWait      sync.WaitGroup
	shutdownTimeout time.Duration
}

func New(opts ...Option) *Lifecycle {
	lcopts := defaultOptions

	for _, opt := range opts {
		opt(&lcopts)
	}

	lc := Lifecycle{
		closed: make(chan struct{}),
	}

	configLifecycle(&lc, lcopts)

	return &lc
}

func (lc *Lifecycle) Append(hook Hook) {
	lc.hooks = append(lc.hooks, hook)
}

func (lc *Lifecycle) Invoke(f func(ctx context.Context) error) {
	lc.hooks = append(lc.hooks, Hook{
		OnStart: f,
	})
}

func (lc *Lifecycle) Shutdown(f func(ctx context.Context) error) {
	lc.hooks = append(lc.hooks, Hook{
		OnStop: f,
	})
}

func (lc *Lifecycle) Run(ctx context.Context) {
	lc.logger.Info("Starting lifecycle...")

	lc.run(ctx)
	<-lc.ctx.Done()

	lc.logger.Info("Stopping lifecycle...")

	done := make(chan struct{})

	go func() {
		defer close(done)
		lc.shutdown(ctx)
		lc.invokeWait.Wait()
	}()

	select {
	case <-done:
		lc.logger.Info("Gracefully stopped lifecycle")
	case <-time.After(lc.shutdownTimeout):
		lc.logger.Info("Forcefully stopped lifecycle")
	}

	close(lc.closed)
}

func (lc *Lifecycle) Wait() {
	<-lc.closed
}

func (lc *Lifecycle) run(ctx context.Context) {
	const timeout = 100 * time.Millisecond

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	lc.logger.Info("Running OnStart hooks")

	for id, hook := range lc.hooks {
		if hook.OnStart == nil {
			continue
		}

		lc.logger.Debug(fmt.Sprintf("Running OnStart hook#%d", id))

		lc.invokeWait.Add(1)
		ch := lc.runOnStart(hook.OnStart, ctx)

		select {
		case <-timer.C:
		case err := <-ch:
			lc.logger.Error(err.Error(), zap.String("hook", "OnStart"))
			lc.cancel()
			return
		}

		go lc.notifyCancel(ch)

		timer.Reset(timeout)
	}
}

func (lc *Lifecycle) shutdown(ctx context.Context) {
	lc.logger.Info("Running OnStop hooks")

	for i := len(lc.hooks); i > 0; i-- {
		id := i - 1
		hook := lc.hooks[id]

		if hook.OnStop == nil {
			continue
		}

		lc.logger.Debug(fmt.Sprintf("Running OnStop hook#%d", id))

		if err := lc.runOnStop(hook.OnStop, ctx); err != nil {
			lc.logger.Error(err.Error(), zap.String("hook", "OnStop"))
		}
	}
}

func (lc *Lifecycle) runOnStart(f func(ctx context.Context) error, ctx context.Context) chan error {
	ch := make(chan error)

	go func() {
		defer lc.invokeWait.Done()
		defer close(ch)

		ch <- f(ctx)
	}()

	return ch
}

func (lc *Lifecycle) runOnStop(f func(ctx context.Context) error, ctx context.Context) error {
	return f(ctx)
}

func (lc *Lifecycle) notifyCancel(ch chan error) {
	for err := range ch {
		if err != nil {
			lc.logger.Error(err.Error(), zap.String("hook", "OnStart"))
			lc.cancel()
		}
	}
}

func configLifecycle(lc *Lifecycle, opts options) {
	ctx, cancel := context.WithCancel(opts.ctx)

	lc.shutdownTimeout = opts.shutdownTimeout
	lc.ctx = ctx
	lc.cancel = cancel
	lc.logger = opts.logger
}
