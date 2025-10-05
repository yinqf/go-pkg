package database

import (
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var (
	dbInstance *gorm.DB
	mu         sync.Mutex
)

const (
	defaultMaxOpenConns    = 120
	defaultMaxIdleConns    = 60
	defaultConnMaxLifetime = 2 * time.Hour
	defaultConnMaxIdleTime = 30 * time.Minute
)

// NewDB 返回基于环境配置初始化的 GORM 单例实例。
func NewDB() (*gorm.DB, error) {
	mu.Lock()
	defer mu.Unlock()

	if dbInstance != nil {
		return dbInstance, nil
	}

	dsn, err := resolveDSN()
	if err != nil {
		return nil, err
	}

	gormDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	handle, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("database handle: %w", err)
	}
	configureConnectionPool(handle)
	dbInstance = gormDB
	return dbInstance, nil
}

func resolveDSN() (string, error) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		return "", fmt.Errorf("env MYSQL_DSN is required")
	}
	return dsn, nil
}

func configureConnectionPool(handle *sql.DB) {
	handle.SetMaxOpenConns(defaultMaxOpenConns)
	handle.SetMaxIdleConns(defaultMaxIdleConns)
	handle.SetConnMaxLifetime(defaultConnMaxLifetime)
	handle.SetConnMaxIdleTime(defaultConnMaxIdleTime)
}
