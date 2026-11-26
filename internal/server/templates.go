package server

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/WuCaiCaiCai/SubS/internal/render"
)

type templateView struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Target    string `json:"target"`
	Content   string `json:"content"`
	LastCheck string `json:"lastCheck"`
}

func (s *Server) listTemplates(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.DB().QueryContext(r.Context(),
		"SELECT id,name,target,content,COALESCE(last_check,'') FROM template ORDER BY id DESC")
	if err != nil {
		fail(w, 500, "database_error", "读取模板列表失败")
		return
	}
	defer rows.Close()
	out := []templateView{}
	for rows.Next() {
		var v templateView
		if err := rows.Scan(&v.ID, &v.Name, &v.Target, &v.Content, &v.LastCheck); err != nil {
			fail(w, 500, "database_error", "读取模板列表失败")
			return
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}

func (s *Server) createTemplate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name    string `json:"name"`
		Target  string `json:"target"`
		Content string `json:"content"`
	}
	if decodeJSON(w, r, &in, 1<<20) != nil || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Content) == "" {
		fail(w, 400, "invalid_input", "模板名称与内容不能为空")
		return
	}
	if in.Target == "" {
		in.Target = render.TargetMihomo
	}
	if in.Target != render.TargetMihomo {
		fail(w, 400, "invalid_target", "一期仅支持 mihomo 模板")
		return
	}
	res, err := s.Store.DB().ExecContext(r.Context(),
		"INSERT INTO template(name,target,content) VALUES(?,?,?)",
		strings.TrimSpace(in.Name), in.Target, in.Content)
	if err != nil {
		fail(w, 500, "database_error", "创建模板失败")
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, 201, templateView{ID: id, Name: strings.TrimSpace(in.Name), Target: in.Target, Content: in.Content})
}

func (s *Server) getTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	var v templateView
	err := s.Store.DB().QueryRowContext(r.Context(),
		"SELECT id,name,target,content,COALESCE(last_check,'') FROM template WHERE id=?", id).
		Scan(&v.ID, &v.Name, &v.Target, &v.Content, &v.LastCheck)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "not_found", "模板不存在")
		return
	}
	if err != nil {
		fail(w, 500, "database_error", "读取模板失败")
		return
	}
	writeJSON(w, 200, v)
}

func (s *Server) updateTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	var in struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if decodeJSON(w, r, &in, 1<<20) != nil || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Content) == "" {
		fail(w, 400, "invalid_input", "模板名称与内容不能为空")
		return
	}
	res, err := s.Store.DB().ExecContext(r.Context(),
		"UPDATE template SET name=?,content=? WHERE id=?", strings.TrimSpace(in.Name), in.Content, id)
	if err != nil {
		fail(w, 500, "database_error", "更新模板失败")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fail(w, 404, "not_found", "模板不存在")
		return
	}
	var v templateView
	_ = s.Store.DB().QueryRowContext(r.Context(),
		"SELECT id,name,target,content,COALESCE(last_check,'') FROM template WHERE id=?", id).
		Scan(&v.ID, &v.Name, &v.Target, &v.Content, &v.LastCheck)
	writeJSON(w, 200, v)
}

func (s *Server) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	var refs int
	if err := s.Store.DB().QueryRowContext(r.Context(), "SELECT count(*) FROM profile WHERE template_id=?", id).Scan(&refs); err != nil {
		fail(w, 500, "database_error", "检查模板引用失败")
		return
	}
	if refs > 0 {
		writeJSON(w, 409, map[string]any{"error": map[string]any{
			"code": "in_use", "message": "此模板被订阅编排引用", "references": refs}})
		return
	}
	if _, err := s.Store.DB().ExecContext(r.Context(), "DELETE FROM template WHERE id=?", id); err != nil {
		fail(w, 500, "database_error", "删除模板失败")
		return
	}
	w.WriteHeader(204)
}

func (s *Server) validateTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	var content string
	if err := s.Store.DB().QueryRowContext(r.Context(), "SELECT content FROM template WHERE id=?", id).Scan(&content); err != nil {
		fail(w, 404, "not_found", "模板不存在")
		return
	}
	// Allow validating unsaved editor content.
	var in struct {
		Content string `json:"content"`
	}
	if decodeJSON(w, r, &in, 1<<20) == nil && strings.TrimSpace(in.Content) != "" {
		content = in.Content
	}
	issues, err := render.ValidateTemplate(content)
	if err != nil {
		fail(w, 500, "validate_failed", "校验失败")
		return
	}
	status := "ok"
	if len(issues) > 0 {
		status = issues[0].Message
	}
	_, _ = s.Store.DB().ExecContext(r.Context(), "UPDATE template SET last_check=? WHERE id=?", status, id)
	if issues == nil {
		issues = []render.Issue{}
	}
	writeJSON(w, 200, map[string]any{"ok": len(issues) == 0, "errors": issues, "lastCheck": nowRFC3339()})
}
