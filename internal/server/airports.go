package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/WuCaiCaiCai/SubS/internal/airport"
	"github.com/WuCaiCaiCai/SubS/internal/profile"
	"github.com/WuCaiCaiCai/SubS/internal/protocol"
	"github.com/WuCaiCaiCai/SubS/internal/secrets"
)

type airportView struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	URL                string `json:"url"`
	Enabled            bool   `json:"enabled"`
	Note               string `json:"note"`
	RefreshIntervalMin *int64 `json:"refreshIntervalMin"`
	LastOKAt           string `json:"lastOkAt"`
	LastError          string `json:"lastError"`
	NodeCount          int64  `json:"nodeCount"`
}

func (s *Server) loadAirport(r *http.Request, id int64) (airportView, error) {
	var v airportView
	var encURL, note, lastOK, lastErr sql.NullString
	var enabled int
	var interval sql.NullInt64
	err := s.Store.DB().QueryRowContext(r.Context(),
		`SELECT id,name,secret_url,enabled,note,refresh_interval_min,last_ok_at,last_error,COALESCE(node_count,0)
		 FROM airport WHERE id=?`, id).
		Scan(&v.ID, &v.Name, &encURL, &enabled, &note, &interval, &lastOK, &lastErr, &v.NodeCount)
	if err != nil {
		return v, err
	}
	v.Enabled = enabled != 0
	v.Note = note.String
	v.LastOKAt = lastOK.String
	v.LastError = lastErr.String
	if interval.Valid {
		v.RefreshIntervalMin = &interval.Int64
	}
	plain, err := s.decrypt(encURL.String)
	if err != nil {
		v.URL = "[无法解密]"
	} else {
		v.URL = profile.RedactURL(plain)
	}
	return v, nil
}

func (s *Server) listAirports(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.DB().QueryContext(r.Context(), "SELECT id FROM airport ORDER BY id DESC")
	if err != nil {
		fail(w, 500, "database_error", "读取机场列表失败")
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	out := make([]airportView, 0, len(ids))
	for _, id := range ids {
		v, err := s.loadAirport(r, id)
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}

const (
	minRefreshIntervalMin = 15
	maxRefreshIntervalMin = 10080 // 7 天
)

func validateRefreshInterval(min *int64) error {
	if min == nil {
		return nil
	}
	if *min < minRefreshIntervalMin || *min > maxRefreshIntervalMin {
		return errors.New("自动刷新间隔需在 15 分钟至 7 天之间")
	}
	return nil
}

func (s *Server) createAirport(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name               string `json:"name"`
		URL                string `json:"url"`
		Note               string `json:"note"`
		RefreshIntervalMin *int64 `json:"refreshIntervalMin"`
	}
	if decodeJSON(w, r, &in, 32<<10) != nil || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.URL) == "" {
		fail(w, 400, "invalid_input", "机场名称与订阅链接不能为空")
		return
	}
	if err := validateRefreshInterval(in.RefreshIntervalMin); err != nil {
		fail(w, 400, "invalid_input", err.Error())
		return
	}
	enc, err := s.encrypt(strings.TrimSpace(in.URL))
	if err != nil {
		fail(w, 500, "encrypt_failed", "保存订阅链接失败")
		return
	}
	res, err := s.Store.DB().ExecContext(r.Context(),
		"INSERT INTO airport(name,secret_url,note,refresh_interval_min) VALUES(?,?,?,?)",
		strings.TrimSpace(in.Name), enc, nullString(in.Note), in.RefreshIntervalMin)
	if err != nil {
		fail(w, 500, "database_error", "创建机场失败")
		return
	}
	id, _ := res.LastInsertId()
	v, err := s.loadAirport(r, id)
	if err != nil {
		fail(w, 500, "database_error", "读取机场失败")
		return
	}
	writeJSON(w, 201, v)
}

func (s *Server) getAirport(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil || id < 1 {
		fail(w, 400, "invalid_id", "机场 ID 无效")
		return
	}
	v, err := s.loadAirport(r, id)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "not_found", "机场不存在")
		return
	}
	if err != nil {
		fail(w, 500, "database_error", "读取机场失败")
		return
	}
	writeJSON(w, 200, v)
}

func (s *Server) updateAirport(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil || id < 1 {
		fail(w, 400, "invalid_id", "机场 ID 无效")
		return
	}
	var in struct {
		Name               string `json:"name"`
		URL                string `json:"url"`
		Note               string `json:"note"`
		Enabled            *bool  `json:"enabled"`
		RefreshIntervalMin *int64 `json:"refreshIntervalMin"`
	}
	if decodeJSON(w, r, &in, 32<<10) != nil || strings.TrimSpace(in.Name) == "" {
		fail(w, 400, "invalid_input", "机场名称不能为空")
		return
	}
	if err := validateRefreshInterval(in.RefreshIntervalMin); err != nil {
		fail(w, 400, "invalid_input", err.Error())
		return
	}
	enabled := 1
	if in.Enabled != nil && !*in.Enabled {
		enabled = 0
	}
	if in.Enabled == nil {
		_ = s.Store.DB().QueryRowContext(r.Context(), "SELECT enabled FROM airport WHERE id=?", id).Scan(&enabled)
	}
	if strings.TrimSpace(in.URL) != "" && !strings.Contains(in.URL, "***") {
		enc, err := s.encrypt(strings.TrimSpace(in.URL))
		if err != nil {
			fail(w, 500, "encrypt_failed", "保存订阅链接失败")
			return
		}
		_, err = s.Store.DB().ExecContext(r.Context(),
			"UPDATE airport SET name=?,secret_url=?,note=?,enabled=?,refresh_interval_min=? WHERE id=?",
			strings.TrimSpace(in.Name), enc, nullString(in.Note), enabled, in.RefreshIntervalMin, id)
		if err != nil {
			fail(w, 500, "database_error", "更新机场失败")
			return
		}
	} else {
		_, err = s.Store.DB().ExecContext(r.Context(),
			"UPDATE airport SET name=?,note=?,enabled=?,refresh_interval_min=? WHERE id=?",
			strings.TrimSpace(in.Name), nullString(in.Note), enabled, in.RefreshIntervalMin, id)
		if err != nil {
			fail(w, 500, "database_error", "更新机场失败")
			return
		}
	}
	v, err := s.loadAirport(r, id)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "not_found", "机场不存在")
		return
	}
	if err != nil {
		fail(w, 500, "database_error", "读取机场失败")
		return
	}
	writeJSON(w, 200, v)
}

func (s *Server) deleteAirport(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil || id < 1 {
		fail(w, 400, "invalid_id", "机场 ID 无效")
		return
	}
	force := r.URL.Query().Get("force") == "1"
	var refs int
	if err := s.Store.DB().QueryRowContext(r.Context(),
		`SELECT count(*) FROM profile_item i WHERE
		   (i.kind='source' AND i.source_type='airport' AND i.ref_id=?)
		   OR (i.kind='node' AND i.ref_id IN
		       (SELECT id FROM node WHERE source_type='airport' AND source_id=?))`, id, id).Scan(&refs); err != nil {
		fail(w, 500, "database_error", "检查机场引用失败")
		return
	}
	if refs > 0 && !force {
		writeJSON(w, 409, map[string]any{"error": map[string]any{
			"code": "in_use", "message": "此机场的节点被订阅编排引用", "references": refs}})
		return
	}
	err = s.Store.Write(r.Context(), func(tx *sql.Tx) error {
		if force {
			for _, q := range []string{
				`DELETE FROM profile_item WHERE kind='source' AND source_type='airport' AND ref_id=?`,
				`DELETE FROM profile_item WHERE kind='node' AND ref_id IN
				 (SELECT id FROM node WHERE source_type='airport' AND source_id=?)`,
			} {
				if _, err := tx.ExecContext(r.Context(), q, id); err != nil {
					return err
				}
			}
		}
		if _, err := tx.ExecContext(r.Context(),
			"DELETE FROM node WHERE source_type='airport' AND source_id=?", id); err != nil {
			return err
		}
		_, err := tx.ExecContext(r.Context(), "DELETE FROM airport WHERE id=?", id)
		return err
	})
	if err != nil {
		fail(w, 500, "database_error", "删除机场失败")
		return
	}
	w.WriteHeader(204)
}

type refreshFail struct {
	status    int
	code      string
	message   string
	parseErrs []protocol.ParseError
}

func (s *Server) acquireRefresh(id int64) (func(), bool) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	if s.refreshing == nil {
		s.refreshing = make(map[int64]bool)
	}
	if s.refreshing[id] {
		return nil, false
	}
	s.refreshing[id] = true
	return func() {
		s.refreshMu.Lock()
		delete(s.refreshing, id)
		s.refreshMu.Unlock()
	}, true
}

func (s *Server) doRefreshAirport(ctx context.Context, id int64) (airport.RefreshStats, *refreshFail) {
	release, ok := s.acquireRefresh(id)
	if !ok {
		return airport.RefreshStats{}, &refreshFail{
			status:  http.StatusConflict,
			code:    "refresh_in_progress",
			message: "该机场正在刷新中，请稍后再试",
		}
	}
	defer release()

	var encURL string
	if err := s.Store.DB().QueryRowContext(ctx, "SELECT secret_url FROM airport WHERE id=?", id).Scan(&encURL); err != nil {
		return airport.RefreshStats{}, &refreshFail{status: 404, code: "not_found", message: "机场不存在"}
	}
	rawURL, err := s.decrypt(encURL)
	if err != nil {
		return airport.RefreshStats{}, &refreshFail{status: 500, code: "decrypt_failed", message: "订阅链接解密失败"}
	}
	body, err := s.fetchSubscription(ctx, rawURL)
	if err != nil {
		msg := err.Error()
		_, _ = s.Store.DB().ExecContext(ctx, "UPDATE airport SET last_error=? WHERE id=?", msg, id)
		return airport.RefreshStats{}, &refreshFail{status: 502, code: "fetch_failed", message: msg}
	}
	parsed, parseErrs, ignored := protocol.ParseSubscription(body)
	if len(parsed) == 0 {
		msg := "订阅内容未解析出可用节点"
		if len(parseErrs) > 0 {
			msg += ": " + parseErrs[0].Reason
		}
		_, _ = s.Store.DB().ExecContext(ctx, "UPDATE airport SET last_error=? WHERE id=?", msg, id)
		return airport.RefreshStats{}, &refreshFail{status: 422, code: "no_nodes", message: msg, parseErrs: parseErrs}
	}
	var stats airport.RefreshStats
	err = s.Store.Write(ctx, func(tx *sql.Tx) error {
		var e error
		stats, e = airport.Merge(ctx, tx, id, parsed)
		return e
	})
	if err != nil {
		return airport.RefreshStats{}, &refreshFail{status: 500, code: "database_error", message: "写入节点失败"}
	}
	stats.Ignored += ignored
	stats.Errors = parseErrs
	_, _ = s.Store.DB().ExecContext(ctx, "UPDATE airport SET last_error=NULL WHERE id=?", id)
	return stats, nil
}

func (s *Server) refreshAirport(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil || id < 1 {
		fail(w, 400, "invalid_id", "机场 ID 无效")
		return
	}
	stats, rerr := s.doRefreshAirport(r.Context(), id)
	if rerr != nil {
		errObj := map[string]any{"code": rerr.code, "message": rerr.message}
		if rerr.parseErrs != nil {
			errObj["errors"] = rerr.parseErrs
		}
		writeJSON(w, rerr.status, map[string]any{"error": errObj})
		return
	}
	writeJSON(w, 200, stats)
}

// StartAutoRefresh runs a background ticker that refreshes enabled airports
// whose refresh_interval_min has elapsed since their last successful/attempted run.
func (s *Server) StartAutoRefresh(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.runAutoRefreshPass(ctx, now)
		}
	}
}

type dueAirport struct {
	id          int64
	name        string
	intervalMin int64
	lastOKAt    string
}

func isAirportDue(intervalMin int64, lastOKAt string, lastAttemptAt time.Time, now time.Time) bool {
	if intervalMin <= 0 {
		return false
	}
	interval := time.Duration(intervalMin) * time.Minute
	// 失败后按同一间隔退避，避免订阅源不可达时每分钟重复请求。
	if !lastAttemptAt.IsZero() && now.Sub(lastAttemptAt) < interval {
		return false
	}
	if lastOKAt == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, lastOKAt)
	if err != nil {
		return true
	}
	return now.Sub(t) >= interval
}

func (s *Server) runAutoRefreshPass(ctx context.Context, now time.Time) {
	rows, err := s.Store.DB().QueryContext(ctx,
		`SELECT id, name, refresh_interval_min, COALESCE(last_ok_at, '')
		 FROM airport
		 WHERE enabled=1 AND refresh_interval_min IS NOT NULL AND refresh_interval_min > 0
		 ORDER BY id ASC`)
	if err != nil {
		return
	}
	var candidates []dueAirport
	for rows.Next() {
		var item dueAirport
		if rows.Scan(&item.id, &item.name, &item.intervalMin, &item.lastOKAt) == nil {
			candidates = append(candidates, item)
		}
	}
	rows.Close()

	for _, item := range candidates {
		if ctx.Err() != nil {
			return
		}
		s.refreshMu.Lock()
		var prevAttempt time.Time
		if s.lastAttempt != nil {
			prevAttempt = s.lastAttempt[item.id]
		}
		due := isAirportDue(item.intervalMin, item.lastOKAt, prevAttempt, now)
		if due {
			if s.lastAttempt == nil {
				s.lastAttempt = make(map[int64]time.Time)
			}
			s.lastAttempt[item.id] = now
		}
		s.refreshMu.Unlock()
		if !due {
			continue
		}

		reqCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		stats, rerr := s.doRefreshAirport(reqCtx, item.id)
		cancel()
		if rerr != nil {
			log.Printf("自动刷新机场 %s (#%d) 失败: %s", item.name, item.id, rerr.message)
		} else {
			log.Printf("自动刷新机场 %s (#%d) 完成: +%d 新增, %d 匹配, %d 缺失",
				item.name, item.id, stats.Added, stats.Matched, stats.Missing)
		}
	}
}

func (s *Server) confirmNew(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil || id < 1 {
		fail(w, 400, "invalid_id", "机场 ID 无效")
		return
	}
	res, err := s.Store.DB().ExecContext(r.Context(),
		"UPDATE node SET is_new=0 WHERE source_type='airport' AND source_id=? AND is_new=1", id)
	if err != nil {
		fail(w, 500, "database_error", "确认新节点失败")
		return
	}
	n, _ := res.RowsAffected()
	writeJSON(w, 200, map[string]any{"confirmed": n})
}

func (s *Server) encrypt(plain string) (string, error) {
	box, err := s.box()
	if err != nil {
		return "", err
	}
	return box.Encrypt(plain)
}

func (s *Server) decrypt(enc string) (string, error) {
	box, err := s.box()
	if err != nil {
		return "", err
	}
	return box.Decrypt(enc)
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

var _ = json.Marshal
var _ = secrets.Secret("")
