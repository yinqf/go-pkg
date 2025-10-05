# go-pkg

共享于多个服务之间的公共 Go 工具包。

## 模块结构

- `auth`：JWT 令牌的签发、校验与上下文辅助函数。
- `crud`：通用 CRUD 处理器与服务封装。
- `database`：数据库初始化与连接池配置。
- `distlock`：基于 Redis 的分布式锁。
- `logger`：基于 zap 的日志封装与文件滚动策略。
- `redis`：Redis 客户端初始化逻辑。
- `response`：HTTP JSON 响应帮助方法。

后续新增模块会沿用同一风格，方便在多个服务之间复用。

## 开发提示

1. 运行 `go mod tidy` 同步依赖。
2. 修改后执行 `go test ./...` 保障兼容性。
3. 在其他项目中通过 `go get github.com/yinqf/go-pkg` 引入。

## 环境变量

- `MYSQL_DSN`：`database` 包初始化 GORM 所需的数据库连接串，例如 `user:pass@tcp(host:3306)/dbname`。
- `REDIS_CONN_STRING`：`redis` 包创建客户端时使用的 Redis URL，例如 `redis://:password@127.0.0.1:6379/0`。
- `JWT_SECRET`：`auth` 包签发/校验 JWT 的对称密钥，必须在运行环境通过环境变量提供，并避免提交到版本库。
