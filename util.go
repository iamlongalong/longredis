package longredis

import (
	"math/rand"
	"time"
)

var seed = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func init() {
	rand.Seed(time.Now().UnixNano())
}

func genID(length int) string {
	buf := make([]byte, length)
	for i := 0; i < length; i++ {
		buf[i] = seed[rand.Intn(len(seed))]
	}

	return string(buf)
}
