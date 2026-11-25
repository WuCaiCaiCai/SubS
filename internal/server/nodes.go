package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/wucai/subs/internal/protocol"
)

type nodeView struct {
	ID          int64           `json:"id"`
	SourceType  string          `json:"sourceType"`
	SourceID    int64           `json:"sourceId"`
	SourceName  string          `json:"sourceName"`
	Protocol    string          `json:"protocol"`
	RawName     string          `json:"rawName"`
	Host        string          `json:"host"`
	Port        int             `json:"port"`
	PortRange   string          `json:"portRange"`
	Fingerprint string          `json:"fingerprint"`
	Config      json.RawMessage `json:"config"`
	Enabled     bool            `json:"enabled"`
	Missing     bool            `json:"missing"`
	IsNew       bool            `json:"isNew"`
	Tags        []string        `json:"tags"`
	InProfiles  int64           `json:"inProfiles"`
}

const nodeSelect = `SELECT n.id,n.source_type,n.source_id,n.protocol,n.raw_name,n.fingerprint,n.config,
       n.enabled,n.missing,n.is_new,COALESCE(n.tags,'[]'),
       CASE WHEN n.source_type='airport' THEN COALESCE((SELECT a.name FROM airport a WHERE a.id=n.source_id),'')
            ELSE COALESCE((SELECT s.name FROM server s WHERE s.id=n.source_id),'') END,
       (SELECT count(*) FROM profile_node pn WHERE pn.node_id=n.id)
       FROM node n`

func scanNode(rows interface{ Scan(...any) error }) (nodeView, error) {
	var v nodeView
	var enabled, missing, isNew int
	var tags, config string
	err := rows.Scan(&v.ID, &v.SourceType, &v.SourceID, &v.Protocol, &v.RawName, &v.Fingerprint,
		&config, &enabled, &missing, &isNew, &tags, &v.SourceName, &v.InProfiles)
	if err != nil {
		return v, err
	}
	v.Config = json.RawMessage(config)
	v.Enabled = enabled != 0
	v.Missing = missing != 0
	v.IsNew = isNew != 0
	v.Tags = parseTags(tags)
	v.Host, v.Port, v.PortRange = configEndpoint(v.Config)
	return v, nil
}

func configEndpoint(raw json.RawMessage) (string, int, string) {
	var c protocol.Config
	if json.Unmarshal(raw, &c) == nil {
		pr, _ := c.Extra["portRange"].(string)
		return c.Host, c.Port, pr
	}
	return "", 0, ""
}

func (s *Server) listNodes(w http.ResponseWriter, r *http.Request) {
	q := nodeSelect + " WHERE 1=1"
	args := []any{}
	filters := []struct{ param, col string }{
		{"sourceType", "n.source_type"},
		{"sourceId", "n.source_id"},
		{"protocol", "n.protocol"},
	}
	for _, f := range filters {
		v := r.URL.Query().Get(f.param)
		if v == "" {
			continue
		}
		if f.param == "sourceId" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				fail(w, 400, "invalid_filter", "sourceId 无效")
				return
			}
			args = append(args, n)
		} else {
			args = append(args, v)
		}
		q += " AND " + f.col + "=?"
	}
	for _, f := range []struct{ param, col string }{{"enabled", "n.enabled"}, {"isNew", "n.is_new"}, {"missing", "n.missing"}} {
		v := r.URL.Query().Get(f.param)
		if v == "" {
			continue
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			fail(w, 400, "invalid_filter", f.param+" 无效")
			return
		}
		if b {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
		q += " AND " + f.col + "=?"
	}
	if term := strings.TrimSpace(r.URL.Query().Get("q")); term != "" {
		q += ` AND n.raw_name LIKE ? ESCAPE '\'`
		args = append(args, "%"+escapeLike(term)+"%")
	}
	q += " ORDER BY n.id DESC LIMIT 1000"
	rows, err := s.Store.DB().QueryContext(r.Context(), q, args...)
	if err != nil {
		fail(w, 500, "database_error", "读取节点失败: "+err.Error())
		return
	}
	defer rows.Close()
	out := []nodeView{}
	for rows.Next() {
		v, err := scanNode(rows)
		if err != nil {
			fail(w, 500, "database_error", "读取节点失败: "+err.Error())
			return
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}

func (s *Server) getNode(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	v, err := scanNode(s.Store.DB().QueryRowContext(r.Context(), nodeSelect+" WHERE n.id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "not_found", "节点不存在")
		return
	}
	if err != nil {
		fail(w, 500, "database_error", "读取节点失败")
		return
	}
	writeJSON(w, 200, v)
}

func (s *Server) createNode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SourceID int64           `json:"sourceId"`
		Protocol string          `json:"protocol"`
		RawName  string          `json:"rawName"`
		Config   json.RawMessage `json:"config"`
		Tags     []string        `json:"tags"`
	}
	if decodeJSON(w, r, &in, 256<<10) != nil {
		fail(w, 400, "invalid_input", "请求体格式错误")
		return
	}
	if in.SourceID < 1 || strings.TrimSpace(in.RawName) == "" || !validProtocol(in.Protocol) || !json.Valid(in.Config) {
		fail(w, 400, "invalid_input", "节点字段无效")
		return
	}
	var exists int
	if err := s.Store.DB().QueryRowContext(r.Context(), "SELECT count(*) FROM server WHERE id=?", in.SourceID).Scan(&exists); err != nil || exists == 0 {
		fail(w, 404, "server_not_found", "VPS 不存在")
		return
	}
	var cfg protocol.Config
	if err := json.Unmarshal(in.Config, &cfg); err != nil || cfg.Host == "" || cfg.Port < 1 {
		fail(w, 400, "invalid_config", "节点配置缺少 host/port")
		return
	}
	cfg.Protocol = in.Protocol
	normalized, _ := json.Marshal(cfg)
	tags, _ := json.Marshal(nonNil(in.Tags))
	res, err := s.Store.DB().ExecContext(r.Context(),
		`INSERT INTO node(source_type,source_id,protocol,raw_name,fingerprint,config,tags)
		 VALUES('server',?,?,?,?,?,?)`,
		in.SourceID, in.Protocol, strings.TrimSpace(in.RawName), protocol.Fingerprint(cfg), string(normalized), string(tags))
	if err != nil {
		fail(w, 409, "duplicate_node", "节点已存在或保存失败")
		return
	}
	id, _ := res.LastInsertId()
	v, err := scanNode(s.Store.DB().QueryRowContext(r.Context(), nodeSelect+" WHERE n.id=?", id))
	if err != nil {
		fail(w, 500, "database_error", "读取节点失败")
		return
	}
	writeJSON(w, 201, v)
}

func (s *Server) updateNode(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	var in struct {
		RawName string          `json:"rawName"`
		Config  json.RawMessage `json:"config"`
		Enabled *bool           `json:"enabled"`
		Tags    []string        `json:"tags"`
	}
	if decodeJSON(w, r, &in, 256<<10) != nil {
		fail(w, 400, "invalid_input", "请求体格式错误")
		return
	}
	var protocolName string
	if err := s.Store.DB().QueryRowContext(r.Context(), "SELECT protocol FROM node WHERE id=?", id).Scan(&protocolName); err != nil {
		fail(w, 404, "not_found", "节点不存在")
		return
	}
	var cfg protocol.Config
	if len(in.Config) > 0 {
		if err := json.Unmarshal(in.Config, &cfg); err != nil || cfg.Host == "" || cfg.Port < 1 {
			fail(w, 400, "invalid_config", "节点配置缺少 host/port")
			return
		}
	} else {
		var raw string
		if err := s.Store.DB().QueryRowContext(r.Context(), "SELECT config FROM node WHERE id=?", id).Scan(&raw); err != nil {
			fail(w, 500, "database_error", "读取节点失败")
			return
		}
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	cfg.Protocol = protocolName
	normalized, _ := json.Marshal(cfg)
	enabled := 1
	if in.Enabled != nil {
		if !*in.Enabled {
			enabled = 0
		}
	} else {
		_ = s.Store.DB().QueryRowContext(r.Context(), "SELECT enabled FROM node WHERE id=?", id).Scan(&enabled)
	}
	rawName := strings.TrimSpace(in.RawName)
	if rawName == "" {
		_ = s.Store.DB().QueryRowContext(r.Context(), "SELECT raw_name FROM node WHERE id=?", id).Scan(&rawName)
	}
	tags, _ := json.Marshal(nonNil(in.Tags))
	if _, err := s.Store.DB().ExecContext(r.Context(),
		"UPDATE node SET raw_name=?,config=?,fingerprint=?,enabled=?,tags=? WHERE id=?",
		rawName, string(normalized), protocol.Fingerprint(cfg), enabled, string(tags), id); err != nil {
		fail(w, 409, "duplicate_node", "更新失败，可能存在重复节点")
		return
	}
	v, err := scanNode(s.Store.DB().QueryRowContext(r.Context(), nodeSelect+" WHERE n.id=?", id))
	if err != nil {
		fail(w, 500, "database_error", "读取节点失败")
		return
	}
	writeJSON(w, 200, v)
}

func (s *Server) deleteNode(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	if err := s.Store.Write(r.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), "DELETE FROM node WHERE id=?", id)
		return err
	}); err != nil {
		fail(w, 500, "database_error", "删除节点失败")
		return
	}
	w.WriteHeader(204)
}

// importNodes previews parsed share links without persisting them.
func (s *Server) importNodes(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SourceID int64  `json:"sourceId"`
		Text     string `json:"text"`
		Commit   bool   `json:"commit"`
	}
	if decodeJSON(w, r, &in, 1<<20) != nil {
		fail(w, 400, "invalid_input", "请求体格式错误")
		return
	}
	if in.SourceID < 1 {
		fail(w, 400, "invalid_input", "请选择 VPS 分组")
		return
	}
	var exists int
	if err := s.Store.DB().QueryRowContext(r.Context(), "SELECT count(*) FROM server WHERE id=?", in.SourceID).Scan(&exists); err != nil || exists == 0 {
		fail(w, 404, "server_not_found", "VPS 不存在")
		return
	}
	parsed, errs, ignored := protocol.ParseSubscription([]byte(in.Text))
	type preview struct {
		RawName  string          `json:"rawName"`
		Protocol string          `json:"protocol"`
		Host     string          `json:"host"`
		Port     int             `json:"port"`
		Config   protocol.Config `json:"config"`
	}
	out := struct {
		Parsed  []preview             `json:"parsed"`
		Errors  []protocol.ParseError `json:"errors"`
		Ignored int                   `json:"ignored"`
		Saved   int                   `json:"saved"`
	}{Parsed: []preview{}, Errors: errs, Ignored: ignored}
	if out.Errors == nil {
		out.Errors = []protocol.ParseError{}
	}
	for _, p := range parsed {
		out.Parsed = append(out.Parsed, preview{
			RawName: p.Name, Protocol: p.Config.Protocol, Host: p.Config.Host, Port: p.Config.Port, Config: p.Config})
	}
	if in.Commit && len(parsed) > 0 {
		saved := 0
		err := s.Store.Write(r.Context(), func(tx *sql.Tx) error {
			for _, p := range parsed {
				cfgJSON, _ := json.Marshal(p.Config)
				res, err := tx.ExecContext(r.Context(),
					`INSERT INTO node(source_type,source_id,protocol,raw_name,fingerprint,config)
					 VALUES('server',?,?,?,?,?)
					 ON CONFLICT(source_type,source_id,fingerprint) DO UPDATE SET raw_name=excluded.raw_name, config=excluded.config, missing=0`,
					in.SourceID, p.Config.Protocol, p.Name, protocol.Fingerprint(p.Config), string(cfgJSON))
				if err != nil {
					return err
				}
				if n, _ := res.RowsAffected(); n > 0 {
					saved++
				}
			}
			return nil
		})
		if err != nil {
			fail(w, 500, "database_error", "保存节点失败")
			return
		}
		out.Saved = saved
	}
	writeJSON(w, 200, out)
}

// confirmAllNew 清除所有机场节点的“待确认”标记。
func (s *Server) confirmAllNew(w http.ResponseWriter, r *http.Request) {
	res, err := s.Store.DB().ExecContext(r.Context(),
		"UPDATE node SET is_new=0 WHERE source_type='airport' AND is_new=1")
	if err != nil {
		fail(w, 500, "database_error", "确认新节点失败")
		return
	}
	n, _ := res.RowsAffected()
	writeJSON(w, 200, map[string]any{"confirmed": n})
}

var _ = errors.Is
