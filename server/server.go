package server

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

var (
	notifySignal   = signal.Notify
	serverShutdown = func(server *http.Server, ctx context.Context) error {
		return server.Shutdown(ctx)
	}
)

// Start starts the http server. http server 启动函数
// 
// @param mux *mux.Router: gorilla/mux 包提供的一个路由器类型的指针
// @param address string: 本机的 ip 地址
// @param cert string: cert 参数表示 SSL/TLS 证书文件的路径
// @param key string: 私钥文件的路径
// @return error: 返回错误码
func Start(mux *mux.Router, address, cert, key string) error {
	// 服务开启
	server, shutdown := startServer(mux, address, cert, key)
	// 因中断信号关闭服务的处理
	shutdownOnInterruptSignal(server, 2*time.Second, shutdown)
	// 报错处理，等待 server 关闭
	return waitForServerToClose(shutdown)
}

// 开启服务
//
// @param mux *mux.Router: gorilla/mux 包提供的一个路由器类型的指针
// @param address string: 本机的 ip 地址
// @param cert string: cert 参数表示 SSL/TLS 证书文件的路径
// @param key string: 私钥文件的路径
// @return *http.Server: 一个指向 http.Server 类型的指针。
// @return chan error: 用于传递 error 类型的通道。
func startServer(mux *mux.Router, address, cert, key string) (*http.Server, chan error) {
	// 根据 ip 和路由器类，创建一个 http.Server 实例
	srv := &http.Server{
		Addr:    address,
		Handler: mux,
	}

	// 创建传递 error 信息的通道
	shutdown := make(chan error)
	// 启动一个 goroutine 来运行 listenAndServe 函数。
	go func() {
		// 如果得到错误信息，传递到错误通道
		err := listenAndServe(srv, address, cert, key)
		shutdown <- err
	}()
	return srv, shutdown
}

// 
func listenAndServe(srv *http.Server, address, cert, key string) error {
	var err error
	var listener net.Listener

	// 根据地址前缀（unix: 或 tcp）创建一个网络监听器。
	if strings.HasPrefix(address, "unix:") {
		listener, err = net.Listen("unix", strings.TrimPrefix(address, "unix:"))
	} else {
		listener, err = net.Listen("tcp", address)
	}
	if err != nil {
		return err
	}

	// 如果提供了证书和密钥，将启动 HTTPS 服务器，否则启动 HTTP 服务器。
	if cert != "" || key != "" {
		log.Info().Str("addr", address).Msg("Start HTTP with tls")
		return srv.ServeTLS(listener, cert, key)
	} else {
		log.Info().Str("addr", address).Msg("Start HTTP")
		return srv.Serve(listener)
	}
}

// 接受中断信号的处理函数
func shutdownOnInterruptSignal(server *http.Server, timeout time.Duration, shutdown chan<- error) {
	interrupt := make(chan os.Signal, 1)
	notifySignal(interrupt, os.Interrupt)

	go func() {
		<-interrupt
		log.Info().Msg("Received interrupt. Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := serverShutdown(server, ctx); err != nil {
			shutdown <- err
		}
	}()
}

func waitForServerToClose(shutdown <-chan error) error {
	err := <-shutdown
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
