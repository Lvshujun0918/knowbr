# 知行桥——开发者常用命令
# 只依赖本机已安装的 Go / Node.js / pnpm（无需 Docker）。
# 目标：把「提交前必须通过什么」固定成一条命令，便于自测与比赛评审复现。

SHELL := /bin/bash
SERVER_DIR := server
WEB_DIR := web

.PHONY: help check check-server check-web test vet build web-install web-test web-build smoke run dev docker-up clean

SMOKE_URL ?= http://127.0.0.1:8080

help:
	@echo "make check        # 提交前质量门：go vet + go test + go build + 前端单测 + 类型检查与打包"
	@echo "make check-server # 仅后端：vet + test + build"
	@echo "make check-web    # 仅前端：单元测试 + vue-tsc --noEmit + vite build"
	@echo "make smoke        # 界面冒烟：用真实浏览器点击一遍核心流程（需先启动服务，另需 Bun ≥1.4）"
	@echo "make run          # 单端口演示：构建前端后由后端托管，访问 http://127.0.0.1:8080"
	@echo "make dev          # 开发模式：后端 8080 + Vite 5173（两个前台进程，建议分终端运行）"
	@echo "make docker-up    # 用 Docker Compose 启动（自动构建前后端）"

check: check-server check-web
	@echo "✅ 质量门全部通过"

check-server: vet test build

vet:
	cd $(SERVER_DIR) && go vet ./...

test:
	cd $(SERVER_DIR) && go test ./...

build:
	cd $(SERVER_DIR) && go build -o /dev/null .

check-web: web-install web-test web-build

web-install:
	cd $(WEB_DIR) && pnpm install --frozen-lockfile

web-test:
	cd $(WEB_DIR) && pnpm test

web-build:
	cd $(WEB_DIR) && pnpm build

run: web-build
	cd $(SERVER_DIR) && go run .

# 界面冒烟测试：需要一个已启动的服务端（默认 127.0.0.1:8080）与 Bun ≥1.4。
# 它会新建一条演示记录并删除它，请对着演示实例跑。
smoke:
	@echo "前置：另开终端运行 cd $(SERVER_DIR) && go run ."
	bun scripts/ui-smoke.mjs $(SMOKE_URL)

dev:
	@echo "请分两个终端运行："
	@echo "  cd $(SERVER_DIR) && go run ."
	@echo "  cd $(WEB_DIR) && pnpm dev"

docker-up:
	docker compose up -d --build --wait

clean:
	rm -f $(SERVER_DIR)/server
