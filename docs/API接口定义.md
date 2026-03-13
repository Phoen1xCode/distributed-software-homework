# RESTful API 接口定义

## 1. 总体规范

### 基础路径

```text
/api/v1
```

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

## 6. 健康检查

### 6.1 健康状态

- **GET** `/health`
- **认证：** 无
- **响应：**

```json
{
  "status": "ok"
}
```
