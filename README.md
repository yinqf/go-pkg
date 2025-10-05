# go-pkg

共享于多个服务之间的公共 Go 工具包。

## 模块结构

- `crud`：通用 CRUD 处理器与服务封装。
- `database`：数据库初始化与连接池配置。
- `distlock`：基于 Redis 的分布式锁。
- `logger`：基于 zap 的日志封装。
- `redis`：Redis 客户端初始化逻辑。
- `response`：HTTP JSON 响应帮助方法。

## 开发提示

1. 运行 `go mod tidy` 同步依赖。
2. 修改后执行 `go test ./...` 保障兼容性。
3. 在其他项目中通过 `go get github.com/yinqf/go-pkg` 引入。
