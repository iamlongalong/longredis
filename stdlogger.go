package longredis

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

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
