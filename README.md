# GoTik

> 项目介绍：`GoTik` 是一个基于 Go 从零实现的短视频后端项目，围绕账号、视频、点赞、评论、关注与 Feed 主链路展开，并逐步接入 `MySQL + Redis + RabbitMQ`，重点实践缓存优化、热度排序、异步解耦与分页稳定性设计。  


## 技术栈

| 维度 | 组件/工具 | 说明 |
| --- | --- | --- |
| 开发语言 | Go | 后端核心业务实现，包含 HTTP 服务与 Worker |
| Web 框架 | Gin | 路由注册、请求绑定、JSON 响应、中间件接入 |
| ORM | GORM | 模型定义、CRUD、事务与启动迁移 |
| 持久化 | MySQL | 存储账号、视频、点赞、评论、关注关系及基础热度字段 |
| 缓存/排序 | Redis | Token 缓存、视频详情缓存、匿名 Feed 缓存、热度分钟桶与热榜读取 |
| 消息队列 | RabbitMQ | 点赞、评论、关注与热度更新事件驱动、主服务与 Worker 解耦 |
| 鉴权 | JWT | 登录签发 token，支持 Redis 优先、MySQL 兜底、自愈式校验 |
| 文件存储 | Local Disk | 视频和封面上传到本地目录，便于本地联调 |



## 核心模块

### 1. 账号模块

已完成能力：

1. 注册
2. 登录
3. 按 ID 查询用户
4. 获取当前登录用户信息
5. 退出登录

设计要点：

1. 登录成功后签发 JWT
2. Token 同时存储在 MySQL 与 Redis 中
3. 鉴权中间件采用 Redis 优先、MySQL 兜底、自愈回填

### 2. 视频模块

已完成能力：

1. 发布视频记录
2. 按作者查询视频列表
3. 查询视频详情
4. 上传视频文件
5. 上传封面文件

设计要点：

1. `video/getDetail` 做了详情缓存
2. 热点详情缓存支持互斥锁防击穿
3. 视频详情中的 `likes_count` 与 `popularity` 会随互动行为变化

### 3. 点赞模块

已完成能力：

1. 点赞
2. 取消点赞
3. 查询是否已点赞
4. 查询当前用户点赞过的视频

设计要点：

1. 点赞关系单独建表
2. 基于唯一索引限制重复点赞
3. 点赞/取消点赞已异步化
4. MQ 不可用时保留同步 fallback 直写

### 4. 评论模块

已完成能力：

1. 发布评论
2. 删除评论
3. 查询某视频下全部评论

设计要点：

1. 评论发布和删除已异步化
2. 发布评论会影响热度，当前规则为 `+2`
3. 当前版本删除评论不回滚热度，属于阶段性取舍

### 5. 关注模块

已完成能力：

1. 关注
2. 取关
3. 查询粉丝列表
4. 查询关注列表

设计要点：

1. 关注关系单独建表
2. `follow/unfollow` 已异步化
3. `follow` 消费端具备幂等思路，避免重复创建关注关系

### 6. Feed 模块

已完成能力：

1. 按发布时间倒序拉取最新视频流：`/feed/listLatest`
2. 按点赞数倒序分页：`/feed/listLikesCount`
3. 按热度排序读取：`/feed/listByPopularity`

设计要点：

1. 匿名 `listLatest` 支持短 TTL 缓存
2. `listLikesCount` 使用 `likes_count + id` 复合游标分页
3. 热榜支持分钟桶、快照读取与 `as_of + offset` 思路

## Redis 设计

### 当前使用场景

1. `account:<id>`：当前账号 token 缓存
2. `video:detail:id=<id>`：视频详情缓存
3. `feed:listLatest:limit=<limit>:before=<before>`：匿名 Feed 缓存
4. `lock:video:detail:id=<id>`：详情缓存互斥锁
5. `lock:feed:listLatest:...`：匿名 Feed 互斥锁
6. `hot:video:1m:<yyyyMMddHHmm>`：热度分钟桶
7. `hot:video:merge:1m:<asOf>`：热榜快照

### 已完成优化

1. Token 缓存：Redis 优先、MySQL 兜底、自愈回填
2. 视频详情缓存：Cache Aside + 写后删除缓存
3. 匿名 Feed 缓存：短 TTL 分页缓存
4. 缓存击穿保护：热点详情与匿名 Feed 互斥锁防击穿
5. 热榜排序：基于 ZSET 和分钟桶维护实时热度

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

1. 主服务负责前置校验与发布事件
2. Worker 负责消费消息并完成最终执行
3. 热度更新作为独立副作用事件流存在
4. MQ 不可用时按职责分别做本地 fallback

### 当前取舍

1. 业务写库与热度更新进一步解耦
2. Redis 热度作为主排序来源
3. MySQL `popularity` 字段作为兜底基础
4. 当前接受一定的最终一致性窗口

## 项目亮点

### 1. Redis 优先、MySQL 兜底、自愈式鉴权

登录成功后会把当前有效 token 写入 Redis，鉴权时优先读缓存，未命中再回源数据库，并在成功后回填 Redis。这样既减少了鉴权对 MySQL 的压力，也保留了 Redis 不可用时的降级能力。

### 2. 热点缓存击穿保护

`video/getDetail` 和匿名 `feed/listLatest` 都实现了基于 Redis `SETNX` 的互斥锁方案。缓存 miss 时并不是所有请求一起回源数据库，而是只让一个请求负责重建缓存，其余请求等待回填或走兜底读取。

### 3. 热榜不仅是 ZSET，而是分钟桶 + 快照分页思路

项目当前不是只维护一个简单总榜，而是使用分钟桶记录热度增量，并通过 `as_of + offset` 的快照分页思路保证热榜翻页更稳定，避免动态热榜在分页过程中出现重复或漏数据。

### 4. RabbitMQ 驱动的业务异步化

点赞、评论、关注都已改造成“主服务发消息 + Worker 最终执行”的模式，主链路响应更轻，业务模块边界更清晰，也便于后续继续扩展更多消费者和副作用处理。

### 5. 热度更新作为独立副作用事件流

点赞和评论不仅会触发业务写库，还会触发热度更新事件。项目当前把热度更新拆成 `PopularityMQ + PopularityWorker`，让业务主写和 Redis 实时热度更新进一步解耦，体现了对写扩散、事件驱动和最终一致性的理解。



## 运行方式

### 1. 配置依赖

当前项目默认依赖：

1. MySQL
2. Redis
3. RabbitMQ

请先在 [configs/config.yaml](D:/Trabajar/GoTik/configs/config.yaml) 中确认：

1. `database`
2. `redis`
3. `rabbitmq`

相关配置正确。

### 2. 启动 HTTP 主服务

```bash
go run ./cmd
```

### 3. 启动 Worker

```bash
go run ./cmd/worker
```

### 4. 当前联调方式

当前完整异步链路联调时，需要同时启动：

1. MySQL
2. Redis
3. RabbitMQ
4. HTTP 主服务
5. Worker 进程

## 当前已完成进度

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

### 未来可优化

1. 评论发布幂等增强
2. 更清晰的 worker 错误分层
3. 更完整的热度链路治理


