package longredis

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

func newErrorLogHook(logger Logger) redis.Hook {
	if logger == nil {
		logger = dfLogger
	}

	return &errorLogHook{
		logger: logger,
	}
}

type errorLogHook struct {
	logger Logger
}

func (h *errorLogHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *errorLogHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	if err := cmd.Err(); err != nil && err != redis.Nil {
		h.logger.Log(LevelError, fmt.Sprintf("redis %s failed: %s", cmd.Name(), err), "command", cmd.FullName())
	}

	return nil
}

func (h *errorLogHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *errorLogHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	errs := arrErr{}
	for _, cmd := range cmds {
		if cmd.Err() != nil {
			errs.Add(errors.Wrapf(cmd.Err(), "%s err", cmd.FullName()))
		}
	}

	if !errs.IsNil() {
		h.logger.Log(LevelError, errs.Error(), "command", "pipeline")
	}

	return nil
}

type arrErr struct {
	errs []error
}

func (e *arrErr) Add(err error) {
	e.errs = append(e.errs, err)
}

func (e *arrErr) IsNil() bool {
	return len(e.errs) == 0
}

func (e *arrErr) Error() string {
	str := ""
	for _, err := range e.errs {
		str += err.Error() + "; "
	}

	return str
}
