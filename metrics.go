package longredis

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
)

// 兼容 promethues metrics
var (
	namespace = "default"
	subsystem = "longredis"
)

// metrics 包含内容
// 1. 请求成功数量
// 2. 请求失败数量
// 3. 超时数量
// 4. miss率
// 3. 命令的数量分布
// 4. 重连次数
// 5. 当前连接数
// 6. 请求时间分布

var (
	// 成功数量
	SuccessCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Name:        "success",
		Subsystem:   subsystem,
		Help:        "success counter of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"_clientid"})

	// 失败数量
	FailCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Name:        "fail",
		Subsystem:   subsystem,
		Help:        "fail counter of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"_clientid"})

	// 上层取消数量
	ContextCancelCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Name:        "ctxcancel",
		Subsystem:   subsystem,
		Help:        "context cancel counter of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"_clientid"})

	// 超时数量
	ContextTimeoutCounter = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "ctxtimeout",
		Subsystem:   subsystem,
		Help:        "context timeout counter of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"_clientid"})

	// miss 数
	MissCounter = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "miss",
		Subsystem:   subsystem,
		Help:        "miss counter of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"_clientid"})

	// 重连次数
	ReconnectCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Name:        "reconnect",
		Subsystem:   subsystem,
		Help:        "reconnect counter of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"_clientid"})

	// 当前连接数
	ConnsGuage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "conns",
		Subsystem:   subsystem,
		Help:        "conns counts of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"_clientid"})

	// 当前空闲连接数
	IdleConnsGuage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "idleconns",
		Subsystem:   subsystem,
		Help:        "idle conns counts of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"_clientid"})

	// 请求时间分布
	DurationHistgram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace:   namespace,
		Name:        "duration",
		Subsystem:   subsystem,
		Help:        "command duration of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
		Buckets:     []float64{1, 3, 5, 10, 20, 50, 100, 300, 500, 1000, 2000}, // ms
	})

	// 请求类型分布
	CommandCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Name:        "duration",
		Subsystem:   subsystem,
		Help:        "command type couter of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"command"})

	// 慢请求数量
	SlowCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Name:        "duration",
		Subsystem:   subsystem,
		Help:        "slow counter of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	})
)

type metricsHook struct{}

func (h *metricsHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *metricsHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	t := ctx.Value(startTimeKey).(time.Time)
	id := ctx.Value(clientidKey).(string)

	// duration
	DurationHistgram.Observe(float64(time.Now().Sub(t).Milliseconds()))

	// command type
	CommandCounter.WithLabelValues(cmd.Name()).Inc()

	// success or fail
	if cmd.Err() == nil {
		SuccessCounter.WithLabelValues(id).Inc()
	} else if errors.Is(cmd.Err(), redis.Nil) {
		SuccessCounter.WithLabelValues(id).Inc()
	} else {
		FailCounter.WithLabelValues(id).Inc()

		if errors.Is(cmd.Err(), context.Canceled) {
			ContextCancelCounter.WithLabelValues(id).Inc()
		}

		if errors.Is(cmd.Err(), context.DeadlineExceeded) {
			ContextTimeoutCounter.WithLabelValues(id).Inc()
		}
	}

	return nil
}

func (h *metricsHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *metricsHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	t := ctx.Value(startTimeKey).(time.Time)
	id := ctx.Value(clientidKey).(string)

	// duration
	DurationHistgram.Observe(float64(time.Now().Sub(t).Milliseconds()))

	// command type
	CommandCounter.WithLabelValues("pipeline").Inc()

	for _, cmd := range cmds {
		// command type
		CommandCounter.WithLabelValues(cmd.Name()).Inc()

		// success or fail
		if cmd.Err() == nil {
			SuccessCounter.WithLabelValues(id).Inc()
		} else if errors.Is(cmd.Err(), redis.Nil) {
			SuccessCounter.WithLabelValues(id).Inc()
			MissCounter.WithLabelValues(id).Inc()
		} else {

			FailCounter.WithLabelValues(id).Inc()

			if errors.Is(cmd.Err(), context.Canceled) {
				ContextCancelCounter.WithLabelValues(id).Inc()
			}

			if errors.Is(cmd.Err(), context.DeadlineExceeded) {
				ContextTimeoutCounter.WithLabelValues(id).Inc()
			}
		}
	}

	return nil
}

func ResetMetrics(namespace, subsystem string) {
	// 成功数量
	SuccessCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Name:        "success",
		Subsystem:   subsystem,
		Help:        "success counter of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"_clientid"})

	// 失败数量
	FailCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Name:        "fail",
		Subsystem:   subsystem,
		Help:        "fail counter of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"_clientid"})

	// 上层取消数量
	ContextCancelCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Name:        "ctxcancel",
		Subsystem:   subsystem,
		Help:        "context cancel counter of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"_clientid"})

	// 超时数量
	ContextTimeoutCounter = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "ctxtimeout",
		Subsystem:   subsystem,
		Help:        "context timeout counter of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"_clientid"})

	// miss 数
	MissCounter = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "miss",
		Subsystem:   subsystem,
		Help:        "miss counter of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"_clientid"})

	// 重连次数
	ReconnectCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Name:        "reconnect",
		Subsystem:   subsystem,
		Help:        "reconnect counter of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"_clientid"})

	// 当前连接数
	ConnsGuage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "conns",
		Subsystem:   subsystem,
		Help:        "conns counts of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"_clientid"})

	// 当前空闲连接数
	IdleConnsGuage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "idleconns",
		Subsystem:   subsystem,
		Help:        "idle conns counts of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"_clientid"})

	// 请求时间分布
	DurationHistgram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace:   namespace,
		Name:        "duration",
		Subsystem:   subsystem,
		Help:        "command duration of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
		Buckets:     []float64{1, 3, 5, 10, 20, 50, 100, 300, 500, 1000, 2000}, // ms
	})

	// 请求类型分布
	CommandCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Name:        "duration",
		Subsystem:   subsystem,
		Help:        "command type couter of longredis",
		ConstLabels: prometheus.Labels{"mod": "longredis"},
	}, []string{"command"})
}
