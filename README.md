# IM System

## 系统拆分

### 接入层 (Connection Service)

维护与客户端的 WebSocket 长连接，只做消息的转发，不处理具体业务逻辑。

- Nginx 
    - 作为 HTTP 流量入口，负责 SSL 终结、静态资源服务和 API 流量的负载均衡。
- Gateway (WebSocket 网关)
    - 有状态服务：维护海量 WebSocket 长连接。
    - 鉴权：在连接建立时进行 Token 校验。
    - 消息路由：
        - 上行：接收客户端消息，不进行复杂业务处理，直接投递到 Kafka。
        - 下行：订阅 Redis Pub/Sub (或 Kafka) 的推送通道，将消息精准推送给目标连接。

### 逻辑层 (Business Service)

- API 网关：处理登录、注册、建群、加群等 HTTP 请求，调用 Service 层复用核心逻辑。
- Service：封装业务逻辑，和 DB / Redis / ES 交互
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
- Elasticsearch
    - 专门用于解决“搜索聊天记录”这一高 IO 消耗的需求，避免对主库造成压力。

### 消息路由

1. 实时消息发送链路 (Write Path)
    - 客户端 A 发送 WebSocket 消息至 Gateway。
    - Gateway 将消息封装后 Produce 到 Kafka。
    - Consumer 消费 kafka 消息，调用 Service。
    - Service 请求 Redis 获取 Seq ID。
    - Service 并行写入 PostgreSQL (持久化) 和 Elasticsearch (索引)。
    - Service 将处理后的消息 Publish 到 Redis Pub/Sub。
2. 实时消息推送链路 (Read Path)
    - 全量广播给所有 Gateway 节点，由 Gateway 节点自行判断本地有哪些连接属于该群
        - Gateway 订阅 Redis Pub/Sub 对应频道。
        - 收到通知后，Gateway 根据用户 ID 查找本地维护的 WebSocket 连接。
        - 通过 WebSocket 将消息 Push 给**在线的**客户端。
3. 历史消息同步 (Sync Path)
    - 客户端 重新上线或断线重连。
    - 发送 HTTP 请求带上本地最新的 Last_Seq_ID 给 API Server。
    - API Server 查询 PostgreSQL 中大于该 ID 的消息列表。
    - 返回消息列表，客户端补齐缺失消息。

## 通信协议

- WebSocket, HTTP/REST
- gRPC
- Protobuf


## 关键技术方案

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

### 消息风暴优化

发送一条消息，需要推给百万人，这是系统的最大瓶颈。

1.  **消息合并 (Batching)**：
    - Gateway 不会每收到一条群消息就立马推给客户端。
    - **策略**：每 50ms 或 100ms 聚合一次该群的新消息，打包成一个 Packet 下发。

2.  **消息过滤与优先级**：
    - 对于百万群，不仅要“推”，还要“丢”。
    - **非重要消息**（如“入群欢迎”、“点赞”）可以只推给当前活跃窗口的用户，或者直接丢弃，让用户下拉刷新时再拉取。

3.  **Kafka 削峰**：
    - 用户发送消息 -> HTTP/WS -> Producer -> **Kafka Topic (GroupMsg)** -> Consumer (Group Service)。

### 未读数管理 (Redis ZSET 优化)

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

## 其他工具

| 工具        | 功能                  |
| ----------- | --------------------- |
| air         | 热加载                |
| rest client | 写 .http 文件发送请求 |
| cloc        | `cloc .` 统计代码行数 |
