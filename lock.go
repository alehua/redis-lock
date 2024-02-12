package redis_lock

import (
	"context"
	_ "embed"
	"errors"
	"sync"
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
	//go:embed script/lua/refresh.lua
	luaRefresh string
)

type onceCloseChan struct {
	once sync.Once
	stop chan struct{}
}

func NewOnceCloseChan() *onceCloseChan {
	return &onceCloseChan{
		stop: make(chan struct{}, 1),
	}
}

func (o *onceCloseChan) Close() {
	o.once.Do(func() {
		close(o.stop)
	})
}

type Lock struct {
	key, value string
	expiration time.Duration
	client     redis.Cmdable
	unlock     *onceCloseChan
}

func newLock(client redis.Cmdable, key, value string, expiration time.Duration) *Lock {
	return &Lock{
		key:        key,
		expiration: expiration,
		client:     client,
		value:      value,
		unlock:     NewOnceCloseChan(),
	}
}

// AutoRefresh 自动续约(谨慎使用, 推荐使用手动)
func (l *Lock) AutoRefresh(interval time.Duration, timeout time.Duration) error {
	ch := make(chan struct{}, 1)
	defer close(ch)
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ch:
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := l.Refresh(ctx)
			cancel()
			// 超时这里，可以继续尝试
			if err == context.DeadlineExceeded {
				select {
				case ch <- struct{}{}:
				default:
				}
				continue
			}
			if err != nil {
				return err
			}
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := l.Refresh(ctx)
			cancel()
			if err == context.DeadlineExceeded {
				select {
				case ch <- struct{}{}:
				default:
				}
				continue
			}
			if err != nil {
				return err
			}
		case <-l.unlock.stop:
			return nil
		}
	}
}

// Refresh 手动刷新锁的过期时间
func (l *Lock) Refresh(ctx context.Context) error {
	res, err := l.client.Eval(ctx, luaRefresh,
		[]string{l.key}, l.value, l.expiration).Int64()
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

func (l *Lock) UnLock(ctx context.Context) error {
	// res, err := l.client.Del(ctx, l.key).Result()
	// 解锁的时候需要确定锁还是自己的锁
	//
	defer func() {
		l.unlock.stop <- struct{}{}
		l.unlock.Close()
	}()
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
