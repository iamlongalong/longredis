package longredis

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
)

func newTracingHook(opt TracingOption) redis.Hook {
	if opt.Tracer == nil {
		opt.Tracer = opentracing.GlobalTracer()
	}

	return &tracingHook{
		tracer: opt.Tracer,
	}
}

type tracingHook struct {
	tracer opentracing.Tracer
}

func (h *tracingHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	_, ctx = opentracing.StartSpanFromContextWithTracer(ctx, h.tracer, fmt.Sprintf("REDIS %s", cmd.FullName()))

	return ctx, nil
}

func (h *tracingHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	sp := opentracing.SpanFromContext(ctx)
	if sp == nil {
		return nil
	}

	if err := cmd.Err(); err != nil && err != redis.Nil {
		ext.Error.Set(sp, true)
		sp.LogFields(log.String("event", "error"), log.String("message", err.Error()))
	}

	sp.Finish()

	return nil
}

func (h *tracingHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	_, ctx = opentracing.StartSpanFromContextWithTracer(ctx, h.tracer, "REDIS pipline")

	return ctx, nil
}

func (h *tracingHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	sp := opentracing.SpanFromContext(ctx)
	if sp == nil {
		return nil
	}

	sp.Finish()

	return nil
}
