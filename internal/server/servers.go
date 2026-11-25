package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

type serverView struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Address   string   `json:"address"`
	Note      string   `json:"note"`
	Tags      []string `json:"tags"`
	NodeCount int64    `json:"nodeCount"`
}

func (s *Server) listServers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.DB().QueryContext(r.Context(),
		`SELECT s.id,s.name,COALESCE(s.address,''),COALESCE(s.note,''),COALESCE(s.tags,'[]'),
		        (SELECT count(*) FROM node n WHERE n.source_type='server' AND n.source_id=s.id)
		 FROM server s ORDER BY s.id DESC`)
	if err != nil {
		fail(w, 500, "database_error", "读取 VPS 列表失败")
		return
	}
	defer rows.Close()
	out := []serverView{}
	for rows.Next() {
		var v serverView
		var tags string
		if err := rows.Scan(&v.ID, &v.Name, &v.Address, &v.Note, &tags, &v.NodeCount); err != nil {
			fail(w, 500, "database_error", "读取 VPS 列表失败")
			return
		}
		v.Tags = parseTags(tags)
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}

type serverInput struct {
	Name    string   `json:"name"`
	Address string   `json:"address"`
	Note    string   `json:"note"`
	Tags    []string `json:"tags"`
}

func (s *Server) createServer(w http.ResponseWriter, r *http.Request) {
	var in serverInput
	if decodeJSON(w, r, &in, 32<<10) != nil || strings.TrimSpace(in.Name) == "" {
		fail(w, 400, "invalid_input", "VPS 名称不能为空")
		return
	}
	tags, _ := json.Marshal(nonNil(in.Tags))
	res, err := s.Store.DB().ExecContext(r.Context(),
		"INSERT INTO server(name,address,note,tags) VALUES(?,?,?,?)",
		strings.TrimSpace(in.Name), nullString(in.Address), nullString(in.Note), string(tags))
	if err != nil {
		fail(w, 500, "database_error", "创建 VPS 失败")
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, 201, serverView{ID: id, Name: strings.TrimSpace(in.Name), Address: in.Address, Note: in.Note, Tags: nonNil(in.Tags)})
}

func (s *Server) getServer(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	var v serverView
	var tags string
	err := s.Store.DB().QueryRowContext(r.Context(),
		`SELECT s.id,s.name,COALESCE(s.address,''),COALESCE(s.note,''),COALESCE(s.tags,'[]'),
		        (SELECT count(*) FROM node n WHERE n.source_type='server' AND n.source_id=s.id)
		 FROM server s WHERE s.id=?`, id).Scan(&v.ID, &v.Name, &v.Address, &v.Note, &tags, &v.NodeCount)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "not_found", "VPS 不存在")
		return
	}
	if err != nil {
		fail(w, 500, "database_error", "读取 VPS 失败")
		return
	}
	v.Tags = parseTags(tags)
	writeJSON(w, 200, v)
}

func (s *Server) updateServer(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	var in serverInput
	if decodeJSON(w, r, &in, 32<<10) != nil || strings.TrimSpace(in.Name) == "" {
		fail(w, 400, "invalid_input", "VPS 名称不能为空")
		return
	}
	tags, _ := json.Marshal(nonNil(in.Tags))
	res, err := s.Store.DB().ExecContext(r.Context(),
		"UPDATE server SET name=?,address=?,note=?,tags=? WHERE id=?",
		strings.TrimSpace(in.Name), nullString(in.Address), nullString(in.Note), string(tags), id)
	if err != nil {
		fail(w, 500, "database_error", "更新 VPS 失败")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fail(w, 404, "not_found", "VPS 不存在")
		return
	}
	writeJSON(w, 200, serverView{ID: id, Name: strings.TrimSpace(in.Name), Address: in.Address, Note: in.Note, Tags: nonNil(in.Tags)})
}

func (s *Server) deleteServer(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	force := r.URL.Query().Get("force") == "1"
	var refs int
	if err := s.Store.DB().QueryRowContext(r.Context(),
		`SELECT count(*) FROM node n JOIN profile_node pn ON pn.node_id=n.id
		 WHERE n.source_type='server' AND n.source_id=?`, id).Scan(&refs); err != nil {
		fail(w, 500, "database_error", "检查 VPS 引用失败")
		return
	}
	if refs > 0 && !force {
		writeJSON(w, 409, map[string]any{"error": map[string]any{
			"code": "in_use", "message": "此 VPS 的节点被订阅编排引用", "references": refs}})
		return
	}
	err := s.Store.Write(r.Context(), func(tx *sql.Tx) error {
		if force {
			if _, err := tx.ExecContext(r.Context(),
				`DELETE FROM profile_node WHERE node_id IN
				 (SELECT id FROM node WHERE source_type='server' AND source_id=?)`, id); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(r.Context(),
			"DELETE FROM node WHERE source_type='server' AND source_id=?", id); err != nil {
			return err
		}
		_, err := tx.ExecContext(r.Context(), "DELETE FROM server WHERE id=?", id)
		return err
	})
	if err != nil {
		fail(w, 500, "database_error", "删除 VPS 失败")
		return
	}
	w.WriteHeader(204)
}

func requireID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := pathID(r)
	if err != nil || id < 1 {
		fail(w, 400, "invalid_id", "ID 无效")
		return 0, false
	}
	return id, true
}

func parseTags(raw string) []string {
	tags := []string{}
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &tags)
	}
	if tags == nil {
		tags = []string{}
	}
	return tags
}

func nonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func nullString(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.TrimSpace(s)
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func validProtocol(p string) bool {
	switch p {
	case "hy2", "vless", "tuic", "anytls", "mieru":
		return true
	}
	return false
}

var _ = strconv.Itoa
