#!/bin/bash
# 端到端冒烟:两个用户加入同一房间 -> 各自开 SSE -> A 发言 -> 验证 B 收到
set -u
BASE=http://127.0.0.1:8099

echo "== 1. Alice 创建房间 =="
A=$(curl -s -X POST "$BASE/v2/watch-together/join?userId=alice&nickname=Alice" \
  -H 'Content-Type: application/json' \
  -d '{"roomName":"smoke","password":"pw","following":true}')
echo "$A"
ROOM=$(echo "$A" | python3 -c 'import sys,json;print(json.load(sys.stdin)["roomId"])')
NA=$(echo "$A" | python3 -c 'import sys,json;print(json.load(sys.stdin)["sessionNonce"])')

echo "== 2. Bob 加入同一房间 =="
B=$(curl -s -X POST "$BASE/v2/watch-together/join?userId=bob&nickname=Bob" \
  -H 'Content-Type: application/json' \
  -d '{"roomName":"smoke","password":"pw","following":true}')
echo "$B"
NB=$(echo "$B" | python3 -c 'import sys,json;print(json.load(sys.stdin)["sessionNonce"])')

echo "== 3. Bob 开启 SSE(后台 6 秒) =="
(timeout 6 curl -sN "$BASE/v2/watch-together/rooms/$ROOM/events?sessionNonce=$NB" > /tmp/bob_sse.txt) &
sleep 1

echo "== 4. Alice 发言 =="
curl -s -X POST "$BASE/v2/watch-together/rooms/$ROOM/chat" \
  -H 'Content-Type: application/json' \
  -d "{\"sessionNonce\":\"$NA\",\"content\":\"大家好,这是一起看测试\"}"
echo

echo "== 5. Alice 上报播放状态(房主) =="
curl -s -X POST "$BASE/v2/watch-together/rooms/$ROOM/report" \
  -H 'Content-Type: application/json' \
  -d "{\"sessionNonce\":\"$NA\",\"memberState\":\"WATCHING\",\"following\":true,\"watching\":{\"subjectId\":1,\"episodeId\":11,\"subjectName\":\"测试番剧\",\"episodeSort\":\"1\",\"episodeName\":\"第一话\",\"positionMillis\":5000,\"positionAtMillis\":5000,\"durationMillis\":100000,\"paused\":false,\"buffering\":false,\"loading\":false,\"playbackRate\":1.0}}" \
  | head -c 400
echo

wait

echo "== 6. Bob 收到的 SSE 内容 =="
cat /tmp/bob_sse.txt
echo "== 7. 房间聊天历史 =="
curl -s "$BASE/v2/watch-together/rooms/$ROOM/chat"