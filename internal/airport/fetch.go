// Package airport pulls remote subscription payloads with SSRF protection and
// merges parsed nodes into the database by fingerprint.
package airport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/WuCaiCaiCai/SubS/internal/protocol"
)

// ErrBlockedURL is returned when a subscription URL targets a private network.
var ErrBlockedURL = errors.New("目标地址被拒绝：不允许访问内网或本机地址")

const maxBody = 8 << 20 // 8 MiB

// Fetch downloads a subscription payload, rejecting non-public targets.
func Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("订阅链接格式错误")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("订阅链接必须以 http 或 https 开头")
	}
	if err := guardHost(ctx, u.Hostname()); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("无法构造请求")
	}
	req.Header.Set("User-Agent", "clash.meta")
	client := &http.Client{
		Timeout: 25 * time.Second,
		CheckRedirect: func(next *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("重定向次数过多")
			}
			if err := guardHost(next.Context(), next.URL.Hostname()); err != nil {
				return err
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("拉取失败: %v", sanitizeErr(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("订阅服务器返回 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败")
	}
	if len(body) > maxBody {
		return nil, fmt.Errorf("订阅内容超过 8MB 限制")
	}
	return body, nil
}

// guardHost rejects loopback, private, link-local and unique-local addresses.
func guardHost(ctx context.Context, host string) error {
	if host == "" {
		return fmt.Errorf("缺少主机名")
	}
	if strings.EqualFold(host, "localhost") {
		return ErrBlockedURL
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil {
		return fmt.Errorf("域名解析失败")
	}
	for _, a := range addrs {
		if isBlockedIP(a.IP) {
			return ErrBlockedURL
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

// sanitizeErr strips the URL (which may carry a token) from transport errors.
func sanitizeErr(err error) string {
	var ue *url.Error
	if errors.As(err, &ue) {
		if errors.Is(ue.Err, context.DeadlineExceeded) {
			return "请求超时"
		}
		return ue.Err.Error()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "请求超时"
	}
	return "网络错误"
}

// RefreshStats summarises one refresh run.
type RefreshStats struct {
	Added   int                   `json:"added"`
	Matched int                   `json:"matched"`
	Missing int                   `json:"missing"`
	Ignored int                   `json:"ignored"`
	Errors  []protocol.ParseError `json:"errors"`
}

// Merge upserts parsed nodes for an airport inside a single transaction.
// Existing fingerprints are updated, new ones inserted as ready (is_new=0), and
// nodes that disappeared this round are removed (along with stale profile_item refs).
func Merge(ctx context.Context, tx *sql.Tx, airportID int64, parsed []protocol.Parsed) (RefreshStats, error) {
	stats := RefreshStats{Errors: []protocol.ParseError{}}
	rows, err := tx.QueryContext(ctx,
		"SELECT id, fingerprint FROM node WHERE source_type='airport' AND source_id=?", airportID)
	if err != nil {
		return stats, err
	}
	byFP := map[string]int64{}
	for rows.Next() {
		var id int64
		var fp string
		if err := rows.Scan(&id, &fp); err != nil {
			rows.Close()
			return stats, err
		}
		byFP[fp] = id
	}
	rows.Close()

	// Flag everything stale first; upserts below clear the flag for survivors.
	if _, err := tx.ExecContext(ctx,
		"UPDATE node SET missing=1 WHERE source_type='airport' AND source_id=?", airportID); err != nil {
		return stats, err
	}

	seen := make(map[string]bool, len(parsed))
	for _, p := range parsed {
		fp := protocol.Fingerprint(p.Config)
		if seen[fp] {
			stats.Ignored++
			continue
		}
		seen[fp] = true
		cfgJSON, err := json.Marshal(p.Config)
		if err != nil {
			return stats, err
		}
		if id, ok := byFP[fp]; ok {
			stats.Matched++
			if _, err := tx.ExecContext(ctx,
				"UPDATE node SET raw_name=?, config=?, missing=0 WHERE id=?",
				p.Name, string(cfgJSON), id); err != nil {
				return stats, err
			}
			continue
		}
		stats.Added++
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO node(source_type, source_id, protocol, raw_name, fingerprint, config, is_new, missing)
			 VALUES('airport', ?, ?, ?, ?, ?, 0, 0)`,
			airportID, p.Config.Protocol, p.Name, fp, string(cfgJSON)); err != nil {
			return stats, err
		}
	}

	var missing int
	if err := tx.QueryRowContext(ctx,
		"SELECT count(*) FROM node WHERE source_type='airport' AND source_id=? AND missing=1", airportID).Scan(&missing); err != nil {
		return stats, err
	}
	stats.Missing = missing

	if missing > 0 {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM profile_item WHERE kind='node' AND ref_id IN
			 (SELECT id FROM node WHERE source_type='airport' AND source_id=? AND missing=1)`, airportID); err != nil {
			return stats, err
		}
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM node WHERE source_type='airport' AND source_id=? AND missing=1", airportID); err != nil {
			return stats, err
		}
	}

	var total int
	if err := tx.QueryRowContext(ctx,
		"SELECT count(*) FROM node WHERE source_type='airport' AND source_id=?", airportID).Scan(&total); err != nil {
		return stats, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE airport SET node_count=?, last_ok_at=? WHERE id=?",
		total, time.Now().UTC().Format(time.RFC3339), airportID); err != nil {
		return stats, err
	}
	return stats, nil
}
