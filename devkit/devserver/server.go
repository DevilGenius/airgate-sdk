package devserver

import (
	"embed"
	"encoding/json"
	"flag"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

//go:embed static
var staticFiles embed.FS

// Config devserver 配置
type Config struct {
	Plugin         sdk.GatewayPlugin                             // 必填：网关插件实例
	Addr           string                                        // 监听地址，默认 ":18080"
	DataDir        string                                        // 数据目录，默认 "./devdata"
	ExtraRoutes    func(mux *http.ServeMux, store *AccountStore) // 插件自定义路由（如 OAuth）
	SchedulePolicy SchedulePolicy                                // 调度策略，默认 "none"（直连第一个账号）
}

// Run 启动 devserver（阻塞运行）
func Run(cfg Config) error {
	// 支持命令行覆盖
	addr := flag.String("addr", cfg.Addr, "监听地址")
	dataDir := flag.String("data", cfg.DataDir, "数据目录")
	logFile := flag.String("log", "", "日志文件路径")
	flag.Parse()

	if *addr == "" {
		*addr = ":18080"
	}
	if *dataDir == "" {
		*dataDir = "./devdata"
	}
	if *logFile == "" {
		*logFile = filepath.Join(*dataDir, "debug.log")
	}

	// 初始化日志：控制台 INFO + 文件 DEBUG
	if err := os.MkdirAll(filepath.Dir(*logFile), 0o755); err == nil {
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			consoleHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
			fileHandler := slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})
			slog.SetDefault(slog.New(&multiHandler{handlers: []slog.Handler{consoleHandler, fileHandler}}))
			log.SetOutput(io.MultiWriter(os.Stderr, f))
			log.Printf("日志文件: %s", *logFile)
		}
	}

	// 初始化插件
	ctx := &devPluginContext{logger: slog.Default()}
	if err := cfg.Plugin.Init(ctx); err != nil {
		return err
	}

	// 初始化账号存储
	store := NewAccountStore(filepath.Join(*dataDir, "accounts.json"))

	// 路由
	mux := http.NewServeMux()

	// 插件信息 API（合并 Info + Routes）
	mux.HandleFunc("/api/plugin/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		info := cfg.Plugin.Info()
		// PluginInfo 结构体不含 routes，用 map 合并输出
		data, _ := json.Marshal(info)
		var merged map[string]any
		_ = json.Unmarshal(data, &merged)
		merged["routes"] = cfg.Plugin.Routes()
		if err := json.NewEncoder(w).Encode(merged); err != nil {
			log.Printf("写入插件信息响应失败: %v", err)
		}
	})

	// 账号管理 API
	accountHandler := &AccountHandler{store: store}
	mux.Handle("/api/accounts/", accountHandler)
	mux.Handle("/api/accounts", accountHandler)

	// 账号连通性测试 API
	mux.HandleFunc("/api/accounts/test/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		idStr := strings.TrimPrefix(r.URL.Path, "/api/accounts/test/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
			return
		}
		account := store.Get(id)
		if account == nil {
			http.Error(w, `{"error":"account not found"}`, http.StatusNotFound)
			return
		}
		start := time.Now()
		validateErr := cfg.Plugin.ValidateAccount(r.Context(), account.Credentials)
		duration := time.Since(start)
		result := map[string]any{
			"id":       id,
			"duration": duration.Truncate(time.Millisecond).String(),
		}
		if validateErr != nil {
			result["ok"] = false
			result["error"] = validateErr.Error()
		} else {
			result["ok"] = true
		}
		if err := json.NewEncoder(w).Encode(result); err != nil {
			log.Printf("写入测试结果响应失败: %v", err)
		}
	})

	// 插件自定义路由
	if cfg.ExtraRoutes != nil {
		cfg.ExtraRoutes(mux, store)
	}

	// 初始化调度器
	scheduler := NewScheduler(store, cfg.SchedulePolicy)

	// 调度管理 API
	schedulerHandler := &SchedulerHandler{scheduler: scheduler, store: store}
	mux.Handle("/api/scheduler/", schedulerHandler)
	mux.Handle("/api/scheduler", schedulerHandler)

	// 从 plugin.Routes() 提取路径前缀注册代理
	proxy := &ProxyHandler{plugin: cfg.Plugin, store: store, scheduler: scheduler}
	prefixes := routePrefixes(cfg.Plugin.Routes())
	hasRootProxy := false
	for _, prefix := range prefixes {
		if prefix == "/" {
			hasRootProxy = true
			continue
		}
		mux.Handle(prefix, proxy)
	}

	// 插件前端资源（如果实现了 WebAssetsProvider）
	if wap, ok := cfg.Plugin.(sdk.WebAssetsProvider); ok {
		assets := wap.GetWebAssets()
		if len(assets) > 0 {
			mux.HandleFunc("/plugin-assets/", func(w http.ResponseWriter, r *http.Request) {
				name := strings.TrimPrefix(r.URL.Path, "/plugin-assets/")
				data, exists := assets[name]
				if !exists {
					http.NotFound(w, r)
					return
				}
				if strings.HasSuffix(name, ".js") {
					w.Header().Set("Content-Type", "application/javascript")
				} else if strings.HasSuffix(name, ".css") {
					w.Header().Set("Content-Type", "text/css")
				}
				if _, err := w.Write(data); err != nil {
					log.Printf("写入插件静态资源失败: %v", err)
				}
			})
		}
	}

	// 内嵌管理页面
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return err
	}
	staticHandler := http.FileServer(http.FS(staticFS))
	if hasRootProxy {
		mux.Handle("/", &rootRouteHandler{
			proxy:  proxy,
			static: staticHandler,
			routes: cfg.Plugin.Routes(),
		})
	} else {
		mux.Handle("/", staticHandler)
	}

	info := cfg.Plugin.Info()
	log.Printf("devserver 启动: http://localhost%s", *addr)
	log.Printf("插件: %s v%s", info.Name, info.Version)
	log.Printf("管理页面: http://localhost%s", *addr)
	log.Printf("调度策略: %s", scheduler.Policy())
	return http.ListenAndServe(*addr, mux)
}

// routePrefixes 从路由声明中提取不重复的路径前缀（如 /v1/）。
func routePrefixes(routes []sdk.RouteDefinition) []string {
	seen := make(map[string]bool)
	prefixes := make([]string, 0, len(routes))
	for _, r := range routes {
		prefix, ok := routePrefix(r.Path)
		if !ok {
			continue
		}
		if !seen[prefix] {
			seen[prefix] = true
			prefixes = append(prefixes, prefix)
		}
	}
	return prefixes
}

func routePrefix(routePath string) (string, bool) {
	routePath = strings.TrimSpace(routePath)
	if routePath == "" {
		return "", false
	}
	if !strings.HasPrefix(routePath, "/") {
		routePath = "/" + routePath
	}
	cleaned := path.Clean(routePath)
	if cleaned == "/" {
		return "/", true
	}
	parts := strings.SplitN(strings.TrimPrefix(cleaned, "/"), "/", 2)
	return "/" + parts[0] + "/", true
}

type rootRouteHandler struct {
	proxy  http.Handler
	static http.Handler
	routes []sdk.RouteDefinition
}

func (h *rootRouteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, route := range h.routes {
		prefix, ok := routePrefix(route.Path)
		if !ok || prefix != "/" {
			continue
		}
		if path.Clean(r.URL.Path) != "/" {
			continue
		}
		if !routeMethodMatches(route.Method, r.Method) {
			continue
		}
		h.proxy.ServeHTTP(w, r)
		return
	}
	h.static.ServeHTTP(w, r)
}

func routeMethodMatches(routeMethod, requestMethod string) bool {
	routeMethod = strings.TrimSpace(routeMethod)
	if routeMethod == "" {
		return true
	}
	return strings.EqualFold(routeMethod, requestMethod)
}
