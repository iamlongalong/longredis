package longredis

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/opentracing/opentracing-go"
)

type redisInsType string

const (
	RedisTypeSingle    redisInsType = "single"
	RedisTypeSingleton redisInsType = "singleton"
	RedisTypeSentinel  redisInsType = "sentinel"
	RedisTypeCluster   redisInsType = "cluster"
)

type Option struct {
	// redis server 地址，会自动探测为单节点/哨兵/集群
	Addrs []string `yaml:"addrs" json:"addrs"`

	// redis 类型，default RedisTypeCluster (单节点)
	RedisType redisInsType `yaml:"redis_type" json:"redis_type"`

	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`

	Prefix string `yaml:"prefix" json:"prefix"`
	// default 7 * 24h
	DefaultExpire time.Duration `yaml:"default_expire" json:"default_expire"`

	// 请求超时时间， default 3s
	RequestTimeout time.Duration `yaml:"request_timeout" json:"request_timeout"`

	// 自定义 Dialer
	Dialer    func(ctx context.Context, network, addr string) (net.Conn, error) `yaml:"-" json:"-"`
	OnConnect func(ctx context.Context, cn *redis.Conn) error                   `yaml:"-" json:"-"`

	// 建连超时时间，default 5s
	DialTimeout time.Duration `yaml:"dial_timeout" json:"dial_timeout"`
	// 读 超时时间，default 5s
	ReadTimeout time.Duration `yaml:"read_timeout" json:"read_timeout"`
	// 写 超时时间，default 5s
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`

	// 重试次数，default 0
	MaxRetries int `yaml:"max_retries" json:"max_retries"`
	// 最小重试间隔时间，default 0
	MinRetryBackoff time.Duration `yaml:"min_retry_backoff" json:"min_retry_backoff"`
	// 最大重试间隔时间，default 0
	MaxRetryBackoff time.Duration `yaml:"max_retry_backoff" json:"max_retry_backoff"`

	// 连接池大小，default 5
	PoolSize int `yaml:"pool_size" json:"pool_size"`
	// 最小空闲连接数，default 2
	MinIdleConns int `yaml:"min_idle_conns" json:"min_idle_conns"`
	// 单个连接最大生命周期，default 10m
	MaxConnAge time.Duration `yaml:"max_conn_age" json:"max_conn_age"`
	// 连接池超时时间, default 5s
	PoolTimeout time.Duration `yaml:"pool_timeout" json:"pool_timeout"`
	// 最大空闲连接超时时间，default 10m
	IdleTimeout time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	// 最大空闲连接检查周期，default 1m
	IdleCheckFrequency time.Duration `yaml:"idle_check_frequency" json:"idle_check_frequency"`

	TLSConfig *tls.Config `yaml:"tls_config" json:"tls_config"`

	// 针对 cluster 的配置
	ClusterOpt ClusterOption `yaml:"cluster_opt" json:"cluster_opt"`
	// 针对 主从 的配置
	SingleOpt SingleOption `yaml:"single_opt" json:"single_opt"`
	// 针对 哨兵 的配置
	SentinelOpt SentinelOption `yaml:"sentinel_opt" json:"sentinel_opt"`

	// 重连配置
	Supervisor SupervisorOption `yaml:"supervisor" json:"supervisor"`

	// opentracing 设置
	Tracing TracingOption `yaml:"tracing" json:"tracing"`

	// metrics 设置
	Metrics MetricsOption `yaml:"metrics" json:"metrics"`

	// error log
	DisableErrorLog bool `yaml:"disable_error_log" json:"disable_error_log"`
	errorLogHook

	// slowlog 设置
	SlowLog SlowLogOption `yaml:"slowlog" json:"slowlog"`

	// log 设置
	Log Logger `yaml:"log" json:"log"`
}

type TracingOption struct {
	Disable bool `yaml:"disable" json:"disable"`

	Tracer opentracing.Tracer
}

type SlowLogOption struct {
	Disable bool `yaml:"disable" json:"disable"`

	// 慢日志红线，default 300ms
	SlowTime time.Duration `yaml:"slow_time" json:"slow_time"`

	// 当同时存在 LogLevelStr 和 LogLevel 时，以 LogLevelStr 为准
	LogLevelStr string `yaml:"log_level" json:"log_level"`

	LogLevel LogLevel
}

type MetricsOption struct {
	Disable bool `yaml:"disable" json:"disable"`

	Namespace string `yaml:"namespace" json:"namespace"`
	Subsystem string `yaml:"subsystem" json:"subsystem"`
}

type ClusterOption struct {
	MaxRedirects   int  `yaml:"max_redirects" json:"max_redirects"`
	ReadOnly       bool `yaml:"read_only" json:"read_only"`
	RouteByLatency bool `yaml:"route_by_latency" json:"route_by_latency"`
	RouteRandomly  bool `yaml:"route_randomly" json:"route_randomly"`
}

type SingleOption struct {
	DB int `yaml:"db" json:"db"`
}

type SentinelOption struct {
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
}

type SupervisorOption struct {
	PingTimeout     time.Duration `yaml:"ping_timeout" json:"ping_timeout"`           // default as writetimeout
	CheckInterval   time.Duration `yaml:"check_interval" json:"check_interval"`       // default 10s
	CheckMaxRetries int           `yaml:"check_max_retries" json:"check_max_retries"` // default 5
}

func convertRedisOption(opt Option) *redis.UniversalOptions {
	return &redis.UniversalOptions{
		Addrs:    opt.Addrs,
		Username: opt.Username,
		Password: opt.Password,

		DB: opt.SingleOpt.DB,

		Dialer:    opt.Dialer,
		OnConnect: opt.OnConnect,

		SentinelUsername: opt.SentinelOpt.Username,
		SentinelPassword: opt.SentinelOpt.Password,

		MaxRetries:      opt.MaxRetries,
		MinRetryBackoff: opt.MinRetryBackoff,
		MaxRetryBackoff: opt.MaxRetryBackoff,

		DialTimeout:  opt.DialTimeout,
		ReadTimeout:  opt.ReadTimeout,
		WriteTimeout: opt.WriteTimeout,

		PoolSize:           opt.PoolSize,
		MinIdleConns:       opt.MinIdleConns,
		MaxConnAge:         opt.MaxConnAge,
		PoolTimeout:        opt.PoolTimeout,
		IdleTimeout:        opt.IdleTimeout,
		IdleCheckFrequency: opt.IdleCheckFrequency,

		TLSConfig: opt.TLSConfig,

		MaxRedirects:   opt.ClusterOpt.MaxRedirects,
		ReadOnly:       opt.ClusterOpt.ReadOnly,
		RouteByLatency: opt.ClusterOpt.RouteByLatency,
		RouteRandomly:  opt.ClusterOpt.RouteRandomly,
	}
}

func ParseRedisType(t string) redisInsType {
	switch t {
	case "single", "singleton":
		return RedisTypeSingleton
	case "cluster":
		return RedisTypeCluster
	case "sentinel":
		return RedisTypeSentinel
	default:
		Log(LevelError, fmt.Sprintf("parse redis type fail of : %s, using default single", t))
		return RedisTypeCluster
	}
}
