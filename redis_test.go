package longredis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	ctx = context.Background()

	singleAddr = []string{"127.0.0.1:6379"}

	addrs = []string{"127.0.0.1:6379"}
)

func TestLongredis(t *testing.T) {
	c, err := NewClient(Option{
		Addrs: []string{"127.0.0.1:6379"},
	})
	assert.Nil(t, err)

	err = c.Set(ctx, "longtest", "helloworld")
	assert.Nil(t, err)

	v, err := c.Get(ctx, "longtest")
	assert.Nil(t, err)
	assert.Equal(t, "helloworld", string(v))
}

func TestRequestTimeout(t *testing.T) {
	_, err := NewClient(Option{
		Addrs:          []string{"127.0.0.1:6379"},
		RequestTimeout: time.Nanosecond * 100,
	})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestPrefix(t *testing.T) {
	c, err := NewClient(Option{
		Addrs: []string{"127.0.0.1:6379"},
	})
	assert.Nil(t, err)

	longtestClient := c.SubPrefix("longtest")

	err = longtestClient.Set(ctx, "hello", "world")
	assert.Nil(t, err)

	v, err := longtestClient.Get(ctx, "hello")
	assert.Nil(t, err)
	assert.Equal(t, "world", string(v))

	v, err = c.Get(ctx, "hello")
	assert.NotNil(t, err)
	assert.Nil(t, v)
}

func TestReidsSingle(t *testing.T) {
	c, err := NewClient(Option{
		Addrs:     []string{"127.0.0.1:6379"}, // local redis
		RedisType: RedisTypeSingle,
	})
	assert.Nil(t, err)

	err = c.Ping(ctx)
	checkErr(err)
}

func TestGetString(t *testing.T) {
	c, err := NewClient(Option{
		Addrs:     []string{"127.0.0.1:6379"},
		RedisType: RedisTypeSingle,
	})
	assert.Nil(t, err)

	err = c.Set(ctx, "test_get_string", "hello world")
	assert.Nil(t, err)

	v, err := c.GetString(ctx, "test_get_string")
	assert.Nil(t, err)
	fmt.Printf("get string : %s\n", v)
	assert.Equal(t, "hello world", v)

	b, err := c.Get(ctx, "test_get_string")
	assert.Nil(t, err)
	fmt.Printf("get byte : %s\n", string(b))
	assert.Equal(t, "hello world", string(b))

}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
