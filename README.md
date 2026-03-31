# 商品库存与秒杀系统

分布式软件开发课程作业 — 基于 Go 语言的高并发商品秒杀系统，采用渐进式架构演进，涵盖容器化部署、负载均衡、分布式缓存、消息队列、读写分离等分布式核心技术。

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
| 反向代理   | Nginx（负载均衡 + 动静分离）           |
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

## 系统架构

```text
                    ┌─────────┐
                    │  Client │
                    └────┬────┘
                         │
                    ┌────▼────┐
                    │  Nginx  │ :80
                    │  (LB)   │
                    └────┬────┘
                   ┌─────┴─────┐
              ┌────▼───┐  ┌────▼───┐
              │ App-1  │  │ App-2  │
              │ :8081  │  │ :8082  │
              └──┬─┬───┘  └──┬─┬───┘
                 │ │         │ │
    ┌────────────┼─┼─────────┼─┼────────────┐
    │            │ │         │ │             │
┌───▼──┐   ┌────▼─▼───┐  ┌──▼─▼──┐   ┌─────▼─────┐
│ PG   │   │  Redis   │  │ PG    │   │   Kafka   │
│Master│   │  Cache   │  │Replica│   │  + ZK     │
│:5432 │   │  :6379   │  │:5433  │   │  :9092    │
└──────┘   └──────────┘  └───────┘   └───────────┘
  写库        缓存/库存      读库       消息队列
```

## 项目结构

```text
flash-sale/
├── cmd/server/main.go           # 应用入口，依赖注入
├── internal/
│   ├── user/                    # 用户模块（注册、登录、JWT）
│   ├── product/                 # 商品模块（CRUD + Redis 缓存装饰器）
│   │   ├── service.go           # 核心业务逻辑
│   │   ├── cached_service.go    # 缓存装饰器（穿透/击穿/雪崩防护）
│   │   └── service_interface.go # ProductServicer 接口
│   ├── inventory/               # 库存模块（乐观锁 + Redis 预扣）
│   ├── order/                   # 订单模块
│   │   ├── service.go           # 同步下单（幂等校验 + 事务）
│   │   ├── seckill_service.go   # 秒杀服务（Redis预扣 + Kafka投递）
│   │   ├── seckill_handler.go   # 秒杀 API 端点
│   │   └── seckill_consumer.go  # Kafka 消费者（异步创建订单）
│   └── middleware/auth.go       # JWT 认证 + Admin 鉴权中间件
├── pkg/
│   ├── snowflake/               # 雪花算法 ID 生成器
│   ├── kafka/                   # Kafka Producer/Consumer 封装
│   ├── cache/                   # Redis 客户端封装
│   ├── database/                # PostgreSQL 连接 + 读写分离
│   ├── config/                  # Viper 配置管理
│   └── response/                # 统一 JSON 响应格式
├── web/                         # 前端 SPA（HTML/CSS/JS）
├── nginx/                       # Nginx 配置（3 种负载均衡算法）
├── config/
│   ├── config.yaml              # 开发环境配置
│   └── config.docker.yaml       # Docker 环境配置
├── Dockerfile                   # 多阶段构建
├── docker-compose.yaml          # 完整部署（8 个服务）
├── docker-compose.dev.yaml      # 开发环境（仅 DB + Redis）
└── jmeter.jmx                   # JMeter 压测计划
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

| 服务              | 地址                  |
| ----------------- | --------------------- |
| Nginx 入口        | http://localhost:80    |
| 后端实例 1        | http://localhost:8081  |
| 后端实例 2        | http://localhost:8082  |
| PostgreSQL 主库   | localhost:5432         |
| PostgreSQL 从库   | localhost:5433         |
| Redis             | localhost:6379         |
| Kafka             | localhost:9092         |

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

测试计划包含 4 个线程组（各 50 并发）：

1. **用户注册/登录** — 注册唯一用户 + 登录获取 JWT Token
2. **商品浏览** — 分页查询商品列表
3. **秒杀下单** — 登录 → 秒杀抢购 → 轮询结果
4. **静态文件** — 请求 CSS 文件（验证动静分离）

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

### 秒杀流程

```text
客户端 POST /seckill
  │
  ├─ 1. Redis SETNX 幂等校验（同用户同商品仅一次）
  ├─ 2. 检查商品状态（在售）
  ├─ 3. Redis DECRBY 原子预扣库存（毫秒级）
  ├─ 4. 雪花算法生成订单号
  ├─ 5. 发送消息到 Kafka（异步）
  └─ 6. 返回 "排队中" + order_no
            │
      Kafka Consumer 异步处理
            │
            ├─ DB 事务：乐观锁扣减库存 + 创建订单
            ├─ 成功 → Redis 标记 SUCCESS
            └─ 失败 → 回滚 Redis 库存 + 清除幂等键
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

## 设计文档

详细设计文档位于 `docs/` 目录：

- [系统架构设计](docs/系统架构设计.md) — 服务拆分、分层架构、部署架构图
- [API 接口定义](docs/API接口定义.md) — 各服务 RESTful 接口详细定义
- [数据库 ER 设计](docs/数据库ER设计.md) — Mermaid ER 图、表结构、设计要点
- [技术栈选型说明](docs/技术栈选型说明.md) — 各技术选型理由与备选对比
