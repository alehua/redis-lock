package redis_lock

import (
	"context"
	_ "embed"
	"errors"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"time"
)

var (
	//go:embed script/lua/reentrant_lock.lua
	reentrantLuaLock string
)

// ReentrantLock 可重入lock

type ReentrantLock struct {
	client     redis.Cmdable
	key, value string
	expiration time.Duration
}

// 思路: 通过Hash实现
// key: 锁名称
// value: 锁持有者UUID、重入次数。 重入的时候次数+1, 解锁次数-1

func (r *ReentrantLock) Lock(ctx context.Context, key string,
	expiration time.Duration, timeout time.Duration) (*ReentrantLock, error) {
	val := uuid.New().String()
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		lctx, cancel := context.WithTimeout(ctx, timeout)
		res, err := r.client.Eval(lctx, reentrantLuaLock, []string{key},
			val, expiration).Int()
		cancel()

		if err != nil && errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if res > 0 {
			// TODO
			return nil, nil
		}
	}
}
