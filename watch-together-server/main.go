// Command watch-together-server 是 Animeko(Ani)「一起看」功能的自托管服务端。
//
// 实现与官方客户端兼容的 /v2/watch-together 协议,并扩展了房间聊天能力。
// 房间之间完全隔离:成员列表、播放状态、聊天记录互不可见。
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sakurajima336/animeko-watch-together/api"
	"github.com/sakurajima336/animeko-watch-together/room"
)

func main() {
	addr := flag.String("addr", ":8080", "监听地址,例如 :8080 或 127.0.0.1:9000")
	flag.Parse()

	rooms := room.NewManager()
	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.NewServer(rooms).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		// SSE 是长连接,不设置整体写超时。
		WriteTimeout: 0,
		IdleTimeout:  0,
	}

	go func() {
		log.Printf("一起看服务端启动: http://%s", *addr)
		log.Printf("健康检查: http://%s/healthz", *addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("监听失败: %v", err)
		}
	}()

	// 优雅退出。
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("正在关闭…")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("关闭异常: %v", err)
	}
	log.Println("已停止")
}
