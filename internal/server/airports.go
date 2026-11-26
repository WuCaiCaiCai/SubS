package server

import (
	"database/sql"
	"encoding/json"
	"errors"
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

func (s *Server) refreshAirport(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil || id < 1 {
		fail(w, 400, "invalid_id", "机场 ID 无效")
		return
	}
	var encURL string
	if err := s.Store.DB().QueryRowContext(r.Context(), "SELECT secret_url FROM airport WHERE id=?", id).Scan(&encURL); err != nil {
		fail(w, 404, "not_found", "机场不存在")
		return
	}
	rawURL, err := s.decrypt(encURL)
	if err != nil {
		fail(w, 500, "decrypt_failed", "订阅链接解密失败")
		return
	}
	body, err := airport.Fetch(r.Context(), rawURL)
	if err != nil {
		msg := err.Error()
		_, _ = s.Store.DB().ExecContext(r.Context(), "UPDATE airport SET last_error=? WHERE id=?", msg, id)
		writeJSON(w, 502, map[string]any{"error": map[string]any{"code": "fetch_failed", "message": msg}})
		return
	}
	parsed, parseErrs, ignored := protocol.ParseSubscription(body)
	if len(parsed) == 0 {
		msg := "订阅内容未解析出可用节点"
		if len(parseErrs) > 0 {
			msg += ": " + parseErrs[0].Reason
		}
		_, _ = s.Store.DB().ExecContext(r.Context(), "UPDATE airport SET last_error=? WHERE id=?", msg, id)
		writeJSON(w, 422, map[string]any{"error": map[string]any{"code": "no_nodes", "message": msg, "errors": parseErrs}})
		return
	}
	var stats airport.RefreshStats
	err = s.Store.Write(r.Context(), func(tx *sql.Tx) error {
		var e error
		stats, e = airport.Merge(r.Context(), tx, id, parsed)
		return e
	})
	if err != nil {
		fail(w, 500, "database_error", "写入节点失败")
		return
	}
	stats.Ignored += ignored
	stats.Errors = parseErrs
	_, _ = s.Store.DB().ExecContext(r.Context(), "UPDATE airport SET last_error=NULL WHERE id=?", id)
	writeJSON(w, 200, stats)
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
