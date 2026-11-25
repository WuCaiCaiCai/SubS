# SubS 构建入口
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
BIN := subs

.PHONY: help web build test vet fmt dev clean check

help:
	@echo "make web     构建前端（web/dist）"
	@echo "make build   构建单文件二进制（内含前端）"
	@echo "make test    运行全部 Go 测试"
	@echo "make check   fmt + vet + test + 前端类型检查"
	@echo "make dev     启动后端（数据目录 ./data，监听 127.0.0.1:8787）"
	@echo "make clean   清理构建产物"

web:
	cd web && npm ci && npm run build

build: web
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/subs

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w ./cmd ./internal *.go

check: vet test
	cd web && npm run build

dev:
	go run ./cmd/subs --data-dir ./data --listen 127.0.0.1:8787

clean:
	rm -f $(BIN)
	rm -rf web/dist
	mkdir -p web/dist && touch web/dist/.gitkeep
