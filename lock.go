package redis_lock

import (
	"context"
	_ "embed"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrFailedToPreemptLock = errors.New("rlock: 抢锁失败")
	ErrLockNotHold         = errors.New("rlock: 未持有锁")
)

type Client struct {
	client redis.Cmdable
}

func NewClient(client redis.Cmdable) *Client {
	return &Client{
		client: client,
	}
}

func (c *Client) TryLock(ctx context.Context, key string,
	expiration time.Duration) (*Lock, error) {
	val := uuid.New().String()
	res, err := c.client.SetNX(ctx, key, val, expiration).Result()
	if err != nil {
		return nil, err
	}
	if !res {
		return nil, ErrFailedToPreemptLock
	}
	return newLock(c.client, key, val, expiration), nil
}

var (
	//go:embed script/lua/unlock.lua
	luaUnLock string
)

type Lock struct {
	key, value string
	expiration time.Duration
	client     redis.Cmdable
}

func newLock(client redis.Cmdable, key, value string, expiration time.Duration) *Lock {
	return &Lock{
		key:        key,
		expiration: expiration,
		client:     client,
		value:      value,
	}
}

func (l *Lock) UnLock(ctx context.Context) error {
	// res, err := l.client.Del(ctx, l.key).Result()
	// 解锁的时候需要确定锁还是自己的锁
	//
	res, err := l.client.Eval(ctx, luaUnLock, []string{l.key}, l.value).Int64()
	if err == redis.Nil {
		return ErrLockNotHold
	}
	if err != nil {
		return err
	}
	if res != 1 {
		return ErrLockNotHold
	}
	return nil
}
