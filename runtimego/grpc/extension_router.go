package grpc

import (
	"net/http"
	"strings"
	"sync"

	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

// extensionRouter 实现 sdk.RouteRegistrar，用于在 gRPC 模式下将 HTTP 请求分发到注册的处理函数
type extensionRouter struct {
	mu       sync.RWMutex
	handlers map[string]http.HandlerFunc // "METHOD /path" → handler
	prefix   string
}

func newExtensionRouter() *extensionRouter {
	return &extensionRouter{
		handlers: make(map[string]http.HandlerFunc),
	}
}

func (r *extensionRouter) Handle(method, path string, handler http.HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fullPath := r.prefix + path
	key := strings.ToUpper(method) + " " + fullPath
	r.handlers[key] = handler
}

func (r *extensionRouter) Group(prefix string) sdk.RouteRegistrar {
	return &extensionRouter{
		handlers: r.handlers,
		prefix:   r.prefix + prefix,
	}
}

// match 查找匹配的处理函数
func (r *extensionRouter) match(method, path string) http.HandlerFunc {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := strings.ToUpper(method) + " " + path
	if handler := r.handlers[key]; handler != nil {
		return handler
	}

	// 支持以 "/" 结尾的前缀路由，用于插件处理动态子路径
	// 例如注册 GET /openclaw/tenants/ 后，可匹配 /openclaw/tenants/123
	methodPrefix := strings.ToUpper(method) + " "
	var (
		bestHandler http.HandlerFunc
		bestLen     int
	)
	for routeKey, handler := range r.handlers {
		if !strings.HasPrefix(routeKey, methodPrefix) {
			continue
		}
		routePath := strings.TrimPrefix(routeKey, methodPrefix)
		if !strings.HasSuffix(routePath, "/") {
			continue
		}
		if !strings.HasPrefix(path, routePath) {
			continue
		}
		if len(routePath) > bestLen {
			bestLen = len(routePath)
			bestHandler = handler
		}
	}
	return bestHandler
}
