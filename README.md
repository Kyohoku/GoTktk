# GoTik

> `GoTik` 是一个基于 Go 从零实现的短视频后端项目，围绕账号、视频、点赞、评论、关注与 Feed 主链路展开，并逐步接入 `MySQL + Redis + RabbitMQ`，重点实践缓存优化、热度排序、异步解耦与分页稳定性设计。

## 技术栈

| 维度 | 组件/工具 | 说明 |
| --- | --- | --- |
| 开发语言 | Go | 后端核心业务实现，包含 HTTP 服务与 Worker |
| Web 框架 | Gin | 路由注册、请求绑定、JSON 响应、中间件接入 |
| ORM | GORM | 模型定义、CRUD、事务与启动迁移 |
| 持久化 | MySQL | 存储账号、视频、点赞、评论、关注关系及基础热度字段 |
| 缓存/排序 | Redis | Token 缓存、视频详情缓存、匿名 Feed 缓存、实时热度与热榜 |
| 消息队列 | RabbitMQ | 点赞、评论、关注与热度更新的异步事件驱动 |
| 鉴权 | JWT | 支持 Redis 优先、MySQL 兜底、自愈式校验 |
| 文件存储 | Local Disk | 视频与封面文件本地存储 |
| 容器化 | Docker / Docker Compose | 支持主服务、Worker 与依赖组件一键拉起 |

## 核心模块

### 1. 账号模块

已完成能力：

1. 注册
2. 登录
3. 按 ID 查询用户
4. 获取当前登录用户信息
5. 退出登录

设计要点：

1. 登录成功后签发 JWT。
2. Token 同时存储到 MySQL 与 Redis。
3. 鉴权采用 Redis 优先、MySQL 兜底、自愈式回填。

### 2. 视频模块

已完成能力：

1. 发布视频记录
2. 按作者查询视频列表
3. 查询视频详情
4. 上传视频文件
5. 上传封面文件

设计要点：

1. `video/getDetail` 做了视频详情缓存。
2. 热点详情缓存支持互斥锁防击穿。
3. 视频详情中的 `likes_count` 与热度会随互动行为变化。

### 3. 点赞模块

已完成能力：

1. 点赞
2. 取消点赞
3. 查询是否已点赞
4. 查询当前用户点赞过的视频

设计要点：

1. 点赞关系单独建表。
2. 基于唯一索引限制重复点赞。
3. 点赞/取消点赞已异步化。
4. MQ 不可用时保留同步 fallback 直写。

### 4. 评论模块

已完成能力：

1. 发布评论
2. 删除评论
3. 查询某视频下全部评论

设计要点：

1. 评论发布与删除已异步化。
2. 评论发布会影响热度，当前规则为 `+2`。
3. 当前版本删除评论不回滚热度，属于阶段性取舍。

### 5. 关注模块

已完成能力：

1. 关注
2. 取关
3. 查询粉丝列表
4. 查询关注列表

设计要点：

1. 关注关系单独建表。
2. `follow/unfollow` 已异步化。
3. `follow` 消费端具备幂等思路，避免重复创建关注关系。

### 6. Feed 模块

已完成能力：

1. 按发布时间倒序拉取最新视频流：`/feed/listLatest`
2. 按点赞数倒序分页：`/feed/listLikesCount`
3. 按热度排序读取：`/feed/listByPopularity`

设计要点：

1. 匿名 `listLatest` 支持短 TTL 缓存。
2. `listLikesCount` 使用 `likes_count + id` 复合游标分页。
3. 热榜支持分钟桶、快照读取与 `as_of + offset` 思路。

## Redis 设计

### 当前使用场景

1. `account:<id>`：账号 token 缓存
2. `video:detail:id=<id>`：视频详情缓存
3. `feed:listLatest:limit=<limit>:before=<before>`：匿名 Feed 缓存
4. `lock:video:detail:id=<id>`：详情缓存互斥锁
5. `lock:feed:listLatest:...`：匿名 Feed 互斥锁
6. `hot:video:1m:<yyyyMMddHHmm>`：热度分钟桶
7. `hot:video:merge:1m:<asOf>`：热榜快照

### 已完成优化

1. Token 缓存：Redis 优先、MySQL 兜底、自愈回填。
2. 视频详情缓存：Cache Aside + 写后删缓存。
3. 匿名 Feed 缓存：短 TTL 分页缓存。
4. 缓存击穿保护：热点详情与匿名 Feed 的互斥锁防击穿。
5. 热榜排序：基于 ZSET 与分钟桶维护实时热度。

## RabbitMQ 设计

### 当前事件流

| 模块 | Exchange / Routing Key | 作用 |
| --- | --- | --- |
| 点赞 | `like.events` / `like.like` `like.unlike` | 异步处理点赞与取消点赞 |
| 评论 | `comment.events` / `comment.publish` `comment.delete` | 异步处理评论发布与删除 |
| 关注 | `social.events` / `social.follow` `social.unfollow` | 异步处理关注与取关 |
| 热度 | `video.popularity.events` / `video.popularity.update` | 异步更新 Redis 实时热度与缓存副作用 |

### 当前 Worker 拆分

1. `LikeWorker`
2. `CommentWorker`
3. `SocialWorker`
4. `PopularityWorker`

### 当前设计思路

1. 主服务负责前置校验与发布事件。
2. Worker 负责消费消息并完成最终执行。
3. 热度更新作为独立副作用事件流存在。
4. MQ 不可用时按职责分别做本地 fallback。

## 项目亮点

### 1. Redis 优先、MySQL 兜底、自愈式鉴权

登录成功后会把当前有效 token 写入 Redis，鉴权时优先读缓存，未命中再回源数据库，并在成功后回填 Redis，减少 MySQL 压力，同时保留 Redis 不可用时的降级能力。

### 2. 热点缓存击穿保护

`video/getDetail` 与匿名 `feed/listLatest` 都实现了基于 Redis `SETNX` 的互斥锁方案，缓存 miss 时只允许一个请求负责重建缓存，其余请求等待回填或走兜底读取。

### 3. 热榜不只是 ZSET，而是分钟桶 + 快照分页

项目当前不仅维护一个简单总榜，还使用分钟桶记录热度增量，并通过 `as_of + offset` 的快照分页思路保证热榜翻页更稳定，减少动态热榜中的重复与漏数问题。

### 4. RabbitMQ 驱动的业务异步化

点赞、评论、关注均已改造成“主服务发消息 + Worker 最终执行”的模式，主链路更轻，模块边界更清晰，也更便于后续扩展更多消费者与副作用处理。

### 5. 热度更新作为独立副作用事件流

点赞与评论除了触发业务写库，也会触发热度更新事件。当前将热度更新拆为 `PopularityMQ + PopularityWorker`，让业务主写与 Redis 实时热度更新进一步解耦，体现了对写扩散、事件驱动与最终一致性的理解。

## 项目结构

```text
GoTik
├─ cmd
│  ├─ main.go
│  └─ worker
│     └─ main.go
├─ configs
│  ├─ config.yaml
│  └─ config.docker.yaml
├─ internal
│  ├─ account
│  ├─ config
│  ├─ db
│  ├─ feed
│  ├─ http
│  ├─ middleware
│  │  ├─ rabbitmq
│  │  └─ redis
│  ├─ social
│  ├─ video
│  └─ worker
├─ .run
│  └─ uploads
├─ Dockerfile
├─ docker-compose.yml
└─ README.md
```

## 本地运行

### 1. 准备依赖

需要先准备：

1. MySQL
2. Redis
3. RabbitMQ

并确认 [configs/config.yaml](D:/Trabajar/GoTik/configs/config.yaml) 中的：

1. `database`
2. `redis`
3. `rabbitmq`

配置正确。

### 2. 启动 HTTP 主服务

```bash
go run ./cmd
```

### 3. 启动 Worker

```bash
go run ./cmd/worker
```

### 4. 本地联调说明

完整异步链路联调时，需要同时启动：

1. MySQL
2. Redis
3. RabbitMQ
4. HTTP 主服务
5. Worker 进程

## Docker 部署

当前项目已支持通过 Docker Compose 一键拉起：

1. `app`：HTTP 主服务
2. `worker`：异步消费进程
3. `mysql`
4. `redis`
5. `rabbitmq`

### 1. 相关文件

1. [Dockerfile](D:/Trabajar/GoTik/Dockerfile)
2. [docker-compose.yml](D:/Trabajar/GoTik/docker-compose.yml)
3. [configs/config.docker.yaml](D:/Trabajar/GoTik/configs/config.docker.yaml)

### 2. 启动前准备

确保本机已安装并启动：

1. Docker Desktop
2. Docker Compose

项目容器环境默认读取 [configs/config.docker.yaml](D:/Trabajar/GoTik/configs/config.docker.yaml)，其中依赖地址使用 compose 服务名：

1. `mysql`
2. `redis`
3. `rabbitmq`

### 3. 一键启动

在项目根目录执行：

```bash
docker compose up --build -d
```

首次启动会完成：

1. 构建 `gotik-app` 镜像
2. 构建 `gotik-worker` 镜像
3. 拉起 MySQL / Redis / RabbitMQ
4. 拉起 HTTP 主服务与 Worker

### 4. 查看运行状态

```bash
docker compose ps
```

查看主服务日志：

```bash
docker compose logs -f app
```

查看 Worker 日志：

```bash
docker compose logs -f worker
```

### 5. 端口说明

当前默认映射：

1. 后端接口：`8080 -> 8080`
2. MySQL：`3307 -> 3306`
3. Redis：`6380 -> 6379`
4. RabbitMQ：`5673 -> 5672`
5. RabbitMQ 管理台：`15673 -> 15672`

因此：

1. 后端接口默认访问：`http://localhost:8080`
2. RabbitMQ 管理台默认访问：`http://localhost:15673`

### 6. 数据持久化说明

当前 compose 使用了命名卷保存关键数据：

1. `mysql_data`
2. `redis_data`
3. `rabbitmq_data`
4. `uploads_data`

这意味着：

1. 普通 `docker compose down` 不会删除这些数据。
2. 重新 `docker compose up` 后数据仍会保留。
3. 如果执行 `docker compose down -v`，相关数据卷会一并删除。

### 7. Docker 环境联调建议

推荐按下面顺序验证：

1. 注册 / 登录
2. 上传封面与视频
3. 发布视频
4. 点赞
5. 发布评论
6. 关注 / 取关
7. 查看视频详情与 Feed

验证重点：

1. app 日志是否出现消息入队日志
2. RabbitMQ 管理台中消息是否被消费
3. 视频详情、点赞状态、评论列表、关注关系是否真实变化

### 8. 停止服务

```bash
docker compose down
```

如果需要连数据卷一起删除：

```bash
docker compose down -v
```

## 当前进度

### 已完成

1. 账号模块主链路
2. JWT 鉴权与登录态缓存
3. 视频发布、详情、上传
4. 点赞模块
5. 评论模块
6. 关注模块
7. Feed 最新流、点赞数流、热榜流
8. Redis 缓存、互斥锁、防击穿
9. RabbitMQ 业务异步化
10. 热度更新异步化
11. Docker Compose 容器化部署

### 后续可优化

1. 评论发布幂等增强
2. 更清晰的 Worker 错误分层与重试策略
3. 更完整的热度链路治理
4. 更完善的容器化启动重试与健康检查
