# 商品库存与秒杀系统

分布式软件开发课程作业 — 基于 Go 语言的高并发商品秒杀系统，采用渐进式架构演进，涵盖容器化部署、负载均衡、分布式缓存、消息队列、读写分离、微服务拆分、事件驱动一致性、服务注册发现与流量治理等分布式核心技术。

## 技术栈

| 层级       | 技术                                  |
| ---------- | ------------------------------------- |
| 语言       | Go 1.23+                             |
| Web 框架   | Gin                                  |
| ORM        | GORM + DBResolver                    |
| 数据库     | PostgreSQL 16（主从读写分离）          |
| 缓存       | Redis 7（商品缓存 + 库存预扣 + 幂等） |
| 消息队列   | Apache Kafka（异步订单处理）           |
| 认证       | JWT (HS256) + bcrypt                  |
| 配置管理   | Viper（YAML + 环境变量覆盖）          |
| 容器化     | Docker + Docker Compose               |
| 反向代理   | Nginx（静态资源 + 入口）               |
| 服务发现   | Nacos 2.3 + nacos-sdk-go/v2           |
| API 网关   | Go (Gin + httputil.ReverseProxy)      |
| 流量治理   | sentinel-golang（限流 / 熔断 / 降级）  |
| 压力测试   | JMeter                                |

## 作业功能覆盖

### Homework 2: 容器化 + 负载均衡 + 动静分离 + 分布式缓存

- **容器化部署** — Dockerfile 多阶段构建，docker-compose 编排全部服务
- **Nginx 负载均衡** — 支持轮询、加权轮询、IP Hash 三种算法，2 个后端实例 (8081/8082)
- **动静分离** — Nginx 直接服务前端静态文件 (`/`)，API 请求代理到后端 (`/api/*`)
- **Redis 商品缓存** — Cache-Aside 模式缓存商品详情页
- **缓存三大问题防护**：
  - 穿透防护：缓存空值（null marker），防止恶意查询不存在的商品
  - 击穿防护：`singleflight.Group` 合并并发请求，防止缓存失效时的数据库风暴
  - 雪崩防护：随机化 TTL（5min + 0~60s 抖动），避免大量缓存同时过期

### Homework 3: 读写分离

- **PostgreSQL 主从架构** — Docker Compose 部署主库 + 从库
- **GORM DBResolver** — 写操作路由到主库，读操作自动路由到从库（Random 策略）
- **优雅降级** — 未配置从库时自动退化为单库模式

### Homework 4: 消息队列 + 秒杀系统

- **Kafka 异步处理订单** — 秒杀请求通过 Kafka 异步创建订单，实现削峰填谷
  - Producer：同步发送，WaitForAll 确认
  - Consumer：消费者组模式，事务性创建订单
- **Redis 库存预扣** — 秒杀热路径使用 Redis DECRBY 原子扣减，毫秒级响应
- **雪花算法订单ID** — 自研 Snowflake 实现（41bit 时间戳 + 10bit 节点 + 12bit 序列），支持分布式环境
- **幂等性控制** — 双重保障防止重复下单：
  - Redis SETNX 快速校验（同一用户 + 同一商品，24h TTL）
  - 数据库联合唯一索引 `(user_id, product_id)` 兜底
- **库存不超卖** — 乐观锁（version 字段）+ 事务保证数据一致性
- **秒杀结果查询** — Redis 缓存秒杀状态 (PENDING/SUCCESS/FAILED)，支持前端轮询

### Homework 5: 微服务拆分 + 分布式事务一致性

- **服务拆分** — `order-service` / `inventory-service` / `payment-service`，各自独立 PostgreSQL
- **事务性 Outbox** — 业务写库与事件落 outbox 表同事务，后台 relay 投递到 Kafka，保证至少一次
- **事件驱动 Saga** — 下单 → 扣库存 → 发起支付 → 支付完成 → 确认锁定库存的链路全部通过事件驱动
- **补偿** — 任一环节失败发布失败事件，回滚已扣减的 Redis 库存与已锁定的 DB 库存

### Homework 6: 服务注册发现与配置 + 流量治理

- **Nacos 注册发现** — 三个微服务启动时通过 `nacos-sdk-go/v2` 向 Nacos 注册健康实例，关闭时反注册
- **动态配置** — `payment-service` 监听 Nacos Data ID `payment.success_rate`，运维直接在 Nacos 控制台改值即热更新支付成功率（无需重启）
- **Go API 网关** — 自研 `cmd/gateway`：
  - 从 Nacos 订阅 + 定时轮询拉取后端实例列表，随机负载均衡
  - 使用 `httputil.ReverseProxy` 透明转发
  - 长前缀优先匹配，路由表见 `config/gateway.yaml`
- **流量治理（`sentinel-golang`）**：
  - **限流**：`/api/v1/seckill` 配置 50 QPS 阈值，超出 Reject 立即返回 503 + 友好提示
  - **熔断**：错误比例 ≥ 50% 触发，开启 5s 后进入半开探测；上游 5xx / 转发失败 / 无健康实例都计入错误
  - **降级**：被限流或熔断时返回统一 `{code:503, message:"service degraded..."}` JSON 响应
- **入口拓扑变更** — Nginx 仅做静态资源和入口，所有 `/api/*` 转发到 Go 网关

## 系统架构 (HW6)

```text
                              ┌─────────┐
                              │  Client │
                              └────┬────┘
                                   │
                              ┌────▼────┐
                              │  Nginx  │ :80
                              │ static  │
                              └────┬────┘
                                   │ /api/*
                              ┌────▼────┐
                              │ Gateway │ :8000  (sentinel: 限流 / 熔断 / 降级)
                              │  (Go)   │◄──┐
                              └────┬────┘   │ 服务发现 / 配置 / 实例订阅
                  ┌────────────────┼────────┼────────────────┐
                  │                │        │                │
            ┌─────▼──────┐  ┌──────▼─────┐  │     ┌──────────▼────────┐
            │  Order     │  │ Inventory  │  │     │   Payment         │
            │  :8081     │  │  :8082     │  │     │   :8083           │
            └──┬──┬──────┘  └────┬───────┘  │     └────────┬──────────┘
               │  │              │          │              │
               │  │              │          │              │
               │  │              │     ┌────▼────┐         │
               │  │              │     │  Nacos  │ :8848   │
               │  │              │     └─────────┘         │
               │  │              │                         │
        ┌──────▼┐ ▼┌─────────┐  │┌──────────┐         ┌──▼──────┐
        │ Order │  │  Redis  │  ││Inventory │         │ Payment │
        │  PG   │  │  cache  │  ││   PG     │         │   PG    │
        └───────┘  └─────────┘  │└──────────┘         └─────────┘
                                │
                          ┌─────▼─────┐
                          │  Kafka    │  (order/inventory/payment topics)
                          │  + ZK     │
                          └───────────┘
```

## 项目结构

```text
flash-sale/
├── cmd/
│   ├── server/main.go             # 单体入口（HW2~HW4 历史版本）
│   ├── order-service/main.go      # 订单 + 秒杀服务（HW5+）
│   ├── inventory-service/main.go  # 库存服务（HW5+）
│   ├── payment-service/main.go    # 支付服务（HW5+，HW6 含动态配置）
│   └── gateway/main.go            # HW6 Go API 网关（Nacos + sentinel）
├── internal/                      # 业务模块（user/product/inventory/order/payment）
├── pkg/
│   ├── registry/                  # HW6 Nacos 客户端封装
│   ├── outbox/                    # HW5 事务性 outbox + relay
│   ├── event/                     # HW5 事件类型定义
│   ├── snowflake/                 # 雪花算法 ID 生成器
│   ├── kafka/                     # Kafka Producer/Consumer 封装
│   ├── cache/                     # Redis 客户端封装
│   ├── database/                  # PostgreSQL 连接 + 读写分离
│   ├── config/                    # Viper 配置管理
│   └── response/                  # 统一 JSON 响应格式
├── web/                           # 前端 SPA
├── nginx/                         # Nginx 配置（HW2 LB + HW6 静态代理）
├── config/
│   ├── order-service.yaml
│   ├── inventory-service.yaml
│   ├── payment-service.yaml
│   └── gateway.yaml               # HW6 网关 + sentinel 规则
├── Dockerfile                     # 多阶段构建（按 SERVICE 参数选 cmd）
├── docker-compose.yaml            # 完整部署：3 DB + 3 服务 + Gateway + Nacos + Redis + Kafka + Nginx
├── docker-compose.dev.yaml        # 开发环境（仅 DB + Redis）
└── jmeter.jmx                     # JMeter 压测计划（含 HW6 限流场景）
```

## 快速启动

### 开发环境

```bash
cd flash-sale

# 1. 启动 PostgreSQL + Redis
docker compose -f docker-compose.dev.yaml up -d

# 2. 运行后端服务
go run cmd/server/main.go
```

服务默认运行在 `http://localhost:8080`，PostgreSQL 开发端口为 2077。

### Docker Compose 完整部署

```bash
cd flash-sale

# 一键启动所有服务
docker compose up --build -d
```

启动后：

| 服务              | 地址                          |
| ----------------- | ----------------------------- |
| Nginx 入口        | http://localhost:80           |
| Go 网关 (Gateway) | http://localhost:8000         |
| Nacos 控制台      | http://localhost:8848/nacos   |
| order-service     | http://localhost:8081         |
| inventory-service | http://localhost:8082         |
| payment-service   | http://localhost:8083         |
| Order PG          | localhost:5441                |
| Order PG Replica  | localhost:5444                |
| Inventory PG      | localhost:5442                |
| Payment PG        | localhost:5443                |
| Redis             | localhost:6379                |
| Kafka             | localhost:9092                |

### 切换负载均衡算法

```bash
# 加权轮询（3:1）
docker compose cp nginx/nginx-weighted.conf nginx:/etc/nginx/nginx.conf
docker compose exec nginx nginx -s reload

# IP Hash（会话保持）
docker compose cp nginx/nginx-iphash.conf nginx:/etc/nginx/nginx.conf
docker compose exec nginx nginx -s reload

# 恢复默认轮询
docker compose cp nginx/nginx.conf nginx:/etc/nginx/nginx.conf
docker compose exec nginx nginx -s reload
```

### JMeter 压力测试

```bash
jmeter -n -t jmeter.jmx -l result.jtl
```

测试计划包含 5 个线程组：

1. **用户注册/登录** — 50 并发，获取 JWT Token
2. **商品浏览** — 50 并发，分页查询商品列表
3. **秒杀下单** — 50 并发，登录 → 秒杀抢购 → 轮询结果
4. **静态文件** — 50 并发，请求 CSS（验证动静分离）
5. **HW6 流量治理** — 200 并发 × 20 轮直打 `gateway:8000/api/v1/seckill`，验证 Sentinel 限流后返回 503，下游故障时熔断生效

### Nacos 动态配置示例（HW6）

启动后访问 `http://localhost:8848/nacos`（无密码模式），按以下步骤验证动态配置：

1. **Configurations → Create Configuration**
2. Data ID 填 `payment.success_rate`，Group 填 `DEFAULT_GROUP`，Content 填 `0.3`
3. 发布后观察 `payment-service` 日志：`[payment] dynamic config payment.success_rate -> success_rate=0.300`
4. 再次执行秒杀，约 70% 的订单会进入 `PAYMENT_FAILED` → `ORDER_CANCELLED` 链路，且无需重启服务

### 流量治理实测路径（HW6）

```bash
# 触发限流：单线程不行，用 hey/ab/jmeter 直接灌爆 seckill 路由
hey -n 2000 -c 200 -m POST -H "Content-Type: application/json" \
    -d '{"product_id":1,"quantity":1}' \
    http://localhost:8000/api/v1/seckill

# 期望：超过 50 QPS 后大量返回 503（fallback JSON），少量 401（未鉴权）
# 同时 gateway 日志中可看到 sentinel 拒绝记录
```

## API 概览

基础路径：`/api/v1`

| 模块 | 端点                            | 说明                             |
| ---- | ------------------------------- | -------------------------------- |
| 认证 | `POST /auth/register`           | 用户注册                         |
| 认证 | `POST /auth/login`              | 用户登录，返回 JWT               |
| 用户 | `GET/PUT /user/profile`         | 用户信息查看与更新               |
| 商品 | `GET /products`                 | 商品列表（分页，走读库）         |
| 商品 | `GET /products/:id`             | 商品详情（Redis 缓存）           |
| 商品 | `POST/PUT/DELETE /products/:id` | 商品管理（Admin，走写库）        |
| 库存 | `GET /inventory/:product_id`    | 查询库存                         |
| 库存 | `PUT /inventory/:product_id`    | 设置库存（Admin）                |
| 订单 | `POST /orders`                  | 同步下单（幂等校验）             |
| 订单 | `GET /orders`                   | 我的订单列表                     |
| 订单 | `PUT /orders/:id/cancel`        | 取消订单（悲观锁 + 库存回退）    |
| 秒杀 | `POST /seckill`                 | 秒杀抢购（Redis预扣 + Kafka异步）|
| 秒杀 | `GET /seckill/result`           | 查询秒杀结果                     |

## 关键技术实现

### 秒杀流程（HW5+ 终态）

```text
客户端 POST /api/v1/seckill
  │
  ├─ 1. (网关) sentinel 限流（50 QPS，超出立即 503 fallback）
  ├─ 2. Redis SETNX 幂等校验（同用户同商品仅一次）
  ├─ 3. 检查商品状态（在售）
  ├─ 4. Redis DECRBY 原子预扣库存（毫秒级）
  ├─ 5. 雪花算法生成 order_no
  ├─ 6. 同事务：写入 orders（pending）+ outbox(ORDER_CREATED)
  └─ 7. 立即返回 "排队中" + order_no
            │
       Outbox relay → Kafka order-events
            │
       inventory-service 消费 ORDER_CREATED：
            ├─ DB 事务：乐观锁扣库存（available→locked）+ outbox(STOCK_DEDUCTED)
            └─ 失败：outbox(STOCK_DEDUCT_FAILED)
            │
       order-service 消费 STOCK_DEDUCTED → orders.status=1 (awaiting_payment)
                                          + outbox(PAYMENT_REQUESTED)
            │
       payment-service 消费 PAYMENT_REQUESTED → 模拟支付（success_rate 由 Nacos 动态配置）
            ├─ 成功：outbox(PAYMENT_SUCCESS)
            └─ 失败：outbox(PAYMENT_FAILED)
            │
       order-service 消费 PAYMENT_SUCCESS → orders.status=2 (paid)
                                           + outbox(ORDER_COMPLETED)
            │
       inventory-service 消费 ORDER_COMPLETED → locked→sold（最终扣减）
   补偿路径：
       任一失败事件触发 ORDER_CANCELLED → 回退 Redis 库存 + 库存回滚 + 清除幂等键
```

### 缓存策略

| 策略     | 实现                                       |
| -------- | ------------------------------------------ |
| 穿透防护 | 缓存空值（null marker），TTL 60s           |
| 击穿防护 | singleflight 合并并发缓存未命中请求        |
| 雪崩防护 | 基础 TTL 5min + 随机抖动 0~60s             |
| 优雅降级 | Redis 故障时自动降级到数据库直接查询       |

### 读写分离

| 操作类型       | 路由目标       |
| -------------- | -------------- |
| SELECT 查询    | PostgreSQL 从库 |
| INSERT/UPDATE/DELETE | PostgreSQL 主库 |

通过 GORM DBResolver 插件实现，对业务代码完全透明。

### 事务性 Outbox + Saga（HW5）

| 关键点 | 实现 |
| ------ | ---- |
| 写一致性 | 业务表写入与 `outbox_events` 写入在**同一本地事务**中，保证业务变更与事件落库同生共死 |
| 投递保证 | `pkg/outbox.Relay` 后台轮询 `sent=false` 的事件，按 `created_at` 顺序推到 Kafka，确保至少一次 |
| 消费幂等 | 事件含唯一 `EventID`；订单状态机 + 库存乐观锁让重复消费天然安全 |
| 补偿 | `STOCK_DEDUCT_FAILED` / `PAYMENT_FAILED` → 触发 `ORDER_CANCELLED`：删除 Redis 幂等键、回退预扣库存、回滚已锁定的 DB 库存 |

### 服务注册发现 + 流量治理（HW6）

| 关键点 | 实现 |
| ------ | ---- |
| 注册 | `pkg/registry` 封装 `nacos-sdk-go/v2`：服务启动 `RegisterInstance`，关闭 `DeregisterInstance` |
| 实例发现 | 网关 `Subscribe`（推）+ 5s 轮询 `SelectInstances`（拉）双保险维护 `instancePool` |
| 动态配置 | `payment-service` 监听 Nacos Data ID `payment.success_rate`，运维改值即热更新（`atomic.Uint64` + `math.Float64bits`） |
| 限流 | sentinel `flow.Rule` 对 `route:seckill` 限 50 QPS（Reject 模式） |
| 熔断 | `circuitbreaker.ErrorRatio` 50% / 10s 窗口 / 最少 10 请求 / 5s 半开探测；上游 5xx 也通过 `proxy.ModifyResponse` 计入错误 |
| 降级 | 限流/熔断命中时统一返回 `{code:503, message:"service degraded..."}` JSON envelope |

## 设计文档

详细设计文档位于 `docs/` 目录：

- [系统架构设计](docs/系统架构设计.md) — 服务拆分、分层架构、部署架构图
- [API 接口定义](docs/API接口定义.md) — 各服务 RESTful 接口详细定义
- [数据库 ER 设计](docs/数据库ER设计.md) — Mermaid ER 图、表结构、设计要点
- [技术栈选型说明](docs/技术栈选型说明.md) — 各技术选型理由与备选对比
