# 一起看服务端 (Watch Together Server)

Animeko / Ani「一起看」功能的**自托管服务端**，用 Go 实现。

官方只开源了客户端协议，**没有开源服务端**。本项目按客户端
`client/src/commonMain/gen/.../WatchTogetherAniApi` 所约定的
`/v2/watch-together` 协议从零实现，可直接对接官方客户端，并额外提供房间聊天能力。

## 特性

- 完整实现官方协议：`join` / `report` / `leave` / `events`(SSE)
- **房间隔离**：每个房间的成员列表、播放状态、聊天记录互相不可见（含测试验证）
- **房间聊天**：`GET`/`POST .../chat`，并通过 SSE `chat` 事件实时下发
- 只有房主上报的播放状态会成为房间 `playback`（跟随者上报不影响他人）
- 非房间成员不能发言
- 自动清理空房间与超时成员（60s 标记断开，5min 移除）

## 运行

```bash
go build -o wts .
./wts -addr :8080
```

健康检查：

```bash
curl http://localhost:8080/healthz
```

## 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/v2/watch-together/join` | 加入房间，不存在则创建 |
| POST | `/v2/watch-together/rooms/{roomId}/report` | 上报成员/播放状态 |
| POST | `/v2/watch-together/rooms/{roomId}/leave` | 离开房间 |
| GET  | `/v2/watch-together/rooms/{roomId}/events` | SSE 事件流（snapshot / chat / bye） |
| GET  | `/v2/watch-together/rooms/{roomId}/chat` | 聊天历史 |
| POST | `/v2/watch-together/rooms/{roomId}/chat` | 发送消息 |
| GET  | `/healthz` | 健康检查 |

## 身份

服务端不接入 Bangumi 账号体系，按以下优先级识别用户：

1. 显式 `userId` / `nickname`（query 或 `X-Ani-User-Id` / `X-Ani-Nickname` 头）
2. `Authorization: Bearer <token>` —— 同一 token 视为同一用户
3. 兜底：按 IP + User-Agent 生成稳定匿名 ID

生产部署如需真实账号，请在 `api/identity.go` 接入校验。

## 测试

```bash
go test ./...
```

覆盖：房间隔离、密码校验、非成员拒绝发言、房主播放同步、聊天广播、并发加入。

> 注：在 proot/受限环境下 `-race` 可能因 VMA 限制无法运行，普通 `go test` 正常。

## 客户端接入

在「一起看」Tab 右上角 Settings 中填写服务端地址，支持：

- `192.168.1.10:8080`（局域网，默认 http）
- `example.com`
- `https://example.com`

留空则使用官方服务端。

## 冒烟测试

```bash
bash smoke.sh   # 需服务端已运行在 127.0.0.1:8099
```