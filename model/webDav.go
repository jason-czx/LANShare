package model

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/net/webdav"
)

// WebDAVService 提供一个简单的本地 WebDAV 服务器，可以在局域网中被访问。
// 它会将给定目录作为根目录暴露出来，并提供 Start/Stop 方法供 Wails 调用。
type WebDAVService struct {
	mu       sync.Mutex
	srv      *http.Server
	listener net.Listener
	root     string
	running  bool
}

// NewWebDAVService 创建一个新的 WebDAVService。root 如果为空，会使用当前工作目录的 "shared" 子目录。
func NewWebDAVService(root string) *WebDAVService {
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		root = filepath.Join(cwd, "shared")
	}
	// 确保目录存在
	if err := os.MkdirAll(root, 0755); err != nil {
		log.Printf("failed to create webdav root: %v", err)
	}

	return &WebDAVService{root: root}
}

// Start 启动 WebDAV 服务，监听在指定端口（port 0 表示自动分配）。
// 返回一个可访问的地址（host:port），或错误。
func (w *WebDAVService) Start(ctx context.Context, port int) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		return "", fmt.Errorf("webdav already running")
	}

	handler := &webdav.Handler{
		Prefix:     "/",
		FileSystem: webdav.Dir(w.root),
		LockSystem: webdav.NewMemLS(),
	}

	mux := http.NewServeMux()
	mux.Handle("/", handler)

	srv := &http.Server{
		Handler: mux,
		// 设置合理的超时
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 监听所有接口，方便局域网访问
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", err
	}

	w.srv = srv
	w.listener = ln
	w.running = true

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("webdav server error: %v", err)
		}
		// ensure state reset
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
	}()

	host := localIPv4()
	if host == "" {
		host = "127.0.0.1"
	}
	actualPort := ln.Addr().(*net.TCPAddr).Port

	return fmt.Sprintf("%s:%d", host, actualPort), nil
}

// Stop 优雅关闭 WebDAV 服务。
func (w *WebDAVService) Stop(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return nil
	}
	// 先关闭 listener，阻止新连接
	if w.listener != nil {
		_ = w.listener.Close()
	}
	if w.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := w.srv.Shutdown(ctx); err != nil {
			return err
		}
	}
	w.running = false
	return nil
}

// IsRunning 返回服务是否在运行
func (w *WebDAVService) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// Root 返回当前共享根目录
func (w *WebDAVService) Root() string {
	return w.root
}

// localIPv4 尝试返回局域网 IPv4 地址（非 127.0.0.1、非回环、非链路本地）。
func localIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		// 忽略 down 或 loopback
		if iface.Flags&(net.FlagUp|net.FlagLoopback) != net.FlagUp {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not ipv4
			}
			// 排除链路本地和私有以外的地址不是必要的，但我们返回第一个可用 IPv4
			return ip.String()
		}
	}
	return ""
}
