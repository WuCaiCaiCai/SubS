package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wucai/subs/internal/backup"
	"github.com/wucai/subs/internal/secrets"
	"github.com/wucai/subs/internal/store"
)

// The CLI is the break-glass path: it runs as root or the subs user on the VPS
// and never prints secrets. Every subcommand opens the data directory directly
// (the HTTP server may be stopped).

// parseFlags allows flags and positionals in any order (Go's flag package stops
// at the first non-flag argument, which is unfriendly for `subs restore a.sbox -p pw`).
func parseFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			positional = append(positional, a)
			continue
		}
		flags = append(flags, a)
		if strings.Contains(a, "=") {
			continue
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			flags = append(flags, args[i+1])
			i++
		}
	}
	if err := fs.Parse(flags); err != nil {
		return nil, err
	}
	return positional, nil
}

func defaultDataDir() string {
	if v := os.Getenv("SUBS_DATA_DIR"); v != "" {
		return v
	}
	return "./data"
}

func openStore(dataDir string) (*store.Store, error) {
	dbPath := filepath.Join(dataDir, "subs.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("数据目录缺少 subs.db：%s", dataDir)
	}
	return store.Open(dbPath)
}

// cmdResetPassword resets the admin password and invalidates every session.
func cmdResetPassword(args []string) error {
	fs := flag.NewFlagSet("reset-password", flag.ExitOnError)
	dataDir := fs.String("data-dir", defaultDataDir(), "数据目录")
	password := fs.String("password", os.Getenv("SUBS_PASSWORD"), "新密码（省略则交互输入）")
	username := fs.String("username", "", "用户名（省略则保持当前用户名）")
	if err := fs.Parse(args); err != nil {
		return err
	}
	pw, err := resolvePassword(*password, "新密码")
	if err != nil {
		return err
	}
	if err := checkPasswordStrength(pw); err != nil {
		return err
	}
	hash, err := secrets.HashPassword(pw)
	if err != nil {
		return err
	}
	db, err := openStore(*dataDir)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx := context.Background()
	var exists int
	if err := db.DB().QueryRowContext(ctx, "SELECT count(*) FROM admin").Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return errors.New("实例尚未初始化，请打开网页完成初始化")
	}
	if _, err := db.DB().ExecContext(ctx,
		"UPDATE admin SET secret_password_hash=? WHERE id=1", hash); err != nil {
		return err
	}
	if res, err := db.DB().ExecContext(ctx, "DELETE FROM admin_session"); err != nil {
		return err
	} else if n, _ := res.RowsAffected(); n > 0 {
		fmt.Printf("已失效 %d 个会话\n", n)
	}
	if *username != "" {
		if _, err := db.DB().ExecContext(ctx,
			"UPDATE admin SET username=? WHERE id=1", *username); err != nil {
			return err
		}
	}
	fmt.Println("管理员密码已重置")
	return nil
}

func checkPasswordStrength(pw string) error {
	if len([]rune(pw)) < 12 {
		return errors.New("密码至少 12 个字符")
	}
	if strings.TrimSpace(pw) == "" {
		return errors.New("密码不能为空白")
	}
	return nil
}

func resolvePassword(given, label string) (string, error) {
	if given != "" {
		return given, nil
	}
	fmt.Fprintf(os.Stderr, "%s: ", label)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("读取%s失败", label)
	}
	pw := strings.TrimRight(line, "\r\n")
	if pw == "" {
		return "", fmt.Errorf("%s不能为空", label)
	}
	return pw, nil
}

// cmdBackup writes an .sbox archive without the web UI.
func cmdBackup(args []string) error {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	dataDir := fs.String("data-dir", defaultDataDir(), "数据目录")
	out := fs.String("o", "", "输出文件路径")
	password := fs.String("p", os.Getenv("SUBS_BACKUP_PASSWORD"), "备份密码")
	if err := fs.Parse(args); err != nil {
		return err
	}
	target := *out
	if target == "" {
		return errors.New("请用 -o 指定输出文件，例如 -o subs-backup.sbox")
	}
	pw, err := resolvePassword(*password, "备份密码")
	if err != nil {
		return err
	}
	if err := checkPasswordStrength(pw); err != nil {
		return err
	}
	db, err := openStore(*dataDir)
	if err != nil {
		return err
	}
	defer db.Close()

	tmpDir := filepath.Join(*dataDir, "backups")
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return err
	}
	snap := filepath.Join(tmpDir, fmt.Sprintf("snapshot-%d.db", time.Now().UnixNano()))
	if _, err := db.DB().ExecContext(context.Background(), "VACUUM INTO ?", snap); err != nil {
		return err
	}
	defer os.Remove(snap)
	dbBytes, err := os.ReadFile(snap)
	if err != nil {
		return err
	}
	meta, err := buildMetadata(*dataDir)
	if err != nil {
		return err
	}
	archive, err := backup.Build(map[string][]byte{"metadata.json": meta, "db.sqlite": dbBytes}, pw)
	if err != nil {
		return err
	}
	if err := os.WriteFile(target, archive, 0o600); err != nil {
		return err
	}
	sum := sha256.Sum256(archive)
	fmt.Printf("已写出 %s\n大小 %s\nsha256 %s\n", target, humanBytes(int64(len(archive))), hex.EncodeToString(sum[:]))
	return nil
}

func buildMetadata(dataDir string) ([]byte, error) {
	meta := map[string]any{
		"format":      "subs.sbox",
		"version":     1,
		"instance":    "cli",
		"createdAt":   time.Now().UTC().Format(time.RFC3339),
		"appVersion":  version,
		"dataDirName": filepath.Base(dataDir),
	}
	return json.MarshalIndent(meta, "", "  ")
}

// cmdRestore restores a backup over the live database, snapshotting the current
// database first so a bad restore can be undone by hand.
func cmdRestore(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	dataDir := fs.String("data-dir", defaultDataDir(), "数据目录")
	password := fs.String("p", os.Getenv("SUBS_BACKUP_PASSWORD"), "备份密码")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 1 {
		return errors.New("用法: subs restore <备份文件.sbox> [-p 密码]")
	}
	src := rest[0]
	pw, err := resolvePassword(*password, "备份密码")
	if err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	files, err := backup.Read(data, pw)
	if err != nil {
		return err
	}
	dbBytes := files["db.sqlite"]
	if len(dbBytes) == 0 {
		return errors.New("备份缺少数据库")
	}
	dbPath := filepath.Join(*dataDir, "subs.db")
	if _, err := os.Stat(dbPath); err == nil {
		pre := filepath.Join(*dataDir, fmt.Sprintf("pre-restore-%d.db", time.Now().Unix()))
		if err := copyFile(dbPath, pre); err != nil {
			return fmt.Errorf("保存恢复前快照失败：%w", err)
		}
		fmt.Printf("已保存恢复前快照 %s\n", pre)
	}
	tmp, err := os.CreateTemp(*dataDir, "restore-*.db")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(dbBytes); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	// Sanity check: the archive must contain a readable SubS database.
	check, err := store.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("备份数据库无法打开：%w", err)
	}
	_ = check.Close()
	return replaceDB(tmpPath, dbPath)
}

func replaceDB(tmpPath, dbPath string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(dbPath + suffix)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, dbPath); err != nil {
		return err
	}
	fmt.Println("恢复完成，请重启服务：systemctl restart subs")
	return nil
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o600)
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KiB", "MiB", "GiB"}
	v := float64(n)
	for _, u := range units {
		v /= unit
		if v < unit {
			return fmt.Sprintf("%.1f %s", v, u)
		}
	}
	return fmt.Sprintf("%.1f TiB", v/unit)
}

// cmdListen rewrites the stored listen host/port and prints a restart hint.
func cmdListen(args []string) error {
	fs := flag.NewFlagSet("listen", flag.ExitOnError)
	dataDir := fs.String("data-dir", defaultDataDir(), "数据目录")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(rest) < 2 {
		return errors.New("用法: subs listen <host> <port>")
	}
	host := rest[0]
	port, err := strconv.Atoi(rest[1])
	if err != nil || port < 1 || port > 65535 {
		return errors.New("端口无效")
	}
	db, err := openStore(*dataDir)
	if err != nil {
		return err
	}
	defer db.Close()
	res, err := db.DB().ExecContext(context.Background(),
		"UPDATE instance SET listen_host=?, listen_port=? WHERE id=1", host, port)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("实例尚未初始化")
	}
	fmt.Printf("监听地址已设置为 %s:%d\n请同步修改 systemd unit 的 --listen 参数并重启服务\n", host, port)
	return nil
}

// cmdStatus prints a health summary without exposing secrets.
func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	dataDir := fs.String("data-dir", defaultDataDir(), "数据目录")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := openStore(*dataDir)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx := context.Background()
	var name, host string
	var port int
	err = db.DB().QueryRowContext(ctx,
		"SELECT name,listen_host,listen_port FROM instance WHERE id=1").Scan(&name, &host, &port)
	if err != nil {
		return fmt.Errorf("读取实例信息失败：%w", err)
	}
	counts := []struct {
		label string
		query string
	}{
		{"机场订阅", "SELECT count(*) FROM airport"},
		{"自建服务器", "SELECT count(*) FROM server"},
		{"节点", "SELECT count(*) FROM node"},
		{"配置", "SELECT count(*) FROM profile"},
		{"模板", "SELECT count(*) FROM template"},
	}
	fmt.Printf("实例 %s  监听 %s:%d  版本 %s\n", name, host, port, version)
	for _, c := range counts {
		var n int
		if err := db.DB().QueryRowContext(ctx, c.query).Scan(&n); err != nil {
			continue
		}
		fmt.Printf("%-10s %d\n", c.label, n)
	}
	dbPath := filepath.Join(*dataDir, "subs.db")
	if st, err := os.Stat(dbPath); err == nil {
		fmt.Printf("数据库 %s（%s）\n", dbPath, humanBytes(st.Size()))
	}
	return nil
}

// cmdUninstall removes the systemd unit and binary, optionally purging data.
func cmdUninstall(args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	dataDir := fs.String("data-dir", "/var/lib/subs", "数据目录")
	service := fs.String("service", "subs", "systemd 服务名")
	purge := fs.Bool("purge", false, "同时删除数据目录（含数据库与密钥）")
	yes := fs.Bool("yes", false, "跳过交互确认")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if os.Geteuid() != 0 {
		return errors.New("请用 root 运行 uninstall")
	}
	if !*yes {
		fmt.Printf("将停止并删除服务 %s 与二进制 /usr/local/bin/subs。\n", *service)
		if *purge {
			fmt.Printf("并删除数据目录 %s（不可恢复，请先备份）。\n", *dataDir)
		}
		fmt.Fprint(os.Stderr, "输入 yes 继续: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.TrimSpace(line) != "yes" {
			return errors.New("已取消")
		}
	}
	unit := "/etc/systemd/system/" + *service + ".service"
	if _, err := os.Stat(unit); err == nil {
		_ = execCommand("systemctl", "disable", "--now", *service)
		if err := os.Remove(unit); err != nil {
			return err
		}
		_ = execCommand("systemctl", "daemon-reload")
		fmt.Printf("已移除 %s\n", unit)
	}
	if err := os.Remove("/usr/local/bin/subs"); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Println("已移除 /usr/local/bin/subs")
	if *purge {
		if err := os.RemoveAll(*dataDir); err != nil {
			return err
		}
		fmt.Printf("已删除 %s\n", *dataDir)
		if idCmd := execCommand("id", "-u", *service); idCmd == nil {
			_ = execCommand("userdel", *service)
			fmt.Printf("已删除系统用户 %s\n", *service)
		}
	} else {
		fmt.Printf("数据保留在 %s，如需删除请加 --purge\n", *dataDir)
	}
	return nil
}

func execCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
