package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/WuCaiCaiCai/SubS/internal/secrets"
	"github.com/WuCaiCaiCai/SubS/internal/server"
	"github.com/WuCaiCaiCai/SubS/internal/store"
)

// version is stamped at build time: -ldflags "-X main.version=v1.2.3".
var version = "dev"

const usage = `subs — 私有订阅管理与分发工具

用法:
  subs [serve] [--data-dir DIR] [--listen HOST:PORT] [--init-token TOKEN]
  subs reset-password [--data-dir DIR] [--password PASS]
  subs backup -o FILE [--data-dir DIR] [-p PASS]
  subs restore FILE [--data-dir DIR] [-p PASS]
  subs status [--data-dir DIR]
  subs uninstall [--purge] [--yes] [--data-dir DIR]
  subs version

子命令说明:
  reset-password  重置管理员密码并使全部会话失效
  backup          在无网页时导出 .sbox 加密备份
  restore         从 .sbox 备份恢复（恢复前自动留存当前库快照）
  status          查看实例与数据统计
  uninstall       停止并移除服务与二进制（--purge 同时删除数据）
  version         打印版本号
`

func main() {
	args := os.Args[1:]
	command := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command = args[0]
		args = args[1:]
	}
	var err error
	switch command {
	case "serve":
		err = runServe(args)
	case "reset-password":
		err = cmdResetPassword(args)
	case "backup":
		err = cmdBackup(args)
	case "restore":
		err = cmdRestore(args)
	case "status":
		err = cmdStatus(args)
	case "uninstall":
		err = cmdUninstall(args)
	case "version":
		fmt.Println(version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		err = fmt.Errorf("未知命令 %q\n\n%s", command, usage)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	dataDir := fs.String("data-dir", env("SUBS_DATA_DIR", "./data"), "数据目录")
	addr := fs.String("listen", env("SUBS_LISTEN", "127.0.0.1:8787"), "HTTP 监听地址")
	initToken := fs.String("init-token", os.Getenv("SUBS_INIT_TOKEN"), "一次性初始化令牌")
	fs.Usage = func() { fmt.Print(usage) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := os.MkdirAll(*dataDir, 0o700); err != nil {
		return fmt.Errorf("创建数据目录失败：%w", err)
	}
	// 安装器把密钥落在 0600 文件里而不写进 systemd unit，避免密钥出现在可被
	// 其他本地用户读取的 unit 文件中；单元与文件哪个先就绪都能正常工作。
	if *initToken == "" {
		if b, readErr := os.ReadFile(filepath.Join(*dataDir, "init-token")); readErr == nil {
			*initToken = strings.TrimSpace(string(b))
		}
	}
	dbPath := filepath.Join(*dataDir, "subs.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		f, e := os.OpenFile(dbPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
		if e != nil {
			return fmt.Errorf("创建数据库失败：%w", e)
		}
		_ = f.Close()
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("打开数据库失败：%w", err)
	}
	defer db.Close()
	box, err := secrets.LoadOrCreateBox(*dataDir)
	if err != nil {
		return fmt.Errorf("加载密钥失败：%w", err)
	}
	srv := &server.Server{Store: db, InitToken: *initToken, Box: box, DataDir: *dataDir, DBPath: dbPath, Version: version}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go srv.StartAutoRefresh(ctx)

	httpSrv := &http.Server{Addr: *addr, Handler: srv.Router()}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()

	log.Printf("subs %s 监听 %s（数据目录 %s）", version, *addr, *dataDir)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("HTTP 服务异常退出：%w", err)
	}
	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
