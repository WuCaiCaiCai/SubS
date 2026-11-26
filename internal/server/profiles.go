package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/WuCaiCaiCai/SubS/internal/profile"
	"github.com/WuCaiCaiCai/SubS/internal/protocol"
	"github.com/WuCaiCaiCai/SubS/internal/render"
)

type profileView struct {
	ID           int64           `json:"id"`
	Name         string          `json:"name"`
	Note         string          `json:"note"`
	TemplateID   int64           `json:"templateId"`
	TemplateName string          `json:"templateName"`
	Enabled      bool            `json:"enabled"`
	NamingRule   json.RawMessage `json:"namingRule"`
	Token        string          `json:"token"`
	LastFetchAt  string          `json:"lastFetchAt"`
	NodeCount    int64           `json:"nodeCount"`
	// URL is the ready-to-paste subscription link, derived from instance.external_url.
	URL string `json:"url"`
	// Items 只在单项详情里返回，列表接口留空。
	Items []itemView `json:"items,omitempty"`
}

// itemView 是编排选择项（节点或整源）的管理视图。
type itemView struct {
	Kind         string `json:"kind"`
	RefID        int64  `json:"refId"`
	SourceType   string `json:"sourceType"`
	NameOverride string `json:"nameOverride"`
	SortOrder    int64  `json:"sortOrder"`
	Label        string `json:"label"`
	Protocol     string `json:"protocol"`
	NodeCount    int64  `json:"nodeCount"`
}

type profileNodeView struct {
	NodeID       int64  `json:"nodeId"`
	NameOverride string `json:"nameOverride"`
	SortOrder    int64  `json:"sortOrder"`
	RawName      string `json:"rawName"`
	Protocol     string `json:"protocol"`
	SourceType   string `json:"sourceType"`
	SourceID     int64  `json:"sourceId"`
	SourceName   string `json:"sourceName"`
	OutputName   string `json:"outputName"`
}

func (s *Server) instanceURL() string {
	var external sql.NullString
	_ = s.Store.DB().QueryRow("SELECT external_url FROM instance WHERE id=1").Scan(&external)
	return strings.TrimSuffix(external.String, "/")
}

func (s *Server) loadProfile(r *http.Request, id int64) (profileView, error) {
	var v profileView
	var note, lastFetch, rule, tokenEnc sql.NullString
	var enabled int
	err := s.Store.DB().QueryRowContext(r.Context(),
		`SELECT p.id,p.name,COALESCE(p.note,''),p.template_id,COALESCE(t.name,''),p.enabled,
		        p.naming_rule,p.secret_token,COALESCE(p.last_fetch_at,''),
		        (SELECT count(*) FROM profile_item i JOIN node n
		              ON (i.kind='node' AND n.id=i.ref_id)
		              OR (i.kind='source' AND n.source_type=i.source_type AND n.source_id=i.ref_id)
		          WHERE i.profile_id=p.id AND n.enabled=1 AND n.missing=0
		            AND (i.kind='node' OR NOT EXISTS (
		                  SELECT 1 FROM profile_item x
		                  WHERE x.profile_id=i.profile_id AND x.kind='node' AND x.ref_id=n.id)))
		 FROM profile p LEFT JOIN template t ON t.id=p.template_id WHERE p.id=?`, id).
		Scan(&v.ID, &v.Name, &note, &v.TemplateID, &v.TemplateName, &enabled, &rule, &tokenEnc, &lastFetch, &v.NodeCount)
	if err != nil {
		return v, err
	}
	v.Note = note.String
	v.Enabled = enabled != 0
	v.LastFetchAt = lastFetch.String
	if rule.Valid {
		v.NamingRule = json.RawMessage(rule.String)
	}
	if v.NamingRule == nil {
		def, _ := json.Marshal(profile.DefaultRule())
		v.NamingRule = def
	}
	if tokenEnc.Valid {
		if plain, err := s.decrypt(tokenEnc.String); err == nil {
			v.Token = plain
			if base := s.instanceURL(); base != "" {
				v.URL = base + "/s/" + plain
			}
		}
	}
	return v, nil
}

func (s *Server) listProfiles(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.DB().QueryContext(r.Context(), "SELECT id FROM profile ORDER BY id DESC")
	if err != nil {
		fail(w, 500, "database_error", "读取编排列表失败")
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
	out := make([]profileView, 0, len(ids))
	for _, id := range ids {
		v, err := s.loadProfile(r, id)
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}

func (s *Server) createProfile(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name       string          `json:"name"`
		Note       string          `json:"note"`
		TemplateID int64           `json:"templateId"`
		NamingRule json.RawMessage `json:"namingRule"`
	}
	if decodeJSON(w, r, &in, 64<<10) != nil || strings.TrimSpace(in.Name) == "" {
		fail(w, 400, "invalid_input", "编排名称不能为空")
		return
	}
	if in.TemplateID < 1 {
		fail(w, 400, "invalid_input", "请选择模板")
		return
	}
	var exists int
	if err := s.Store.DB().QueryRowContext(r.Context(), "SELECT count(*) FROM template WHERE id=?", in.TemplateID).Scan(&exists); err != nil || exists == 0 {
		fail(w, 404, "template_not_found", "模板不存在")
		return
	}
	rule, err := profile.ParseRule(string(in.NamingRule))
	if err != nil {
		fail(w, 400, "invalid_naming_rule", err.Error())
		return
	}
	ruleJSON, _ := json.Marshal(rule)
	token, err := profile.NewToken()
	if err != nil {
		fail(w, 500, "token_failed", "生成订阅令牌失败")
		return
	}
	enc, err := s.encrypt(token)
	if err != nil {
		fail(w, 500, "encrypt_failed", "保存令牌失败")
		return
	}
	res, err := s.Store.DB().ExecContext(r.Context(),
		`INSERT INTO profile(name,note,template_id,naming_rule,secret_token,token_hash)
		 VALUES(?,?,?,?,?,?)`,
		strings.TrimSpace(in.Name), nullString(in.Note), in.TemplateID, string(ruleJSON), enc, profile.HashToken(token))
	if err != nil {
		fail(w, 500, "database_error", "创建编排失败")
		return
	}
	id, _ := res.LastInsertId()
	v, err := s.loadProfile(r, id)
	if err != nil {
		fail(w, 500, "database_error", "读取编排失败")
		return
	}
	writeJSON(w, 201, v)
}

func (s *Server) getProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	v, err := s.loadProfile(r, id)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "not_found", "编排不存在")
		return
	}
	if err != nil {
		fail(w, 500, "database_error", "读取编排失败")
		return
	}
	if items, err := s.profileItems(r, id); err == nil {
		v.Items = items
	}
	writeJSON(w, 200, v)
}

func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	var in struct {
		Name       string          `json:"name"`
		Note       string          `json:"note"`
		TemplateID int64           `json:"templateId"`
		Enabled    *bool           `json:"enabled"`
		NamingRule json.RawMessage `json:"namingRule"`
	}
	if decodeJSON(w, r, &in, 64<<10) != nil {
		fail(w, 400, "invalid_input", "请求体格式错误")
		return
	}
	// Partial updates are allowed: empty name/template keeps the current value.
	var curTemplate int64
	var curName string
	if err := s.Store.DB().QueryRowContext(r.Context(),
		"SELECT template_id,name FROM profile WHERE id=?", id).Scan(&curTemplate, &curName); err != nil {
		fail(w, 404, "not_found", "编排不存在")
		return
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = curName
	}
	templateID := in.TemplateID
	if templateID < 1 {
		templateID = curTemplate
	}
	var exists int
	if err := s.Store.DB().QueryRowContext(r.Context(), "SELECT count(*) FROM template WHERE id=?", templateID).Scan(&exists); err != nil || exists == 0 {
		fail(w, 404, "template_not_found", "模板不存在")
		return
	}
	rule, err := profile.ParseRule(string(in.NamingRule))
	if err != nil {
		fail(w, 400, "invalid_naming_rule", err.Error())
		return
	}
	ruleJSON, _ := json.Marshal(rule)
	enabled := 1
	if in.Enabled != nil {
		if !*in.Enabled {
			enabled = 0
		}
	} else {
		_ = s.Store.DB().QueryRowContext(r.Context(), "SELECT enabled FROM profile WHERE id=?", id).Scan(&enabled)
	}
	if _, err := s.Store.DB().ExecContext(r.Context(),
		"UPDATE profile SET name=?,note=?,template_id=?,enabled=?,naming_rule=? WHERE id=?",
		name, nullString(in.Note), templateID, enabled, string(ruleJSON), id); err != nil {
		fail(w, 500, "database_error", "更新编排失败")
		return
	}
	v, err := s.loadProfile(r, id)
	if err != nil {
		fail(w, 500, "database_error", "读取编排失败")
		return
	}
	writeJSON(w, 200, v)
}

func (s *Server) deleteProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	err := s.Store.Write(r.Context(), func(tx *sql.Tx) error {
		for _, q := range []string{
			"DELETE FROM profile_item WHERE profile_id=?",
			"DELETE FROM profile_node WHERE profile_id=?",
		} {
			if _, err := tx.ExecContext(r.Context(), q, id); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(r.Context(), "DELETE FROM profile WHERE id=?", id)
		return err
	})
	if err != nil {
		fail(w, 500, "database_error", "删除编排失败")
		return
	}
	w.WriteHeader(204)
}

func (s *Server) setProfileNodes(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	items, ok := decodeProfileItems(w, r)
	if !ok {
		return
	}
	err := s.Store.Write(r.Context(), func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(r.Context(), "SELECT count(*) FROM profile WHERE id=?", id).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return sql.ErrNoRows
		}
		if _, err := tx.ExecContext(r.Context(), "DELETE FROM profile_item WHERE profile_id=?", id); err != nil {
			return err
		}
		seen := map[string]bool{}
		for i, item := range items {
			key := fmt.Sprintf("%s:%s:%d", item.Kind, item.SourceType, item.RefID)
			if seen[key] {
				continue
			}
			seen[key] = true
			if err := checkItemTarget(tx, r.Context(), item); err != nil {
				return err
			}
			if _, err := tx.ExecContext(r.Context(),
				`INSERT INTO profile_item(profile_id,kind,ref_id,source_type,sort_order,name_override)
				 VALUES(?,?,?,?,?,?)`,
				id, item.Kind, item.RefID, item.SourceType, i, nullString(item.NameOverride)); err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "not_found", "编排不存在")
		return
	}
	if errors.Is(err, errBadItem) {
		fail(w, 400, "invalid_input", "节点或来源不存在")
		return
	}
	if err != nil {
		fail(w, 500, "database_error", "保存节点顺序失败")
		return
	}
	nodes, err := s.profileNodes(r, id)
	if err != nil {
		fail(w, 500, "database_error", "读取编排节点失败")
		return
	}
	writeJSON(w, 200, nodes)
}

var errBadItem = errors.New("unknown selection item")

// itemInput 是编排选择的一项：单个节点或整个来源。
type itemInput struct {
	Kind         string `json:"kind"`
	RefID        int64  `json:"refId"`
	NodeID       int64  `json:"nodeId"`
	SourceID     int64  `json:"sourceId"`
	SourceType   string `json:"sourceType"`
	NameOverride string `json:"nameOverride"`
}

// normalise 把三种可接受的写法统一成 node/source + refId：
//  1. {"items":[{"kind":"node","refId":1}, {"kind":"source","refId":2,"sourceType":"airport"}]} —— 规范形态，保留顺序
//  2. {"nodes":[{"nodeId":1}], "sources":[{"sourceId":2,"sourceType":"airport"}]} —— 分桶形态
//  3. [{"nodeId":1},{"nodeId":2}] —— 旧版裸数组
func (in itemInput) normalise() (itemInput, bool) {
	out := in
	if out.Kind == "" {
		switch {
		case out.NodeID > 0:
			out.Kind = "node"
		case out.SourceID > 0:
			out.Kind = "source"
		}
	}
	switch out.Kind {
	case "node":
		if out.RefID == 0 {
			out.RefID = out.NodeID
		}
		out.SourceType = ""
		return out, out.RefID > 0
	case "source":
		if out.RefID == 0 {
			out.RefID = out.SourceID
		}
		if out.SourceType != "airport" && out.SourceType != "server" {
			return out, false
		}
		return out, out.RefID > 0
	default:
		return out, false
	}
}

func decodeProfileItems(w http.ResponseWriter, r *http.Request) ([]itemInput, bool) {
	var raw json.RawMessage
	if decodeJSON(w, r, &raw, 512<<10) != nil {
		fail(w, 400, "invalid_input", "请求体格式错误")
		return nil, false
	}
	var wrapped struct {
		Items   []itemInput `json:"items"`
		Nodes   []itemInput `json:"nodes"`
		Sources []itemInput `json:"sources"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && len(wrapped.Items)+len(wrapped.Nodes)+len(wrapped.Sources) > 0 {
		rawItems := append(append([]itemInput{}, wrapped.Items...), wrapped.Nodes...)
		rawItems = append(rawItems, wrapped.Sources...)
		return normaliseItems(w, rawItems)
	}
	var legacy []itemInput
	if err := json.Unmarshal(raw, &legacy); err != nil {
		fail(w, 400, "invalid_input", "请求体格式错误")
		return nil, false
	}
	return normaliseItems(w, legacy)
}

func normaliseItems(w http.ResponseWriter, in []itemInput) ([]itemInput, bool) {
	if len(in) > 5000 {
		fail(w, 400, "invalid_input", "单个编排最多 5000 项")
		return nil, false
	}
	out := make([]itemInput, 0, len(in))
	for _, item := range in {
		norm, ok := item.normalise()
		if !ok {
			continue
		}
		out = append(out, norm)
	}
	return out, true
}

// checkItemTarget rejects references to nodes or sources that do not exist.
func checkItemTarget(tx *sql.Tx, ctx context.Context, item itemInput) error {
	var q string
	switch {
	case item.Kind == "node":
		q = "SELECT count(*) FROM node WHERE id=?"
	case item.SourceType == "airport":
		q = "SELECT count(*) FROM airport WHERE id=?"
	default:
		q = "SELECT count(*) FROM server WHERE id=?"
	}
	var n int
	if err := tx.QueryRowContext(ctx, q, item.RefID).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return errBadItem
	}
	return nil
}

// itemSelectPath expands one profile_item into its node rows. Explicit node
// selections win over source expansion so a node never appears twice.
const itemSelectPath = `JOIN node n
	 ON (i.kind='node' AND n.id=i.ref_id)
	 OR (i.kind='source' AND n.source_type=i.source_type AND n.source_id=i.ref_id)`

const itemVisible = `i.profile_id=? AND n.enabled=1 AND n.missing=0
	AND (i.kind='node' OR NOT EXISTS (
	      SELECT 1 FROM profile_item x
	      WHERE x.profile_id=i.profile_id AND x.kind='node' AND x.ref_id=n.id))`

const sourceNameExpr = `CASE WHEN n.source_type='airport'
	   THEN COALESCE((SELECT a.name FROM airport a WHERE a.id=n.source_id),'')
	   ELSE COALESCE((SELECT sv.name FROM server sv WHERE sv.id=n.source_id),'') END`

// profileItems returns the raw selection rows (sources stay collapsed).
func (s *Server) profileItems(r *http.Request, profileID int64) ([]itemView, error) {
	rows, err := s.Store.DB().QueryContext(r.Context(),
		`SELECT i.kind,i.ref_id,i.source_type,COALESCE(i.name_override,''),i.sort_order,
		        CASE i.kind WHEN 'node' THEN COALESCE((SELECT n.raw_name FROM node n WHERE n.id=i.ref_id),'')
		                    WHEN 'airport' THEN COALESCE((SELECT a.name FROM airport a WHERE a.id=i.ref_id),'')
		                    ELSE COALESCE((SELECT sv.name FROM server sv WHERE sv.id=i.ref_id),'') END,
		        CASE i.kind WHEN 'node' THEN COALESCE((SELECT n.protocol FROM node n WHERE n.id=i.ref_id),'')
		                    ELSE '' END,
		        CASE i.kind WHEN 'source' THEN (
		              SELECT count(*) FROM node n WHERE n.source_type=i.source_type AND n.source_id=i.ref_id
		                AND n.enabled=1 AND n.missing=0)
		            ELSE 1 END
		 FROM profile_item i WHERE i.profile_id=? ORDER BY i.sort_order`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []itemView{}
	for rows.Next() {
		var v itemView
		if err := rows.Scan(&v.Kind, &v.RefID, &v.SourceType, &v.NameOverride, &v.SortOrder,
			&v.Label, &v.Protocol, &v.NodeCount); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// profileNodes returns the ordered node list with computed output names.
func (s *Server) profileNodes(r *http.Request, profileID int64) ([]profileNodeView, error) {
	var ruleRaw string
	if err := s.Store.DB().QueryRowContext(r.Context(),
		"SELECT naming_rule FROM profile WHERE id=?", profileID).Scan(&ruleRaw); err != nil {
		return nil, err
	}
	rule, err := profile.ParseRule(ruleRaw)
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.DB().QueryContext(r.Context(),
		`SELECT n.id, COALESCE(i.name_override,''), i.sort_order, n.raw_name, n.protocol,
		        n.source_type, n.source_id, `+sourceNameExpr+`
		 FROM profile_item i `+itemSelectPath+`
		 WHERE `+itemVisible+`
		 ORDER BY i.sort_order, n.id`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []profileNodeView
	var inputs []profile.NodeInput
	for rows.Next() {
		var v profileNodeView
		if err := rows.Scan(&v.NodeID, &v.NameOverride, &v.SortOrder, &v.RawName, &v.Protocol,
			&v.SourceType, &v.SourceID, &v.SourceName); err != nil {
			return nil, err
		}
		inputs = append(inputs, profile.NodeInput{
			RawName: v.RawName, Source: v.SourceName, Protocol: v.Protocol,
			Override: v.NameOverride, Index: len(inputs) + 1})
		out = append(out, v)
	}
	names := make([]string, len(inputs))
	for i, in := range inputs {
		names[i] = rule.Apply(in)
	}
	for i := range out {
		out[i].OutputName = names[i]
	}
	return out, nil
}

// renderProfile loads nodes for a profile and renders the requested target.
func (s *Server) renderProfile(r *http.Request, profileID int64, target string) (render.Result, error) {
	var templateContent string
	var ruleRaw string
	err := s.Store.DB().QueryRowContext(r.Context(),
		`SELECT COALESCE(t.content,''), p.naming_rule FROM profile p
		 LEFT JOIN template t ON t.id=p.template_id WHERE p.id=?`, profileID).Scan(&templateContent, &ruleRaw)
	if err != nil {
		return render.Result{}, err
	}
	rule, err := profile.ParseRule(ruleRaw)
	if err != nil {
		return render.Result{}, err
	}
	rows, err := s.Store.DB().QueryContext(r.Context(),
		`SELECT n.raw_name, n.protocol, n.config, COALESCE(i.name_override,''), `+sourceNameExpr+`
		 FROM profile_item i `+itemSelectPath+`
		 WHERE `+itemVisible+`
		 ORDER BY i.sort_order, n.id`, profileID)
	if err != nil {
		return render.Result{}, err
	}
	defer rows.Close()
	var nodes []render.Node
	var inputs []profile.NodeInput
	var configs []protocol.Config
	for rows.Next() {
		var rawName, proto, cfgJSON, override, source string
		if err := rows.Scan(&rawName, &proto, &cfgJSON, &override, &source); err != nil {
			return render.Result{}, err
		}
		var cfg protocol.Config
		if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
			continue
		}
		inputs = append(inputs, profile.NodeInput{RawName: rawName, Source: source, Protocol: proto, Override: override, Index: len(inputs) + 1})
		configs = append(configs, cfg)
	}
	raw := make([]string, len(inputs))
	for i, in := range inputs {
		raw[i] = rule.Apply(in)
	}
	names := profile.Dedupe(raw)
	for i := range names {
		nodes = append(nodes, render.Node{Name: names[i], Config: configs[i]})
	}
	return render.Render(target, templateContent, nodes)
}

func (s *Server) previewProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	var in struct {
		Target string `json:"target"`
	}
	_ = decodeJSON(w, r, &in, 8<<10)
	target := in.Target
	if target == "" {
		target = render.TargetMihomo
	}
	if target != render.TargetMihomo && target != render.TargetSingBox && target != render.TargetLoon {
		fail(w, 400, "invalid_target", "未知输出目标")
		return
	}
	res, err := s.renderProfile(r, id, target)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "not_found", "编排不存在")
		return
	}
	if err != nil {
		fail(w, 422, "render_failed", "生成配置失败: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"target": target, "content": res.Body, "skipped": nonNil(res.Skipped)})
}

func (s *Server) rotateToken(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	token, err := profile.NewToken()
	if err != nil {
		fail(w, 500, "token_failed", "生成订阅令牌失败")
		return
	}
	enc, err := s.encrypt(token)
	if err != nil {
		fail(w, 500, "encrypt_failed", "保存令牌失败")
		return
	}
	res, err := s.Store.DB().ExecContext(r.Context(),
		"UPDATE profile SET secret_token=?,token_hash=? WHERE id=?", enc, profile.HashToken(token), id)
	if err != nil {
		fail(w, 500, "database_error", "更新令牌失败")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fail(w, 404, "not_found", "编排不存在")
		return
	}
	v, err := s.loadProfile(r, id)
	if err != nil {
		fail(w, 500, "database_error", "读取编排失败")
		return
	}
	writeJSON(w, 200, v)
}

func (s *Server) namingPreview(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	nodes, err := s.profileNodes(r, id)
	if err != nil {
		fail(w, 500, "database_error", "读取编排节点失败")
		return
	}
	// Surface duplicates so the UI can explain auto-suffixing.
	counts := map[string]int{}
	for _, n := range nodes {
		counts[n.OutputName]++
	}
	dupes := []string{}
	for name, c := range counts {
		if c > 1 {
			dupes = append(dupes, name)
		}
	}
	writeJSON(w, 200, map[string]any{"nodes": nodes, "duplicates": dupes})
}

var _ = fmt.Sprintf

func (s *Server) listProfileItems(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	items, err := s.profileItems(r, id)
	if err != nil {
		fail(w, 500, "database_error", "读取编排选择失败")
		return
	}
	writeJSON(w, 200, items)
}
