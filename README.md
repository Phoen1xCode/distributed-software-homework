# 商品库存与秒杀系统

分布式软件开发课程作业 — 基于 Go 语言的商品库存与秒杀系统，采用渐进式架构演进，涵盖容器化部署、Nginx 负载均衡与动静分离。

## 技术栈

| 层级     | 技术                    |
| -------- | ----------------------- |
| 语言     | Go 1.23+                |
| Web 框架 | Gin                     |
| ORM      | GORM                    |
| 数据库   | PostgreSQL 16           |
| 认证     | JWT + bcrypt            |
| 配置管理 | Viper                   |
| 容器化   | Docker + Docker Compose |
| 反向代理 | Nginx                   |
| 压力测试 | JMeter                  |

## 项目结构

```text
flash-sale/
├── cmd/server/main.go           # 应用入口
├── internal/
│   ├── user/                    # 用户服务（注册、登录、JWT）
│   ├── product/                 # 商品服务（CRUD、分页查询）
│   ├── inventory/               # 库存服务（乐观锁扣减）
│   ├── order/                   # 订单服务（Saga 模式下单）
│   └── middleware/auth.go       # JWT 认证中间件
├── pkg/                         # 公共工具（配置、数据库、响应格式）
├── web/                         # 前端静态文件（HTML/CSS/JS）
├── nginx/                       # Nginx 配置（轮询/加权/IP Hash）
├── config/config.yaml           # 应用配置
├── Dockerfile                   # 多阶段构建
├── docker-compose.yaml          # 生产部署（PostgreSQL + 2 实例 + Nginx）
└── docker-compose.dev.yaml      # 开发环境（仅 PostgreSQL）
```

## 快速启动

### 开发环境

```bash
cd flash-sale

# 1. 启动 PostgreSQL
make dev/up

# 2. 运行后端服务
go run cmd/server/main.go
```

服务默认运行在 `http://localhost:8080`，PostgreSQL 开发端口为 2077。

### Docker Compose 完整部署

```bash
cd flash-sale

# 一键启动（PostgreSQL + 2 个后端实例 + Nginx）
docker compose up --build -d
```

启动后：

| 服务       | 地址                  |
| ---------- | --------------------- |
| Nginx 入口 | http://localhost:80   |
| 后端实例 1 | http://localhost:8081 |
| 后端实例 2 | http://localhost:8082 |

### 切换负载均衡算法

```bash
# 加权轮询
docker compose cp nginx/nginx-weighted.conf nginx:/etc/nginx/nginx.conf
docker compose exec nginx nginx -s reload

# IP Hash
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

测试计划包含两个线程组：

- 静态文件压测（10 线程，请求 `/css/style.css`）
- API 接口压测（1000 线程，请求 `/api/v1/products`）

## API 概览

基础路径：`/api/v1`

| 模块 | 端点                            | 说明                 |
| ---- | ------------------------------- | -------------------- |
| 认证 | `POST /auth/register`           | 用户注册             |
| 认证 | `POST /auth/login`              | 用户登录，返回 JWT   |
| 用户 | `GET/PUT /user/profile`         | 用户信息查看与更新   |
| 商品 | `GET /products`                 | 商品列表（分页）     |
| 商品 | `GET /products/:id`             | 商品详情（含库存）   |
| 商品 | `POST/PUT/DELETE /products/:id` | 商品管理（Admin）    |
| 库存 | `GET /inventory/:product_id`    | 查询库存             |
| 库存 | `PUT /inventory/:product_id`    | 设置库存（Admin）    |
| 订单 | `POST /orders`                  | 创建订单（秒杀下单） |
| 订单 | `GET /orders`                   | 我的订单列表         |
| 订单 | `PUT /orders/:id/cancel`        | 取消订单             |

## 设计文档

详细设计文档位于 `docs/` 目录：

- [系统架构设计](docs/系统架构设计.md) — 服务拆分、分层架构、部署架构图
- [API 接口定义](docs/API接口定义.md) — 各服务 RESTful 接口详细定义
- [数据库 ER 设计](docs/数据库ER设计.md) — Mermaid ER 图、表结构、设计要点
- [技术栈选型说明](docs/技术栈选型说明.md) — 各技术选型理由与备选对比
