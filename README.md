# IM System

## 系统拆分

### 接入层 (Connection Service) —— 处理 WebSocket 连接和 HTTP 请求

- Nginx 
    - 作为 HTTP 流量入口，负责 SSL 终结、静态资源服务和 API 流量的负载均衡。
- Gateway (WebSocket 网关)
    - 有状态服务：维护海量 WebSocket 长连接。
    - 鉴权：在连接建立时进行 Token 校验。
    - 消息路由：
        - 上行：接收客户端消息，不进行复杂业务处理，直接投递到 Kafka。
        - 下行：订阅 Redis Pub/Sub (或 Kafka) 的推送通道，将消息精准推送给目标连接。

### 逻辑层 (Business Service) —— 处理业务逻辑并消费消息

- API 网关：处理登录、注册、建群、加群等 HTTP 请求，调用 Service 层复用核心逻辑。
- Service：封装业务逻辑，和 postgresql / Redis 交互
- 消费者：定序、过滤、存储分发

### 存储与中间件层 (Storage & Middleware)

- Kafka
    - 核心缓冲带。当秒杀或热点事件导致流量暴增时，保护后端数据库不被压垮。
- Redis
    - Seq ID: 利用 INCR 原子操作生成消息序列号，保证群聊/单聊消息严格有序。
    - Pub/Sub: 实现逻辑层到网关层的即时通知机制。
    - Cache: 缓存用户信息和在线状态。
- PostgreSQL
    - 系统的 Source of Truth (真实数据源)，存储所有持久化数据。
    - 搜索聊天记录

### 消息路由

1. 上行消息 (Write Path)
    - 客户端 A 发送 WebSocket 消息至 Gateway。
    - Gateway 将消息封装后投递到 Kafka。
    - Consumer 消费 kafka 消息，调用 Service。
    - Service 处理业务逻辑（生成 ID、验证权限）
    - 并行写入 PostgreSQL 和发布到 Redis Pub/Sub
```
Client → Gateway → Kafka → Consumer → Service → PostgreSQL + Redis Pub/Sub
```

2. 下行消息 (Read Path)
    - 全量广播给所有 Gateway 节点
        - Gateway 订阅 Redis Pub/Sub 对应频道。
    - Gateway 查找本地 WebSocket 连接
        - 收到通知后，Gateway 根据用户 ID 查找本地维护的 WebSocket 连接。
        - 通过 WebSocket 将消息 Push 给**在线的**客户端。
```
Service → Redis Pub/Sub → Gateway → Client
```

3. 历史消息同步 (Sync Path)
    - 客户端重新上线或断线重连，发送 HTTP 请求（带 Last_Seq_ID）
    - Service 查询 PostgreSQL，返回增量消息列表
```
Client → API Gateway → Service → PostgreSQL → Client
```


### 生产环境部署

**Kubernetes 部署：**

- 使用 Deployment 管理无状态服务（API、Consumer）
- 使用 StatefulSet 管理有状态服务（Gateway）
- 使用 Service 进行服务发现
- 使用 Ingress 管理外部访问
- 使用 ConfigMap 管理配置
- 使用 Secret 管理敏感信息

**监控与告警：**

- Prometheus + Grafana 监控系统指标
- ELK Stack 收集和分析日志
- Jaeger 进行分布式追踪
- AlertManager 发送告警通知


### 核心特性

- 基于 WebSocket 的实时双向通信
- 分布式架构，支持水平扩展
- kafka 消息队列解耦，保证高可用
- 雪花算法生成全局唯一消息 ID
- gRPC + Protobuf (服务间通信)
- 前端基于 Fetch API
- Nginx 反代 + 负载均衡
- Docker 容器化

#### 一致性哈希实现动态扩缩容

使用一致性哈希环管理 Gateway 节点：

```
         Node A (120°)
              ╱
             ╱
            ╱
   ────────●────────
  ╱                 ╲
 ╱                   ╲
●                     ●
Node C (240°)    Node B (0°)
```

- 用户连接根据 UserID 哈希到特定 Gateway
- 节点增删时，只影响相邻节点的部分连接
- 加入虚拟节点提高负载均衡


#### 服务发现

- 使用 Redis 或 etcd 存储 Gateway 节点列表
- Gateway 启动时注册自己的地址和端口
- 定期发送心跳保持在线状态
- 节点下线时自动从列表移除


## 性能指标

### 容量
- **并发连接**: 10,000+ / Gateway 节点
- **消息吞吐**: 10,000+ 消息/秒
- **消息延迟**: < 100ms (P99)

### 资源使用
- **Gateway**: ~2GB RAM, 2 CPU (10K 连接)
- **API**: ~1GB RAM, 2 CPU
- **Consumer**: ~1GB RAM, 2 CPU

## 安全性

### 认证和授权
- JWT Token 认证
- Token 有效期 24 小时
- 密码 bcrypt 加密存储

### 限流保护
- 注册/登录: 10 次/分钟/IP
- 发送消息: 60 次/分钟/用户
- API 查询: 100 次/分钟/用户

### 输入验证
- 用户名: 3-20 字符，字母数字下划线
- 密码: 8-64 字符，必须包含字母和数字
- 消息内容: 最大 2000 字符


## 关键技术方案

### 为什么可以选择 Nginx 来做负载均衡？

Nginx 适合 c2s / s2c 这种形式的消息走向，而本项目不支持私聊（c2c），可以选用 Nginx 这种通用的解决方案。

### 消息有序性 (Sequence ID)

为了保证消息在群内的绝对顺序，不能依赖服务器系统时间。

- **方案**：每个群维护一个 `Group_Seq_ID` (利用 Redis `INCR`)。
- **流程**：
    1. 消息进入逻辑层，Redis 对 `Group:{GroupID}:Seq` 执行 `INCR` 拿到 `MsgID`。
    2. 消息带上 `MsgID` 存入 PG 和 Redis 缓存。
    3. 客户端收到消息后，对比本地维护的 `Last_MsgID`。
        - 如果 `New_MsgID = Last_MsgID + 1`，正常显示。
        - 如果 `New_MsgID > Last_MsgID + 1`，说明中间丢包了，触发**主动拉取**逻辑。

### 状态保存与路由 (Redis)

需要知道“哪个用户连在哪个网关节点上”。

- **Redis Key 设计**：
    - `User:Connect:{UserID}` -> `gateway_node_ip:port`
- **流程**：
    1. 用户连接 Gateway A，Gateway A 在 Redis 写入映射关系，并设置过期时间（心跳续期）。
    2. 逻辑层要发消息给 User B 时，查 Redis 找到 Gateway B，通过 gRPC 将消息投递给 Gateway B。

### TODO：消息风暴优化

发送一条消息，需要推给百万人，这是系统的最大瓶颈。

1.  **消息合并 (Batching)**：
    - Gateway 不会每收到一条群消息就立马推给客户端。
    - **策略**：每 50ms 或 100ms 聚合一次该群的新消息，打包成一个 Packet 下发。

2.  **消息过滤与优先级**：
    - 对于百万群，不仅要“推”，还要“丢”。
    - **非重要消息**（如“入群欢迎”、“点赞”）可以只推给当前活跃窗口的用户，或者直接丢弃，让用户下拉刷新时再拉取。

### TODO：未读数管理 (Redis ZSET 优化)

对于百万群，实时维护每个人的精确未读数成本极高。

- **通用方案**：`Group_Current_Seq` (群最新ID) - `Member_Last_Read_Seq` (成员已读ID) = 未读数。
- **展示优化**：
    - 超过 99 条显示 "99+"。
    - **ZSET 只存最近 100 条热消息 ID** 用于快速补齐列表，未读数通过 Seq 相减计算即可。


## 踩坑日记

- 忘记改 .air.toml 的路径，导致请求打不到 handler
- 注意单机 TCP 并发连接数瓶颈

## 心得体会

- 大多数业务逻辑（校验、状态变更、token 失效等）放到 Service；Handler 只负责 HTTP/Context 层（解析请求、读 token、返回响应）
- Git 的使用：如果之前将文件 push 到远端，现在本地进行了修改，并不想放弃修改，而是想要 git 忽略该文件，可以 `git rm --cached <file>` 并且在 `.gitignore` 文件中补充即可

## 其他工具

| 工具        | 功能                  |
| ----------- | --------------------- |
| air         | 热加载                |
| rest client | 写 .http 文件发送请求 |
| cloc        | `cloc .` 统计代码行数 |
