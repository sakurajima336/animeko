# 一起看聊天扩展服务端 (Watch Together Chat Extension)

Animeko / Ani「一起看」的**聊天扩展**服务端，用 Go 实现。

## 定位

一起看的**房间、成员与播放同步始终由官方服务端提供**，本项目**不是**官方服务端的替代品。

本服务只做一件事：给官方房间**附加一个聊天室**。因此：

- 不接收客户端的 `join` / `report` / `leave`，房间不存在于此
- 不接受客户端自行创建房间，客户端只能使用**官方下发的 `roomId`**
- 未在官方房间内的 `roomId`，一律视为无效
- 客户端未配置扩展链接时，聊天功能整体禁用，不影响官方一起看

> 早期版本曾把自己当作可替换的一起看服务端（自带 join/report/leave），
> 这会导致房间与官方源脱节。现已移除该模式，仅保留聊天扩展。

## 房间绑定

房间身份完全来自官方服务端：

1. 客户端通过官方服务端加入房间，拿到 `roomId` 与 `sessionNonce`
2. 客户端把官方 `roomId` 透传给本扩展，扩展在该 id 下维护聊天室
3. `roomId` 失效（房间关闭/重建）后，旧 id 的聊天室不再有任何新消息

扩展**不做**房间名校验，也无法自行创建房间 —— 这是有意为之：房间的权威来源只有官方服务端。

## 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET  | `/v2/watch-together/rooms/{roomId}/chat` | 房间聊天历史 |
| POST | `/v2/watch-together/rooms/{roomId}/chat` | 发送消息 |
| GET  | `/v2/watch-together/rooms/{roomId}/events` | SSE 事件流（`chat` / `ping`） |
| GET  | `/healthz` | 健康检查 |

`{roomId}` 必须是官方服务端下发的 id。扩展不解析房间名，也不提供房间列表。

### 聊天历史

```http
GET /v2/watch-together/rooms/{roomId}/chat
```

```json
{
  "serverTime": 1790352808861,
  "messages": [
    {
      "id": "m_xxx",
      "roomId": "r_official_id",
      "userId": "u_xxx",
      "nickname": "Alice",
      "content": "hello",
      "sentAt": 1790352808861,
      "system": false
    }
  ]
}
```

### 发送消息

```http
POST /v2/watch-together/rooms/{roomId}/chat
Content-Type: application/json

{"sessionNonce": "<官方下发的 sessionNonce>", "content": "hello"}
```

`sender` 身份由 `sessionNonce` 对应到该扩展内的参与者记录。

### SSE

```http
GET /v2/watch-together/rooms/{roomId}/events
```

```text
:connected

event: chat
data: {"message":{"id":"m_xxx","content":"hello",...}}
```

另有 15 秒一次的 `:ping` 心跳。

## 运行

```bash
go build -o wts-chat-extension .
./wts-chat-extension -addr :8788
```

健康检查：

```bash
curl http://localhost:8788/healthz
```

## 隔离与安全

- **房间隔离**：每个官方 `roomId` 独立维护消息与参与者，互不可见（含测试验证）
- **非参与者不能发言**：未知 `sessionNonce` 一律拒绝
- 消息上限 500 字符，每房间保留最近 200 条
- 空房间与超时参与者自动清理

## 身份

本扩展不接入 Bangumi 账号体系，参与者由官方 `sessionNonce` 标识，昵称按以下优先级确定：

1. 显式 `userId` / `nickname`（query 或 `X-Ani-User-Id` / `X-Ani-Nickname` 头）
2. `Authorization: Bearer <token>`
3. 兜底：按 IP + User-Agent 生成稳定匿名 ID

生产部署如需真实账号，请在 `api/identity.go` 接入校验。

## 测试

```bash
go test ./...
```

覆盖：房间隔离、非参与者拒绝发言、消息广播、并发写入。

> 注：在 proot/受限环境下 `-race` 可能因 VMA 限制无法运行，普通 `go test` 正常。

## 客户端接入

「一起看」Tab 右上角 Settings →「聊天扩展」，填写扩展链接：

- `192.168.1.10:8080`（局域网，默认 http）
- `example.com`
- `https://example.com`

留空即关闭聊天扩展。

## 冒烟测试

```bash
bash smoke.sh   # 需服务端已运行在 127.0.0.1:8099
```