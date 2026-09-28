#!/bin/bash
# 聊天扩展冒烟测试: 用"官方下发的 roomId"收发消息, 并验证房间隔离。
# 需要服务端已运行在 127.0.0.1:8099
set -u
BASE=http://127.0.0.1:8099

# 模拟官方服务端下发的 roomId 与 sessionNonce
ROOM_A="r_official_abc123"
ROOM_B="r_official_xyz789"
NONCE_A="official-nonce-alice"
NONCE_B="official-nonce-bob"

echo "== 1. 健康检查(应标明 chat-extension) =="
curl -s "$BASE/healthz"; echo

echo "== 2. 房间生命周期接口应不存在 =="
printf 'join:   '; curl -s -o /dev/null -w '%{http_code}\n' -X POST "$BASE/v2/watch-together/join" -d '{}'
printf 'report: '; curl -s -o /dev/null -w '%{http_code}\n' -X POST "$BASE/v2/watch-together/rooms/$ROOM_A/report" -d '{}'
printf 'leave:  '; curl -s -o /dev/null -w '%{http_code}\n' -X POST "$BASE/v2/watch-together/rooms/$ROOM_A/leave" -d '{}'

echo "== 3. Alice 在官方房间 A 发言 =="
curl -s -X POST "$BASE/v2/watch-together/rooms/$ROOM_A/chat" \
  -H 'Content-Type: application/json' \
  -H 'X-Ani-User-Id: u1' -H 'X-Ani-Nickname: Alice' -H 'X-Ani-Avatar: https://example.com/alice.png' \
  -d "{\"sessionNonce\":\"$NONCE_A\",\"content\":\"大家好,这是A房间\"}"; echo

echo "== 4. Bob 在官方房间 B 发言 =="
curl -s -X POST "$BASE/v2/watch-together/rooms/$ROOM_B/chat" \
  -H 'Content-Type: application/json' \
  -H 'X-Ani-User-Id: u2' -H 'X-Ani-Nickname: Bob' -H 'X-Ani-Avatar: https://example.com/bob.png' \
  -d "{\"sessionNonce\":\"$NONCE_B\",\"content\":\"B房间消息\"}"; echo

echo "== 5. 无 sessionNonce 应被拒绝 =="
printf 'empty nonce: '; curl -s -o /dev/null -w '%{http_code}\n' -X POST \
  "$BASE/v2/watch-together/rooms/$ROOM_A/chat" -H 'Content-Type: application/json' \
  -d '{"sessionNonce":"","content":"let me in"}'

echo "== 6. 房间 A 历史(不应含 B 的消息) =="
curl -s "$BASE/v2/watch-together/rooms/$ROOM_A/chat"; echo

echo "== 7. 房间 B 历史(不应含 A 的消息) =="
curl -s "$BASE/v2/watch-together/rooms/$ROOM_B/chat"; echo

echo "== 8. SSE 实时推送(后台 6 秒) =="
(timeout 6 curl -sN "$BASE/v2/watch-together/rooms/$ROOM_A/events" > /tmp/chat_sse.txt) &
sleep 1
curl -s -X POST "$BASE/v2/watch-together/rooms/$ROOM_A/chat" \
  -H 'Content-Type: application/json' \
  -H 'X-Ani-User-Id: u1' -H 'X-Ani-Nickname: Alice' \
  -d "{\"sessionNonce\":\"$NONCE_A\",\"content\":\"SSE 测试消息\"}" > /dev/null
wait
echo "--- SSE 收到 ---"
cat /tmp/chat_sse.txt