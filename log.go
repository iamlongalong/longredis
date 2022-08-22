package longredis

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
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

type stdLogger struct {
	level LogLevel
	log   *log.Logger
	pool  *sync.Pool
}

// NewStdLogger 基本格式为 xx=xxx
func NewStdLogger(level LogLevel, w io.Writer) Logger {
	if w == nil {
		w = os.Stderr
	}
	logger := log.New(w, "[longredis]", 0)

	return &stdLogger{
		log:   logger,
		level: level,
		pool: &sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}
}

func (l *stdLogger) Log(level LogLevel, msg string, keyvals ...interface{}) {
	if level.Less(l.level) {
		return
	}

	if (len(keyvals) % 2) != 0 {
		keyvals = append(keyvals, "KEYVALS UNPAIRED")
	}
	keyvals = append(keyvals, "_mod", "redis")

	buf := l.pool.Get().(*bytes.Buffer)
	buf.WriteString(level.String() + " " + msg)
	for i := 0; i < len(keyvals); i += 2 {
		_, _ = fmt.Fprintf(buf, " %s=%v", keyvals[i], keyvals[i+1])
	}
	_ = l.log.Output(4, buf.String()) //nolint:gomnd
	buf.Reset()
	l.pool.Put(buf)
	return
}

func NewLogrusLogger(l logrus.Logger) Logger {
	return &logrusLogger{logger: l}
}

type logrusLogger struct {
	logger logrus.Logger
}

func (l *logrusLogger) Log(level LogLevel, msg string, keyvals ...interface{}) {
	l.logger.WithFields(logrus.Fields{}).Log(l.parseLevel(level), msg)
	return
}

func (l *logrusLogger) parseLevel(le LogLevel) logrus.Level {
	level, err := logrus.ParseLevel(le.String())
	if err != nil {
		fmt.Printf("[longredis] parse level (%s) to logrus level fail : %s", le.String(), err)
		return logrus.ErrorLevel
	}
	return level
}
