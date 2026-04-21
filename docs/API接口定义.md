# RESTful API 接口定义

## 1. 总体规范

### 基础路径

```text
/api/v1
```

### 入口拓扑（HW6）

```text
Browser  ──▶  Nginx :80  ──▶  Gateway :8000  ──▶  microservice (8081/8082/8083)
            (静态 + /api/*)   (Nacos 发现 +
                               sentinel 限流/熔断/降级)
```

直接访问 `gateway:8000` 与经过 Nginx 走 `:80/api/*` 在协议层等价；JMeter HW6 场景使用前者绕开静态层做更高 QPS 压测。

### 认证分级

| 级别        | 说明     | Header                                    |
| ----------- | -------- | ----------------------------------------- |
| 公开        | 无需登录 | 无                                        |
| JWT         | 登录用户 | `Authorization: Bearer <token>`           |
| JWT + Admin | 管理员   | `Authorization: Bearer <token>`（role=1） |

### 统一响应格式

**成功响应：**

```json
{
  "code": 200,
  "message": "success",
  "data": { ... }
}
```

**错误响应：**

```json
{
  "code": 400,
  "message": "",
  "data": null
}
```

### 分页参数

列表接口统一使用查询参数分页：

| 参数      | 类型 | 默认值 | 说明                 |
| --------- | ---- | ------ | -------------------- |
| page      | int  | 1      | 页码                 |
| page_size | int  | 10     | 每页数量（最大 100） |

**分页响应格式：**

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "items": [...],
    "total": 100,
    "page": 1,
    "page_size": 10
  }
}
```

---

## 2. 用户服务（User Service）

### 2.1 用户注册

- **POST** `/api/v1/auth/register`
- **认证：** 无
- **请求体：**

```json
{
  "username": "testuser",
  "password": "password123",
  "email": "test@example.com",
  "phone": "13800138000"
}
```

- **成功响应（201）：**

```json
{
  "code": 201,
  "message": "注册成功",
  "data": {
    "id": 1,
    "username": "testuser"
  }
}
```

- **错误场景：** 用户名已存在（409）、邮箱已存在（409）、参数缺失（400）

### 2.2 用户登录

- **POST** `/api/v1/auth/login`
- **认证：** 无
- **请求体：**

```json
{
  "username": "testuser",
  "password": "password123"
}
```

- **成功响应（200）：**

```json
{
  "code": 200,
  "message": "登录成功",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "username": "testuser"
  }
}
```

- **错误场景：** 用户不存在（401）、密码错误（401）

### 2.3 获取用户信息

- **GET** `/api/v1/user/profile`
- **认证：** JWT

- **成功响应（200）：**

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": 1,
    "username": "testuser",
    "email": "test@example.com",
    "phone": "13800138000",
    "role": 0
  }
}
```

### 2.4 更新用户信息

- **PUT** `/api/v1/user/profile`
- **认证：** JWT
- **请求体：**

```json
{
  "email": "newemail@example.com",
  "phone": "13900139000"
}
```

---

## 3. 商品服务（Product Service）

### 3.1 商品列表

- **GET** `/api/v1/products?page=1&page_size=10`
- **认证：** 无
- **成功响应（200）：** 分页格式，items 为商品数组

### 3.2 商品详情

- **GET** `/api/v1/products/:id`
- **认证：** 无
- **成功响应（200）：**

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": 1,
    "name": "iPhone 16 Pro",
    "description": "最新款手机",
    "price": 7999.0,
    "category": "手机",
    "image_url": "",
    "status": 1,
    "inventory": {
      "total": 1000,
      "available": 998,
      "locked": 2
    }
  }
}
```

- **错误场景：** 商品不存在（404）

### 3.3 创建商品

- **POST** `/api/v1/products`
- **认证：** JWT + Admin
- **请求体：**

```json
{
  "name": "iPhone 16 Pro",
  "description": "最新款手机",
  "price": 7999.0,
  "category": "手机",
  "image_url": ""
}
```

### 3.4 更新商品

- **PUT** `/api/v1/products/:id`
- **认证：** JWT + Admin
- **请求体：** 同创建，字段可选

### 3.5 删除商品

- **DELETE** `/api/v1/products/:id`
- **认证：** JWT + Admin

---

## 4. 库存服务（Inventory Service）

### 4.1 查询库存

- **GET** `/api/v1/inventory/:product_id`
- **认证：** 无
- **成功响应（200）：**

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "product_id": 1,
    "total": 1000,
    "available": 998,
    "locked": 2
  }
}
```

### 4.2 设置库存

- **PUT** `/api/v1/inventory/:product_id`
- **认证：** JWT + Admin
- **请求体：**

```json
{
  "total": 1000,
  "available": 1000
}
```

### 4.3 扣减库存

- **POST** `/api/v1/inventory/:product_id/deduct`
- **认证：** JWT
- **请求体：**

```json
{
  "quantity": 1
}
```

- **说明：** 使用乐观锁（version 字段）防止超卖，最多重试 3 次
- **错误场景：** 库存不足（409）、乐观锁冲突重试失败（409）

---

## 5. 订单服务（Order Service）

### 5.1 创建订单（秒杀下单）

- **POST** `/api/v1/orders`
- **认证：** JWT
- **请求体：**

```json
{
  "product_id": 1,
  "quantity": 1
}
```

- **成功响应（201）：**

```json
{
  "code": 201,
  "message": "下单成功",
  "data": {
    "id": 1,
    "order_no": "ORD20260313143000001",
    "product_id": 1,
    "quantity": 1,
    "total_price": 7999.0,
    "status": 0
  }
}
```

- **处理流程（Saga 模式）：**
  1. 校验商品存在且在售
  2. 开启事务：扣减库存 → 创建订单
  3. 任一步骤失败则回滚

- **错误场景：** 商品不存在（404）、商品已下架（400）、库存不足（409）

### 5.2 我的订单列表

- **GET** `/api/v1/orders?page=1&page_size=10`
- **认证：** JWT
- **说明：** 只返回当前登录用户的订单

### 5.3 订单详情

- **GET** `/api/v1/orders/:id`
- **认证：** JWT

### 5.4 取消订单

- **PUT** `/api/v1/orders/:id/cancel`
- **认证：** JWT
- **说明：** 取消订单并归还库存（事务操作）
- **错误场景：** 订单不存在（404）、订单已取消（400）

---

## 6. 秒杀接口（HW4，由 order-service 提供）

### 6.1 秒杀下单

- **POST** `/api/v1/seckill`
- **认证：** JWT
- **归属服务：** `order-service:8081`（经 gateway 路由 `route:seckill`，受 sentinel 限流/熔断保护）
- **请求体：**

```json
{
  "product_id": 1,
  "quantity": 1
}
```

- **成功响应（202）：** 立刻返回排队中状态，订单创建为异步过程

```json
{
  "code": 202,
  "message": "秒杀请求已受理",
  "data": {
    "order_no": "1735286847123456789"
  }
}
```

- **处理流程：**
  1. Redis SETNX 幂等键 `seckill:user:{uid}:product:{pid}` (TTL 24h)
  2. Redis DECRBY 预扣库存 `seckill:stock:{pid}`
  3. Snowflake 生成 `order_no`
  4. 写 `orders` (status=0) + `outbox_events` (ORDER_CREATED) 同事务
  5. Outbox relay 异步投递到 Kafka，触发后续 Saga
- **错误场景：** 重复下单（409）、库存售罄（409）、限流（503，由网关返回）

### 6.2 查询秒杀结果

- **GET** `/api/v1/seckill/result?order_no=xxx`
- **认证：** JWT
- **响应：**

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "order_no": "1735286847123456789",
    "status": "PENDING|SUCCESS|FAILED",
    "order_status": 0
  }
}
```

- **说明：** 前端轮询本接口直到 `SUCCESS/FAILED`；状态来自 Redis `seckill:result:{order_no}`，由消费者写入

## 7. 库存扣减事件接口（HW5）

库存扣减不再暴露同步 HTTP 接口，而是由 `inventory-service` 消费 `order-events` 主题中 `ORDER_CREATED` 事件触发。同步扣减接口仅保留给管理员调试：

- **POST** `/api/v1/inventory/:product_id/deduct`（**Admin** 鉴权）

业务方应通过 `POST /api/v1/seckill` 间接驱动扣减。

## 8. 网关与基础设施端点（HW6）

### 8.1 网关健康检查

- **GET** `/health` （无论从 Nginx :80 还是 gateway :8000）
- **响应：** `{"status":"ok","service":"gateway"}` 或对应微服务的 service 名

### 8.2 网关实例池快照

- **GET** `/gateway/instances`
- **认证：** 无
- **响应：**

```json
{
  "order-service":     ["172.18.0.7:8081"],
  "inventory-service": ["172.18.0.8:8082"],
  "payment-service":   ["172.18.0.9:8083"]
}
```

用于排查 Nacos 订阅是否正常更新实例列表。

### 8.3 流量治理响应

当请求被 sentinel 拒绝（限流命中或熔断打开），网关返回：

```json
{
  "code": 503,
  "message": "service degraded, please retry later",
  "resource": "route:seckill",
  "reason": "BlockTypeFlow"
}
```

- `resource` 指明命中的路由资源
- `reason` 取自 sentinel `BlockType`（`Flow` / `CircuitBreaking` 等）

## 9. 接口归属与 sentinel 资源映射（HW6）

| 路由前缀                   | 归属服务            | 网关 sentinel resource | QPS 阈值 |
|----------------------------|---------------------|------------------------|----------|
| `/api/v1/auth`             | order-service       | `route:auth`           | default_qps |
| `/api/v1/user`             | order-service       | `route:user`           | default_qps |
| `/api/v1/products`         | order-service       | `route:products`       | default_qps |
| `/api/v1/orders`           | order-service       | `route:orders`         | default_qps |
| **`/api/v1/seckill`**      | order-service       | **`route:seckill`**    | **50（含错误率熔断）** |
| `/api/v1/inventory`        | inventory-service   | `route:inventory`      | default_qps |
| 健康检查 `/health`         | gateway 自身         | —                      | —          |
| `/gateway/instances`       | gateway 自身         | —                      | —          |
