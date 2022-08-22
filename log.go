package longredis

import (
	"fmt"
)

var dfLogger Logger = &NilLogger{}

func SetDefaultLogger(l Logger) {
	dfLogger = l
}

type Logger interface {
	Log(level LogLevel, msg string, keyvals ...interface{})
}

type LogLevel struct {
	val int
	str string
}

func (l LogLevel) Less(il LogLevel) bool {
	return l.val < il.val
}
func (l LogLevel) Val() int {
	return l.val
}
func (l LogLevel) String() string {
	return l.str
}

var (
	LevelDebug = LogLevel{val: 1, str: "debug"}
	LevelInfo  = LogLevel{val: 2, str: "info"}
	LevelWarn  = LogLevel{val: 3, str: "warn"}
	LevelError = LogLevel{val: 4, str: "error"}
)

func ParseLevel(level string) LogLevel {
	switch level {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError

	default:
		Log(LevelWarn, fmt.Sprintf("parse level failed of [%s], using error as default level", level))
		return LevelError
	}
}

type NilLogger struct{}

func (l *NilLogger) Log(level LogLevel, msg string, keyvals ...interface{}) {}

func Log(level LogLevel, msg string, keyvals ...interface{}) {
	dfLogger.Log(level, msg, keyvals...)
}

type kvPairs struct {
	vals []interface{}
}

func (p *kvPairs) Add(k string, v interface{}) {
	p.vals = append(p.vals, k, v)
}

func (p *kvPairs) Expand() []interface{} {
	return p.vals
}
