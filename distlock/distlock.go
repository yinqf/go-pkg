package distlock

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

var releaseScript = goredis.NewScript(`
    if redis.call("GET", KEYS[1]) == ARGV[1] then
        return redis.call("DEL", KEYS[1])
    else
        return 0
    end
`)

// Do 尝试通过 Redis 分布式锁执行任务。成功获取锁时返回 true。
func Do(ctx context.Context, client *goredis.Client, key string, ttl time.Duration, task func(context.Context)) (bool, error) {
	if client == nil {
		return false, errors.New("redis client is nil")
	}
	if ttl <= 0 {
		task(ctx)
		return true, nil
	}

	lockKey := "lock:" + key
	lockVal := uuid.NewString()

	ok, err := client.SetNX(ctx, lockKey, lockVal, ttl).Result()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	defer func() {
		_, _ = releaseScript.Run(ctx, client, []string{lockKey}, lockVal).Result()
	}()

	task(ctx)
	return true, nil
}
