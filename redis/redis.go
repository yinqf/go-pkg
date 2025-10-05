package redis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/yinqf/go-pkg/logger"
	"go.uber.org/zap"
)

var (
	errEmptyConnString = errors.New("redis connection string is empty")
)

// NewClient 初始化 Redis 客户端并验证连通性，由依赖注入容器管理其生命周期。
func NewClient() (*goredis.Client, error) {
	redisConnString := os.Getenv("REDIS_CONN_STRING")
	if redisConnString == "" {
		logger.Error("Redis 连接字符串为空")
		return nil, errEmptyConnString
	}

	opt, err := goredis.ParseURL(redisConnString)
	if err != nil {
		logger.Error("解析 Redis 连接字符串失败", zap.Error(err))
		return nil, fmt.Errorf("parse redis connection string: %w", err)
	}

	client := goredis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		logger.Error("Redis 连接失败", zap.Error(err))
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	logger.Info("Redis 客户端已初始化", zap.String("addr", opt.Addr))
	return client, nil
}
