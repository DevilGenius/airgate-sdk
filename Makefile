# AirGate SDK Makefile

GO := GOTOOLCHAIN=local go

# Protobuf 工具版本（锁定）
PROTOC_VERSION     := 29.5
PROTOC_GEN_GO_VER  := v1.36.11
PROTOC_GEN_GRPC_VER := v1.6.0

# 本地工具目录
TOOLS_DIR   := $(CURDIR)/.tools
PROTOC_BIN  := $(TOOLS_DIR)/bin/protoc

.PHONY: help ci pre-commit lint fmt test vet build theme-package-build theme-package-check theme theme-check proto proto-check proto-tools clean setup-hooks

help: ## 显示帮助信息
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ===================== 质量检查 =====================

ci: lint test vet build proto-check theme-package-check theme-check ## 本地运行与 CI 完全一致的检查

pre-commit: lint vet build ## pre-commit hook 调用（跳过耗时的 race 测试）

lint: ## 代码检查（需要安装 golangci-lint）
	@if ! command -v golangci-lint > /dev/null 2>&1; then \
		echo "错误: 未安装 golangci-lint，请执行: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"; \
		exit 1; \
	fi
	golangci-lint run ./...
	@echo "代码检查通过"

fmt: ## 格式化代码
	@if command -v goimports > /dev/null 2>&1; then \
		goimports -w -local github.com/DouDOU-start .; \
	else \
		$(GO) fmt ./...; \
	fi
	@echo "代码格式化完成"

test: ## 运行测试（race 检测 + 覆盖率）
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out
	@echo "测试完成"

vet: ## 静态分析
	$(GO) vet ./...

build: ## 编译检查
	$(GO) build ./...

# ===================== 主题包 =====================

theme-package-build: ## 构建 AirGate 主题包
	cd theme && pnpm build

theme-package-check: theme-package-build ## 校验 AirGate 主题包构建产物无漂移
	@git diff --exit-code -- theme/dist
	@echo "AirGate 主题包构建产物无漂移"

theme: theme-package-build ## 构建 AirGate 主题包并生成 DevServer 用 theme.css
	node --input-type=module -e "import{generateThemeCSS}from'./theme/dist/css.js';process.stdout.write(generateThemeCSS() + '\n')" > devkit/devserver/static/theme.css
	@echo "theme.css 已生成"

# ===================== 代码生成 =====================

proto-tools: ## 安装指定版本的 protoc 和 Go 插件
	@if [ -x "$(PROTOC_BIN)" ] && $(PROTOC_BIN) --version | grep -q "$(PROTOC_VERSION)"; then \
		echo "protoc v$(PROTOC_VERSION) 已就绪"; \
	else \
		echo "安装 protoc v$(PROTOC_VERSION)..."; \
		mkdir -p $(TOOLS_DIR); \
		OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
		ARCH=$$(uname -m); \
		case $$ARCH in x86_64) ARCH=x86_64;; aarch64|arm64) ARCH=aarch_64;; esac; \
		case $$OS in darwin) OS=osx;; esac; \
		curl -sSL "https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-$$OS-$$ARCH.zip" \
			-o $(TOOLS_DIR)/protoc.zip; \
		unzip -oq $(TOOLS_DIR)/protoc.zip -d $(TOOLS_DIR); \
		rm -f $(TOOLS_DIR)/protoc.zip $(TOOLS_DIR)/readme.txt; \
		echo "protoc v$(PROTOC_VERSION) 安装完成"; \
	fi
	@GOBIN=$(TOOLS_DIR)/bin $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VER)
	@GOBIN=$(TOOLS_DIR)/bin $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GRPC_VER)
	@echo "protoc-gen-go / protoc-gen-go-grpc 已就绪"

proto: proto-tools ## 重新生成 protobuf 代码
	@cd protocol/proto && PATH=$(TOOLS_DIR)/bin:$$PATH $(PROTOC_BIN) \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		plugin.proto
	@echo "Proto 代码生成完成"

proto-check: proto ## 校验 protobuf 生成代码无漂移
	@git diff --exit-code -- protocol/proto/plugin.pb.go protocol/proto/plugin_grpc.pb.go protocol/proto/plugin.proto
	@echo "Proto 代码无漂移"

theme-check: theme ## 校验 DevServer 主题 CSS 无漂移
	@git diff --exit-code -- devkit/devserver/static/theme.css
	@echo "theme.css 无漂移"

# ===================== Git Hooks =====================

setup-hooks: ## 安装 Git pre-commit hook
	@echo '#!/bin/sh' > .git/hooks/pre-commit
	@echo 'make pre-commit' >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "pre-commit hook 已安装"

# ===================== 清理 =====================

clean: ## 清理构建产物
	@$(GO) clean ./...
