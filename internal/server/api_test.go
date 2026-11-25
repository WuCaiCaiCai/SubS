package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wucai/subs/internal/profile"
	"github.com/wucai/subs/internal/protocol"
	"github.com/wucai/subs/internal/secrets"
	"github.com/wucai/subs/internal/store"
)

// apiClient drives the authenticated API with cookies and CSRF headers.
type apiClient struct {
	t       *testing.T
	h       http.Handler
	cookies []*http.Cookie
	csrf    string
}

func newAPI(t *testing.T) (*apiClient, *Server) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	box, err := secrets.LoadOrCreateBox(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Store: db, InitToken: "init-secret", Box: box, DataDir: dir, DBPath: filepath.Join(dir, "db.sqlite")}
	t.Cleanup(func() { _ = db.Close() })
	h := s.Router()

	c := &apiClient{t: t, h: h}
	c.do(http.MethodPost, "/api/auth/init", map[string]string{
		"username": "admin", "password": "correct horse battery staple", "initToken": "init-secret"}, nil, http.StatusOK)
	return c, s
}

func (c *apiClient) do(method, path string, body any, headers map[string]string, want int) []byte {
	c.t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	r := httptest.NewRequest(method, path, reader)
	r.Header.Set("Content-Type", "application/json")
	for _, ck := range c.cookies {
		r.AddCookie(ck)
	}
	if c.csrf != "" {
		r.Header.Set("X-CSRF-Token", c.csrf)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	for _, ck := range w.Result().Cookies() {
		c.absorb(ck)
	}
	if w.Code != want {
		c.t.Fatalf("%s %s: status=%d want=%d body=%s", method, path, w.Code, want, w.Body.String())
	}
	return w.Body.Bytes()
}

func (c *apiClient) absorb(ck *http.Cookie) {
	if ck.Name == "subs_csrf" {
		c.csrf = ck.Value
	}
	for i, old := range c.cookies {
		if old.Name == ck.Name {
			c.cookies[i] = ck
			return
		}
	}
	c.cookies = append(c.cookies, ck)
}

func TestAirportServerNodeFlow(t *testing.T) {
	c, _ := newAPI(t)

	// VPS group + imported node.
	c.do(http.MethodPost, "/api/servers", map[string]any{"name": "东京自建", "address": "jp.example.com"}, nil, 201)
	var imported struct {
		Parsed []struct {
			RawName  string          `json:"rawName"`
			Protocol string          `json:"protocol"`
			Config   protocol.Config `json:"config"`
		} `json:"parsed"`
		Saved int `json:"saved"`
	}
	body := c.do(http.MethodPost, "/api/nodes/import", map[string]any{
		"sourceId": 1,
		"commit":   true,
		"text": "hy2://pw@jp.example.com:443?sni=jp.example.com#日本 东京 01\n" +
			"vless://11111111-2222-3333-4444-555555555555@jp.example.com:8443?security=tls&sni=jp.example.com&type=ws&path=%2Fws#日本 东京 02\n" +
			"garbage-line\n",
	}, nil, 200)
	if err := json.Unmarshal(body, &imported); err != nil {
		t.Fatal(err)
	}
	if len(imported.Parsed) != 2 || imported.Saved != 2 {
		t.Fatalf("import: parsed=%d saved=%d", len(imported.Parsed), imported.Saved)
	}
	if imported.Parsed[0].Config.Host != "jp.example.com" {
		t.Fatalf("unexpected host: %+v", imported.Parsed[0])
	}

	// Re-import must not duplicate.
	c.do(http.MethodPost, "/api/nodes/import", map[string]any{
		"sourceId": 1, "commit": true,
		"text": "hy2://pw@jp.example.com:443?sni=jp.example.com#日本 东京 01",
	}, nil, 200)
	nodes := c.listNodes(t)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes after re-import, got %d", len(nodes))
	}

	// Template from the real reference file.
	raw, err := os.ReadFile("../../模板示例/Router.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var tmpl templateView
	json.Unmarshal(c.do(http.MethodPost, "/api/templates", map[string]any{
		"name": "Router", "content": string(raw)}, nil, 201), &tmpl)
	if tmpl.ID == 0 {
		t.Fatal("template not created")
	}
	var val struct {
		OK     bool          `json:"ok"`
		Errors []renderIssue `json:"errors"`
	}
	json.Unmarshal(c.do(http.MethodPost, "/api/templates/1/validate", nil, nil, 200), &val)
	if !val.OK {
		t.Fatalf("template validation failed: %+v", val.Errors)
	}

	// Profile + node ordering.
	var p profileView
	json.Unmarshal(c.do(http.MethodPost, "/api/profiles", map[string]any{
		"name": "主订阅", "templateId": 1,
		"namingRule": map[string]any{"segments": []string{"index", "region", "protocol", "rawName"}, "separator": "-", "zeroPad": 2, "upperProtocol": true},
	}, nil, 201), &p)
	if p.Token == "" {
		t.Fatal("profile token missing")
	}
	ids := []int64{}
	for _, n := range nodes {
		ids = append(ids, n.ID)
	}
	c.do(http.MethodPut, "/api/profiles/1/nodes", []map[string]any{
		{"nodeId": ids[0]}, {"nodeId": ids[1]},
	}, nil, 200)

	var naming struct {
		Nodes []profileNodeView `json:"nodes"`
	}
	json.Unmarshal(c.do(http.MethodGet, "/api/profiles/1/naming-preview", nil, nil, 200), &naming)
	if len(naming.Nodes) != 2 {
		t.Fatalf("naming preview nodes=%d", len(naming.Nodes))
	}
	if !strings.HasPrefix(naming.Nodes[0].OutputName, "01-JP-") {
		t.Fatalf("unexpected output name %q", naming.Nodes[0].OutputName)
	}

	var preview struct {
		Content string `json:"content"`
	}
	json.Unmarshal(c.do(http.MethodPost, "/api/profiles/1/preview", map[string]string{"target": "mihomo"}, nil, 200), &preview)
	for _, want := range []string{"jp.example.com", "proxy-groups:", "hysteria2"} {
		if !strings.Contains(preview.Content, want) {
			t.Errorf("preview missing %q", want)
		}
	}
}

func (c *apiClient) listNodes(t *testing.T) []nodeView {
	t.Helper()
	var nodes []nodeView
	json.Unmarshal(c.do(http.MethodGet, "/api/nodes", nil, nil, 200), &nodes)
	return nodes
}

type renderIssue struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

func TestSubscriptionNegotiatesByUA(t *testing.T) {
	c, _ := newAPI(t)
	c.do(http.MethodPost, "/api/servers", map[string]any{"name": "VPS"}, nil, 201)
	c.do(http.MethodPost, "/api/nodes/import", map[string]any{
		"sourceId": 1, "commit": true,
		"text": "hy2://pw@hk.example.com:443?sni=hk.example.com#香港01\n" +
			"mieru://user:pw@hk.example.com:9000?sni=hk.example.com#香港Mieru\n",
	}, nil, 200)
	raw, _ := os.ReadFile("../../模板示例/Client.yaml")
	c.do(http.MethodPost, "/api/templates", map[string]any{"name": "Client", "content": string(raw)}, nil, 201)
	var p profileView
	json.Unmarshal(c.do(http.MethodPost, "/api/profiles", map[string]any{"name": "P", "templateId": 1}, nil, 201), &p)
	nodes := c.listNodes(t)
	ids := []map[string]any{}
	for _, n := range nodes {
		ids = append(ids, map[string]any{"nodeId": n.ID})
	}
	c.do(http.MethodPut, "/api/profiles/1/nodes", ids, nil, 200)

	fetch := func(ua string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/s/"+p.Token, nil)
		r.Header.Set("User-Agent", ua)
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		return w
	}
	mihomo := fetch("clash-verge/1.0")
	if mihomo.Code != 200 || !strings.Contains(mihomo.Body.String(), "hysteria2") {
		t.Fatalf("mihomo body: %s", mihomo.Body.String())
	}
	singbox := fetch("sing-box 1.11")
	if singbox.Code != 200 || !strings.Contains(singbox.Body.String(), `"type": "hysteria2"`) {
		t.Fatalf("singbox body: %s", singbox.Body.String())
	}
	if !json.Valid(singbox.Body.Bytes()) {
		t.Error("sing-box body must be valid JSON (no comments)")
	}
	if got := singbox.Header().Get("X-Skipped-Nodes"); got == "" {
		t.Error("sing-box should report skipped mieru node via header")
	}
	loon := fetch("Loon/3.2")
	if loon.Code != 200 || !strings.Contains(loon.Body.String(), "[Proxy]") {
		t.Fatalf("loon body: %s", loon.Body.String())
	}
	if got := loon.Header().Get("X-Skipped-Nodes"); got == "" {
		t.Error("missing X-Skipped-Nodes header")
	}
	if cc := loon.Header().Get("Cache-Control"); cc != "private, no-store" {
		t.Errorf("cache-control=%q", cc)
	}

	// ?target= 覆盖 UA 协商，?download=1 切换为附件下载。
	override := func(query, ua string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/s/"+p.Token+query, nil)
		r.Header.Set("User-Agent", ua)
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		return w
	}
	sb := override("?target=sing-box&download=1", "clash-verge/1.0")
	if sb.Code != 200 || !json.Valid(sb.Body.Bytes()) {
		t.Fatalf("target override must beat UA: status=%d body=%s", sb.Code, sb.Body.String())
	}
	if ct := sb.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("override content-type=%q", ct)
	}
	if cd := sb.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, "subs.json") {
		t.Errorf("download disposition=%q", cd)
	}
	if bad := override("?target=bogus", "clash-verge/1.0"); bad.Code != 400 {
		t.Errorf("invalid target should 400, got %d", bad.Code)
	}

	// Unknown token must 404 identically.
	r := httptest.NewRequest(http.MethodGet, "/s/nonexistent", nil)
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	if w.Code != 404 || w.Body.Len() != 0 {
		t.Fatalf("unknown token: status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestBackupDownloadAndDestroyGuard(t *testing.T) {
	c, _ := newAPI(t)
	var bk struct {
		DownloadID string `json:"downloadId"`
		SHA256     string `json:"sha256"`
		SizeBytes  int    `json:"sizeBytes"`
	}
	json.Unmarshal(c.do(http.MethodPost, "/api/backup", map[string]string{
		"password": "backup password 123"}, nil, 201), &bk)
	if bk.SHA256 == "" || bk.SizeBytes == 0 {
		t.Fatal("backup metadata missing")
	}

	// Destroy without sha -> 409.
	c.do(http.MethodPost, "/api/destroy", map[string]string{"confirmPhrase": "my-subs"}, nil, 409)

	// Download the archive and verify the sha matches the header.
	r := httptest.NewRequest(http.MethodGet, "/api/backup/"+bk.DownloadID, nil)
	for _, ck := range c.cookies {
		r.AddCookie(ck)
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	if w.Code != 200 || w.Body.Len() != bk.SizeBytes {
		t.Fatalf("download status=%d size=%d want=%d", w.Code, w.Body.Len(), bk.SizeBytes)
	}
	if got := w.Header().Get("X-Backup-SHA256"); got != bk.SHA256 {
		t.Fatalf("sha header=%q want %q", got, bk.SHA256)
	}

	// Wrong phrase still rejected.
	c.do(http.MethodPost, "/api/destroy", map[string]string{
		"confirmPhrase": "nope", "backupSha256": bk.SHA256}, nil, 403)

	// Correct phrase + downloaded sha succeeds.
	c.do(http.MethodPost, "/api/destroy", map[string]string{
		"confirmPhrase": "my-subs", "backupSha256": bk.SHA256}, nil, 200)
}

func TestRestoreRoundTrip(t *testing.T) {
	c, s := newAPI(t)
	c.do(http.MethodPost, "/api/servers", map[string]any{"name": "待恢复 VPS"}, nil, 201)
	var bk struct {
		DownloadID string `json:"downloadId"`
	}
	json.Unmarshal(c.do(http.MethodPost, "/api/backup", map[string]string{
		"password": "backup password 123"}, nil, 201), &bk)

	r := httptest.NewRequest(http.MethodGet, "/api/backup/"+bk.DownloadID, nil)
	for _, ck := range c.cookies {
		r.AddCookie(ck)
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	archive := w.Body.Bytes()

	// Mutate state after backup.
	c.do(http.MethodPost, "/api/servers", map[string]any{"name": "备份后新增"}, nil, 201)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "backup.sbox")
	fw.Write(archive)
	mw.WriteField("password", "backup password 123")
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/restore", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for _, ck := range c.cookies {
		req.AddCookie(ck)
	}
	req.Header.Set("X-CSRF-Token", c.csrf)
	rw := httptest.NewRecorder()
	c.h.ServeHTTP(rw, req)
	if rw.Code != 200 {
		t.Fatalf("restore status=%d body=%s", rw.Code, rw.Body.String())
	}
	var servers []serverView
	json.Unmarshal(c.do(http.MethodGet, "/api/servers", nil, nil, 200), &servers)
	if len(servers) != 1 || servers[0].Name != "待恢复 VPS" {
		t.Fatalf("restore mismatch: %+v", servers)
	}
	_ = s
}

func TestDestroyRequiresInstanceName(t *testing.T) {
	c, _ := newAPI(t)
	c.do(http.MethodPost, "/api/destroy", map[string]any{"confirmPhrase": "my-subs", "dryRun": true}, nil, 200)
	c.do(http.MethodPost, "/api/destroy", map[string]any{"confirmPhrase": "other", "dryRun": true}, nil, 403)
}

func TestNamingRuleValidation(t *testing.T) {
	c, _ := newAPI(t)
	c.do(http.MethodPost, "/api/templates", map[string]any{"name": "T", "content": "proxies: []\n"}, nil, 201)
	c.do(http.MethodPost, "/api/profiles", map[string]any{
		"name": "bad", "templateId": 1,
		"namingRule": map[string]any{"segments": []string{}},
	}, nil, 400)
	c.do(http.MethodPost, "/api/profiles", map[string]any{
		"name": "bad2", "templateId": 1,
		"namingRule": map[string]any{"segments": []string{"nope"}},
	}, nil, 400)
}

var _ = profile.HashToken
