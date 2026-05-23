# Video Feed

一个面向短视频社区场景的 Go 后端项目，覆盖账号鉴权、视频发布、互动关系、Feed 流、热榜、通知、私信、分片上传和异步 Worker 等核心链路。

## 功能概览

- 账号体系：注册、登录、刷新 Token、退出登录、资料更新、头像上传、用户信息查询。
- 鉴权与安全：JWT 鉴权、Redis Token 缓存与撤销、公共 Feed 软鉴权、Redis 分布式限流。
- 视频模块：视频发布、删除、详情查询、作者视频列表、视频和封面上传、标签关联。
- 分片上传：上传会话初始化、分片上传、进度查询、完成合并、重复分片幂等处理。
- Feed 流：最新流、点赞榜、热榜、标签 Feed、关注流，按场景使用不同游标策略。
- 热榜缓存：Redis ZSET 存储热度排序，本地缓存 + Redis MGET + MySQL 回源补齐视频实体。
- 社交互动：点赞、取消点赞、评论、删除评论、关注、取消关注、粉丝/关注列表和计数。
- 异步处理：RabbitMQ 承载点赞、评论、关注、热度、通知、时间线等事件，Worker 独立消费。
- 一致性补偿：视频发布后通过 Outbox 投递 Feed 时间线事件，降低发布成功但时间线遗漏的风险。
- 通知与消息：SSE 实时通知、通知列表/已读/未读数、站内私信发送与会话查询。
- 可观测性：Zap 结构化日志（trace_id 自动注入）、OpenTelemetry 全链路追踪、Prometheus 指标暴露、Swagger 接口文档、API 与 Worker 独立 pprof 开关。

## 技术栈

- 语言与框架：Go, Gin
- 数据访问：GORM, MySQL
- 缓存与状态：Redis
- 消息队列：RabbitMQ
- 鉴权：JWT
- 可观测性：Zap, OpenTelemetry, Prometheus, Swagger (swaggo)
- 测试依赖：miniredis, SQLite test driver
- 部署构建：Docker multi-stage build

## 项目结构

```text
.
|-- cmd
|   |-- server      # API 服务入口
|   `-- worker      # MQ Worker 入口
|-- configs         # 本地、容器环境配置
|-- internal
|   |-- account     # 账号、资料、Token 相关业务
|   |-- auth        # JWT 基础能力
|   |-- config      # 配置加载
|   |-- db          # 数据库连接与迁移
|   |-- feed        # Feed 查询、游标、热榜读取
|   |-- http        # Gin 路由装配
|   |-- message     # 私信
|   |-- middleware  # JWT、Redis、RabbitMQ、限流
|   |-- observability
|   |-- social      # 关注关系
|   |-- video       # 视频、点赞、评论、标签、分片上传
|   `-- worker      # 异步消费者、通知、时间线
`-- Dockerfile
```

## 核心设计

### Feed 游标

不同 Feed 场景使用不同翻页策略：

- 最新流按发布时间和 ID 做时间游标，适合连续滚动读取最新内容。
- 点赞榜按 `likes_count + id` 复合游标翻页，避免点赞数相同时排序不稳定。
- 热榜通过 `as_of + offset` 固定榜单快照，减少翻页过程中榜单变化导致的重复或漏读。
- 关注流先按关注关系过滤作者，再按 Feed 游标读取对应视频。

### 热榜与实体缓存

热榜读取分成两层：

- Redis ZSET 保存视频 ID 与热度分数，用于排序和分页。
- 视频实体通过本地缓存、Redis 批量读取和 MySQL 回源补齐。

当实体缓存未命中时，服务端使用 singleflight 合并同一视频的并发回源，减少热点 miss 时的重复数据库查询。

### 异步事件与降级

点赞、评论、关注等写操作会影响计数、热度和通知。项目将可最终一致的副作用拆为 RabbitMQ 事件，由 Worker 异步消费；主业务状态仍在 Service 内完成同步写入。消息投递不可用时，核心链路保留同步降级路径，避免 MQ 故障直接阻断用户操作。

### Outbox 时间线

视频发布链路在本地事务内写入视频记录和 Outbox 消息。Outbox 轮询器负责投递时间线事件，Timeline Worker 将视频写入 Redis 全局时间线，用于 Feed 冷热数据拼接。

## 快速开始

### 环境依赖

- Go 1.26.1
- MySQL
- Redis
- RabbitMQ

本地默认配置见 `configs/config.yaml`。如果你的 MySQL、Redis 或 RabbitMQ 端口不同，可以复制配置文件后通过 `CONFIG_PATH` 指定。

### 初始化数据库

创建数据库即可，表结构由 GORM AutoMigrate 在服务启动时自动迁移：

```sql
CREATE DATABASE IF NOT EXISTS video_feed DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 启动 API

```powershell
$env:CONFIG_PATH = "configs/config.yaml"
go run ./cmd/server
```

启动后健康检查：

```powershell
Invoke-RestMethod http://127.0.0.1:8080/health
```

### 启动 Worker

```powershell
$env:CONFIG_PATH = "configs/config.yaml"
go run ./cmd/worker
```

Worker 负责消费点赞、评论、关注、热度、通知和时间线等异步事件。

### Docker Compose

也可以直接启动 MySQL、Redis、RabbitMQ、API 和 Worker：

```powershell
docker compose up -d --build
```

本地连接端口：

- API: `http://127.0.0.1:8080`
- MySQL: `127.0.0.1:13306`
- Redis: `127.0.0.1:6379`
- RabbitMQ: `127.0.0.1:5672`
- RabbitMQ Management: `http://127.0.0.1:15672`

## Docker 构建

构建 API 镜像：

```powershell
docker build --target api -t video-feed-api .
```

构建 Worker 镜像：

```powershell
docker build --target worker -t video-feed-worker .
```

容器内默认读取 `configs/config.docker.yaml`，其中 MySQL、Redis、RabbitMQ 主机名分别为 `mysql`、`redis`、`rabbitmq`。

## 测试

```powershell
go test ./...
```

项目测试覆盖配置加载、JWT、路由、限流、Feed 游标、热榜缓存、视频详情缓存、分片上传、点赞评论、关注、私信、通知、Worker 消费和 Outbox 时间线等链路。

## 压测与量化实验

压测主入口使用 JMeter，Go 工具负责造数据和切换对比状态，不依赖前端页面。

```powershell
go run ./cmd/benchseed -config configs/config.yaml -users 50 -videos 1000
```

`benchseed` 会生成账号、视频、关注、点赞、评论、标签和 Redis 热榜/时间线数据，并输出：

- `bench/results/seed-*.json`：压测 manifest，记录密码、热榜 `as_of`、CSV 路径等。
- `bench/results/users-*.csv`：JMeter 登录用户数据。
- `bench/results/videos-*.csv`：JMeter 视频 ID 数据。

读取最近一次 seed 结果：

```powershell
$seed = Get-ChildItem bench/results/seed-*.json | Sort-Object LastWriteTime -Descending | Select-Object -First 1
$m = Get-Content $seed.FullName | ConvertFrom-Json
```

推荐直接运行完整 JMeter 对比脚本。`FeedThreads` 会对最新 Feed 游标分页跑多档并发，`SkipComment` 可跳过评论写入场景，避免把账号限流混入读路径实验：

```powershell
.\bench\jmeter\run-comparison.ps1 `
  -Manifest $seed.FullName `
  -Threads 5 `
  -FeedThreads 5,10,20 `
  -Duration 30 `
  -RampUp 5 `
  -Limit 20 `
  -SkipComment
```

脚本会依次生成热榜 DB fallback、热榜 Redis 快照、详情冷读、详情热缓存、最新 Feed 多并发档位和可选评论写入的 `.jtl` 与 HTML 报告。JMeter 场景对最新 Feed 会提取响应中的 `next_time` 并写回下一轮请求，模拟真实游标分页。

提取两次 JTL 的对比指标：

```powershell
.\bench\jmeter\compare-jtl.ps1 `
  -Baseline bench/results/hot-db.jtl `
  -Candidate bench/results/hot-redis.jtl `
  -BaselineName hot-db `
  -CandidateName hot-redis `
  -Out bench/results/hot-comparison.md
```

汇总多份 JTL：

```powershell
.\bench\jmeter\summarize-jtl.ps1 `
  -Jtl bench/results/hot-db.jtl,bench/results/hot-redis.jtl,bench/results/latest-t5.jtl `
  -Name hot-db,hot-redis,latest-t5 `
  -Out bench/results/summary.md
```

热榜对比使用同一个 JMeter 场景，通过 Redis 状态切换得到前后对比：

```powershell
go run ./cmd/benchstate -config configs/config.yaml -manifest $seed.FullName -mode db
jmeter -n -t bench/jmeter/video-feed-benchmark.jmx `
  "-Jscenario=hot" "-Jbase_host=127.0.0.1" "-Jbase_port=8080" `
  "-Jusers_csv=$($m.users_csv)" "-Jvideos_csv=$($m.videos_csv)" "-Jhot_as_of=$($m.hot_as_of)" "-Jpassword=$($m.password)" `
  "-Jthreads=20" "-Jduration=60" `
  -l bench/results/hot-db.jtl -e -o bench/results/hot-db-html

go run ./cmd/benchstate -config configs/config.yaml -manifest $seed.FullName -mode hot
jmeter -n -t bench/jmeter/video-feed-benchmark.jmx `
  "-Jscenario=hot" "-Jbase_host=127.0.0.1" "-Jbase_port=8080" `
  "-Jusers_csv=$($m.users_csv)" "-Jvideos_csv=$($m.videos_csv)" "-Jhot_as_of=$($m.hot_as_of)" "-Jpassword=$($m.password)" `
  "-Jthreads=20" "-Jduration=60" `
  -l bench/results/hot-redis.jtl -e -o bench/results/hot-redis-html
```

详情缓存对比：

```powershell
go run ./cmd/benchstate -config configs/config.yaml -manifest $seed.FullName -mode detail-cold
jmeter -n -t bench/jmeter/video-feed-benchmark.jmx `
  "-Jscenario=detail" "-Jbase_host=127.0.0.1" "-Jbase_port=8080" `
  "-Jusers_csv=$($m.users_csv)" "-Jvideos_csv=$($m.videos_csv)" `
  "-Jthreads=20" "-Jduration=60" `
  -l bench/results/detail-cold.jtl -e -o bench/results/detail-cold-html

jmeter -n -t bench/jmeter/video-feed-benchmark.jmx `
  "-Jscenario=detail" "-Jbase_host=127.0.0.1" "-Jbase_port=8080" `
  "-Jusers_csv=$($m.users_csv)" "-Jvideos_csv=$($m.videos_csv)" `
  "-Jthreads=20" "-Jduration=60" `
  -l bench/results/detail-hot.jtl -e -o bench/results/detail-hot-html
```

其他场景可以通过 `-Jscenario=latest` 或 `-Jscenario=comment` 单独运行。JMeter HTML 报告用于正式截图和复盘，`bench/results/` 是本地生成结果目录，不会进入 Git。详情冷读实验要确保样本数小于视频 CSV 行数，否则 CSV 循环后会混入缓存命中。

项目也保留了轻量 Go runner，适合本地快速 smoke 和自动化对比：

```powershell
go run ./cmd/benchrun -config configs/config.yaml -manifest $seed.FullName -requests 300 -concurrency 20
```

对比实验重点：

- `popularity-db`：清理热榜 Redis key 后压测 `/feed/listByPopularity` 的 DB fallback 路径。
- `popularity-hot`：写入 Redis 热榜快照后压测同一路由的热榜路径。
- `detail-cold` / `detail-hot`：对比视频详情冷读与缓存命中后的延迟。
- `latest`：压测最新 Feed 游标分页。
- `comment`：压测登录态评论写入链路。

## API 概览

主要路由按业务域划分：

- `GET /health`
- `GET /metrics` — Prometheus 指标暴露端点
- `GET /swagger/index.html` — Swagger UI 交互式接口文档
- `/account`: 注册、登录、刷新、退出、资料与用户查询
- `/video`: 发布、删除、上传、详情、作者视频、分片上传
- `/feed`: 最新流、点赞榜、热榜、标签 Feed、关注流
- `/like`: 点赞、取消点赞、点赞状态、我的点赞列表
- `/comment`: 评论发布、删除、列表
- `/social`: 关注、取消关注、粉丝/关注列表、关系计数
- `/message`: 私信发送与列表
- `/notification`: SSE 通知流、通知列表、已读、未读数

## 可观测性

启动 API 后,以下端点直接可用,无需额外配置:

- **`GET /metrics`** — Prometheus 文本格式指标。自定义指标:
  - `http_requests_total{method,path,status}` / `http_request_duration_seconds{method,path}` — 接口流量与延迟
  - `cache_hit_total{component,layer}` / `cache_miss_total{component,layer}` — 视频实体本地/Redis 缓存命中率、热榜聚合命中率
  - `mq_publish_total{exchange,routing_key,result}` / `mq_consume_total{queue,result}` — MQ 投递与消费状态(success/error/retry/drop)
  - `rate_limit_reject_total{key_prefix}` — Lua 限流拒绝计数
  - `outbox_pending_messages` — Outbox 待投递消息数
- **`GET /swagger/index.html`** — Swagger UI;原始规范在 `/swagger/doc.json`
- **结构化日志** — Zap JSON 输出到 stdout,每条请求日志自动包含 `trace_id` 字段
- **链路追踪** — OpenTelemetry 默认 10% 采样,span 通过 `stdouttrace` 打到 stderr;接入 Jaeger/OTel Collector 只需替换 `internal/observability/otel.go` 的 exporter
- **pprof** — 开关在 `configs/*.yaml` 的 `observability.pprof`,API 与 Worker 各自独立端口

### 重新生成 Swagger 文档

修改 handler 注释后执行:

```sh
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/server/main.go -o internal/docs --parseDependency --parseInternal
```

### 端到端验证脚本

```sh
go run ./cmd/verify_observability   # 检查 /metrics, /swagger/doc.json, /swagger/index.html
go run ./cmd/verify_trace           # 触发 30 次请求,确认 trace span dump 到 stderr
```

## 后续优化方向

- 增加端到端接口测试和压测脚本，补充 Feed 延迟、缓存命中率和数据库查询次数等可复现指标。
- 对上传文件存储增加对象存储适配层，便于从本地目录切换到云存储。
- 接入 OTLP/Jaeger exporter 替换默认 stdouttrace,生产化链路追踪。
- 提供 Grafana 仪表盘 JSON,把 `/metrics` 暴露的指标可视化。
