package longredis

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

var startTimeKey = "_longredis_start_time"
var clientidKey = "_longredis_client_id"

type slowlog struct {
	slowTime time.Duration
	logLevel LogLevel
}

func (h *slowlog) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return context.WithValue(ctx, startTimeKey, time.Now()), nil
}

func (h *slowlog) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	it := ctx.Value(startTimeKey)
	t := it.(time.Time)

	d := time.Now().Sub(t)
	if d > h.slowTime { // 超时
		SlowCounter.Inc()
		Log(h.logLevel, "longredis slowlog", "duration", d.String(), "baseline", h.slowTime.String(), "command", cmd.FullName())
	}

	return nil
}

func (h *slowlog) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	return context.WithValue(ctx, startTimeKey, time.Now()), nil
}

func (h *slowlog) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	it := ctx.Value(startTimeKey)
	t := it.(time.Time)

	d := time.Now().Sub(t)
	if d > h.slowTime { // 超时
		SlowCounter.Inc()

		cmdStr := ""
		for _, cmd := range cmds {
			cmdStr += cmd.FullName() + "; "
		}

		Log(h.logLevel, "longredis slowlog", "duration", d.String(), "baseline", h.slowTime.String(), "cmdcounts", len(cmds), "command", cmdStr)
	}

	return nil
}

type injectorHook struct {
	clientid string
}

func (h *injectorHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	ctx = context.WithValue(ctx, startTimeKey, time.Now())
	ctx = context.WithValue(ctx, clientidKey, h.clientid)
	return ctx, nil
}

func (h *injectorHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	return nil
}

func (h *injectorHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	ctx = context.WithValue(ctx, startTimeKey, time.Now())
	ctx = context.WithValue(ctx, clientidKey, h.clientid)
	return ctx, nil
}

func (h *injectorHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	return nil
}
