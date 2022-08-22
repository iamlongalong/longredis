package example

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

var ctx = context.Background()

func ExampleLog() {
	c, err := longredis.NewClient(longredis.Option{
		Addrs:     []string{"127.0.0.1:6379"},
		RedisType: longredis.RedisTypeSingle,
	})
	if err != nil {
		panic(err)
	}

	key := time.Now().String()
	_, err = c.Get(ctx, key)
	if !errors.Is(err, redis.Nil) {
		panic(err)
	}

	err = c.Ping(ctx)
	if err != nil {
		panic(err)
	}

	// Output: xxx
}
