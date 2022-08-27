package longredis

import (
	"context"
	"testing"
	"time"
)

func BenchmarkLongredisGetSet(b *testing.B) {
	c, err := NewClient(Option{
		Addrs:    []string{"127.0.0.1:6379", "127.0.0.1:6380", "127.0.0.1:6381"},
		Prefix:   "longredis:presstest",
		PoolSize: 5,
	})
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	val := genID(10)
	for n := 0; n < b.N; n++ {
		key := genID(6)
		err = c.SetEx(ctx, key, val, time.Minute)
		if err != nil {
			panic(err)
		}

		_, err = c.Get(ctx, key)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkLongredisHGetHSet(b *testing.B) {
	c, err := NewClient(Option{
		Addrs:    []string{"127.0.0.1:6379", "127.0.0.1:6380", "127.0.0.1:6381"},
		Prefix:   "longredis:presstest",
		PoolSize: 5,
	})
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	key := genID(6)
	fields := []string{genID(6), genID(6), genID(6), genID(6), genID(6), genID(6), genID(6), genID(6), genID(6), genID(6), genID(6), genID(6), genID(6), genID(6), genID(6), genID(6), genID(6), genID(6)}
	for n := 0; n < b.N; n++ {
		i := n % len(fields)
		val := genID(10)
		err = c.HSet(ctx, key, fields[i], val)
		if err != nil {
			panic(err)
		}

		_, err = c.HGet(ctx, key, fields[i])
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkLongredisRpushLpop(b *testing.B) {
	c, err := NewClient(Option{
		Addrs:    []string{"127.0.0.1:6379", "127.0.0.1:6380", "127.0.0.1:6381"},
		Prefix:   "longredis:presstest",
		PoolSize: 5,
	})
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	key := genID(6)

	for i := 0; i < 100; i++ { // 预置数据
		val := genID(10)
		c.RPush(ctx, key, val)
	}

	for n := 0; n < b.N; n++ {
		val := genID(10)
		err = c.RPush(ctx, key, val)
		if err != nil {
			panic(err)
		}

		_, err = c.LPop(ctx, key)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkLongredisSaddSrem(b *testing.B) {
	c, err := NewClient(Option{
		Addrs:    []string{"127.0.0.1:6379", "127.0.0.1:6380", "127.0.0.1:6381"},
		Prefix:   "longredis:presstest",
		PoolSize: 5,
	})
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	key := genID(6)

	ss := make([]string, 100)
	for i := 0; i < 100; i++ { // 预置数据
		val := genID(6)
		ss[i] = val
	}
	c.SAdd(ctx, key, ss)

	for n := 0; n < b.N; n++ {
		val := genID(6)
		err = c.SAdd(ctx, key, val)
		if err != nil {
			panic(err)
		}

		_, err = c.SIsMember(ctx, key, val)
		if err != nil {
			panic(err)
		}

		err = c.SRem(ctx, key, val)
		if err != nil {
			panic(err)
		}
	}
}
