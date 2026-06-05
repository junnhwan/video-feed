# Video Feed — 简历优化方案

## 一、参考简历

> **短视频 Feed 流后端系统**
> Go · Gin · GORM · MySQL · Redis · RabbitMQ · Prometheus · Docker
>
> 面向千万级用户短视频场景，针对大 V 写扩散风暴、缓存击穿、流量突刺三大核心问题设计的信息流后端。API + Worker 双进程架构，覆盖 5 个业务模块、24 个接口、5 个异步消费者，单机压测 2.4 万 QPS、P95 < 13 ms、500 并发零错误。
>
> 核心方案：
>
> - **三级缓存 + 防击穿**：针对热点视频高并发回源导致 DB 瞬时过载问题，设计 go-cache → Redis → MySQL 三级缓存架构，结合 singleflight 合并并发请求与分布式锁串行化回源，消除缓存击穿风险。
> - **推拉结合 Feed 流**：针对大 V 发布时写扩散风暴（万级粉丝逐一推送）问题，设计推拉混合分发策略——普通用户写时 fanout 至活跃粉丝 inbox、大 V 读时按活跃度配额拉取；实现 k-way 堆归并，RTT 不随关注数线性增长。
> - **滑动窗口热榜**：针对固定窗口排行榜存在的边界突变与刷榜问题，基于 Redis ZSET 按分钟分桶 + 指数衰减加权合并最近 60 窗口，快照分页保证翻页一致性。
> - **Outbox + 幂等消费**：针对视频发布时 DB 与 MQ 双写一致性问题，设计本地消息表事务写入 + 轮询器批量投递保证最终一致性；消费端唯一索引 + 事务内 INSERT IGNORE 实现幂等，避免引入分布式事务。
> - **多层流量防护**：针对突刺流量与中间件故障的级联风险，设计 Redis 熔断器（gobreaker：连续失败熔断 → 半开探测 → 自动恢复）+ 基于 ZSET + Lua 的滑动窗口限流（按业务差异化配额）+ MQ 不可用时自动降级同步直写，核心链路零中断。
> - **可观测性驱动调优**：针对高并发下延迟劣化但无法定位瓶颈的问题，搭建 Prometheus 9 项指标 + Grafana 8 面板监控体系，定位 MySQL 连接池饱和瓶颈后针对性调优，comment 接口吞吐量提升 246%、P95 降低 56%。

---

## 二、优化后简历（基于 video-feed 真实代码）

> **短视频 Feed 流后端系统**
> Go · Gin · GORM · MySQL · Redis · RabbitMQ · Prometheus · Docker
>
> 面向千万级用户短视频场景，针对缓存击穿、热榜性能退化、异步一致性、流量突刺四大核心问题设计的信息流后端。API + Worker 双进程架构，覆盖 7 个业务模块、30 余个接口、8 个异步消费者，JMeter A/B 压测热榜 P99 降低约 85%、吞吐提升约 8 倍。
>
> 核心方案：
>
> - **三级缓存 + 热点探测防击穿**：针对热点视频高并发回源导致 DB 瞬时过载问题，设计 go-cache(L1, 3s) → Redis(L2, 1h) → MySQL 三级缓存架构；结合 singleflight 合并并发回源、分布式锁串行化缓存重建消除击穿风险；自研滑动窗口热点探测器（60s 窗口 / 10s 分桶），按访问频次 LOW / MEDIUM / HIGH 三级动态延长缓存 TTL，热点内容缓存命中率提升至 XX%。
> - **分片 ZSET 热榜 + 快照分页**：针对固定窗口排行榜存在的边界突变与刷榜问题，基于 Redis ZSET 按分钟分桶写入互动分数（2h 过期），读路径 ZUNIONSTORE 聚合最近 60 个窗口并缓存合并结果（2min TTL），asOf 快照时间戳保证翻页一致性；singleflight 合并并发回源，JMeter A/B 压测较 DB fallback P99 降低约 85%、吞吐提升约 8 倍。
> - **Outbox + 幂等消费**：针对视频发布时 DB 与 MQ 双写一致性问题，设计本地消息表同事务写入 + 轮询器秒级批量投递保证最终一致性；消费端 OnConflict DoNothing（等价 INSERT IGNORE）实现幂等点赞/评论，DLX 死信 + 最大重试 3 次兜底异常消息，避免引入分布式事务。
> - **多层流量防护**：针对突刺流量与中间件故障的级联风险，设计 Redis 熔断器（gobreaker：连续失败熔断 → 半开探测 → 自动恢复）+ 基于 ZSET + Lua 的滑动窗口限流（按业务差异化配额）+ MQ 不可用时自动降级同步直写，核心链路零中断。

### 差异化亮点

相比参考简历，本项目的独有优势：

- **自研热点探测器**：运行时自适应——滑动窗口分桶统计访问频次 → 分级延长缓存 TTL，比固定 TTL 缓存高一个档次，参考简历没有。
- **热榜有量化数据**：JMeter A/B 对比实验，P99 ↓ ~85%、吞吐 ↑ ~8x，不是空谈。

---

## 三、简历描述 ↔ 代码证据对照

每一条简历描述都能在代码中找到对应实现，面试时可以随时展开。

| 简历描述 | 代码位置 | 状态 |
|---------|---------|:---:|
| go-cache(L1, 3s) 本地缓存 | `internal/feed/service.go:37` `localcache.New(3\*time.Second, ...)` | ✅ |
| Redis(L2, 1h) 实体缓存 | `internal/feed/service.go:279-311` `setVideoEntityCaches` entityCacheTTL=1h | ✅ |
| MySQL(L3) fallback | `internal/feed/service.go:266-290` `loadVideoEntitiesFromDB` | ✅ |
| singleflight 合并回源 | `internal/feed/service.go:29` `requestGroup singleflight.Group` | ✅ |
| 分布式锁串行化缓存重建 | `internal/video/service.go:111-151` `getDetailWithRedisRebuild` Lock + spin wait | ✅ |
| 热点探测器 LOW/MED/HIGH | `internal/cache/hotkey/detector.go` 滑动窗口分桶 + `ExtendTTL` | ✅ |
| ZSET 按分钟分桶 ZINCRBY | `internal/video/popularity_cache.go:42-48` `windowKey` + `ZIncrBy` | ✅ |
| ZUNIONSTORE 聚合 60 窗口 | `internal/feed/service.go:501-506` 循环构造 60 个 key + `ZUnionStore` | ✅ |
| asOf 快照时间戳分页 | `internal/feed/service.go:496-498` `asOf` 锚定 + `offset` 分页 | ✅ |
| Outbox 同事务写入 | `internal/video/service.go:63-74` `tx.Create(&OutboxMsg{...})` | ✅ |
| 轮询器秒级批量投递 | `internal/worker/timeline_worker.go:106-147` 1s 间隔 + `Limit(100)` | ✅ |
| INSERT IGNORE 幂等消费 | `internal/video/like_repo.go:33-35` `OnConflict{DoNothing: true}` | ✅ |
| DLX 死信 + 最大重试 3 次 | `internal/middleware/rabbitmq/dlx.go` + `MaxRetryCount` | ✅ |
| MQ 降级同步直写 | `internal/video/like_service.go:61-85` publisher 失败 → `applyLike` 直写 DB | ✅ |
| MQ 降级测试覆盖 | `internal/video/mq_fallback_test.go` + `internal/social/mq_fallback_test.go` | ✅ |
| gobreaker 熔断器 | — | ❌ 需新增 |
| ZSET + Lua 滑动窗口限流 | — | ❌ 需新增 |

---

## 四、待实现技术点

### 1. gobreaker 熔断器（预计 1 天）

**目标**：包裹 Redis 和 MQ 的外部依赖调用，连续失败时自动熔断，半开探测后恢复。

**实现要点**：

- 引入 `github.com/sony/gobreaker`
- 为 Redis Client 和 MQ Publisher 各创建 breaker 实例
- 熔断触发时走降级路径（DB 直写 / 缓存穿透放行）
- Prometheus 指标记录熔断状态变化

**影响范围**：

- `internal/middleware/redis/redis.go` — Client 方法加 breaker 包裹
- `internal/middleware/rabbitmq/rabbitmq.go` — Publisher 加 breaker 包裹
- `internal/observability/metrics.go` — 新增 breaker 状态指标

### 2. ZSET + Lua 滑动窗口限流（预计 1-2 天）

**目标**：将当前固定窗口限流（INCR + EXPIRE）升级为滑动窗口限流（ZADD 时间戳 + Lua 脚本原子清理+计数）。

**实现要点**：

- Lua 脚本：移除窗口外成员 → ZADD 当前时间戳 → 返回窗口内计数
- 按业务配置差异化配额（写接口严格、读接口宽松）
- 保留 Redis 异常时降级放行的策略
- 替换 `internal/middleware/ratelimit/ratelimit.go` 中的固定窗口逻辑

**影响范围**：

- `internal/middleware/ratelimit/ratelimit.go` — 重写限流核心逻辑
- `internal/middleware/ratelimit/ratelimit_test.go` — 补充滑动窗口测试
- `internal/observability/metrics.go` — 复用已有 `RateLimitRejectTotal`

### 3. 缓存命中率压测（预计半天）

**目标**：填补简历中 "XX%" 的缓存命中率数据。

**实现方式**：

- 利用已有的 Prometheus 指标 `cache_hit_total` 和 `cache_miss_total`
- 压测后查询：`cache_hit_total / (cache_hit_total + cache_miss_total) * 100`
- 分别统计 local / redis 两层的命中率
- 将结果填入简历 "热点内容缓存命中率提升至 XX%"

**注意**：该指标已在代码中埋点完成（`internal/feed/service.go` 中每层缓存 hit/miss 都有 `Inc()`），只需跑压测 + 查询即可。

### 4. 可选：绝对 QPS 数据

**目标**：在简历概述中加入类似 "单机压测 XXX QPS" 的绝对性能数据。

**实现方式**：

- 使用已有的 JMeter 压测工具 (`bench/`) 跑一轮完整压测
- 记录 RPS、P95、P99、错误率
- 数据格式参考 ripple-note 项目的 `docs/12-load-test.md`
