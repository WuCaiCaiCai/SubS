package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAirportRefreshIntervalValidation(t *testing.T) {
	c, _ := newAPI(t)

	// 太短的间隔应被拒绝
	c.do(http.MethodPost, "/api/airports", map[string]any{
		"name":               "bad-short",
		"url":                "https://example.com/sub",
		"refreshIntervalMin": 5,
	}, nil, http.StatusBadRequest)

	// 超过 7 天的间隔应被拒绝
	c.do(http.MethodPost, "/api/airports", map[string]any{
		"name":               "bad-long",
		"url":                "https://example.com/sub",
		"refreshIntervalMin": 20000,
	}, nil, http.StatusBadRequest)

	// 合法间隔应成功创建并回传
	raw := c.do(http.MethodPost, "/api/airports", map[string]any{
		"name":               "good",
		"url":                "https://example.com/sub",
		"refreshIntervalMin": 360,
	}, nil, http.StatusCreated)
	var created airportView
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatal(err)
	}
	if created.RefreshIntervalMin == nil || *created.RefreshIntervalMin != 360 {
		t.Fatalf("expected refreshIntervalMin=360, got %v", created.RefreshIntervalMin)
	}

	// 更新为仅手动（null）
	raw = c.do(http.MethodPut, "/api/airports/1", map[string]any{
		"name":               "good",
		"refreshIntervalMin": nil,
	}, nil, http.StatusOK)
	var updated airportView
	if err := json.Unmarshal(raw, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.RefreshIntervalMin != nil {
		t.Fatalf("expected refreshIntervalMin=nil, got %v", *updated.RefreshIntervalMin)
	}
}

func TestAutoRefreshPassAndBackoff(t *testing.T) {
	c, s := newAPI(t)

	const sampleSub = "vless://11111111-1111-1111-1111-111111111111@hk.example.com:443?security=tls&sni=hk.example.com#HK-01\n"
	fetchCalls := map[string]int{}
	failNext := false
	s.FetchAirport = func(_ context.Context, rawURL string) ([]byte, error) {
		fetchCalls[rawURL]++
		if failNext {
			return nil, errors.New("upstream timeout")
		}
		return []byte(sampleSub), nil
	}

	// 1. 启用且配置 60 分钟自动刷新
	c.do(http.MethodPost, "/api/airports", map[string]any{
		"name":               "auto-60",
		"url":                "https://example.com/a1",
		"refreshIntervalMin": 60,
	}, nil, http.StatusCreated)

	// 2. 仅手动刷新（null）
	c.do(http.MethodPost, "/api/airports", map[string]any{
		"name":               "manual-only",
		"url":                "https://example.com/a2",
		"refreshIntervalMin": nil,
	}, nil, http.StatusCreated)

	// 3. 配置了间隔但已停用
	c.do(http.MethodPost, "/api/airports", map[string]any{
		"name":               "disabled",
		"url":                "https://example.com/a3",
		"refreshIntervalMin": 60,
	}, nil, http.StatusCreated)
	c.do(http.MethodPut, "/api/airports/3", map[string]any{
		"name":               "disabled",
		"enabled":            false,
		"refreshIntervalMin": 60,
	}, nil, http.StatusOK)

	base := time.Now().UTC()
	// 首次巡检：a1 从未刷新过，应立即拉取；a2/a3 不应拉取
	s.runAutoRefreshPass(context.Background(), base)
	if fetchCalls["https://example.com/a1"] != 1 {
		t.Fatalf("expected a1 fetched once, got %d", fetchCalls["https://example.com/a1"])
	}
	if fetchCalls["https://example.com/a2"] != 0 || fetchCalls["https://example.com/a3"] != 0 {
		t.Fatalf("unexpected fetch for manual/disabled: %+v", fetchCalls)
	}

	// 30 分钟后再巡检：未满 60 分钟，不应拉取
	s.runAutoRefreshPass(context.Background(), base.Add(30*time.Minute))
	if fetchCalls["https://example.com/a1"] != 1 {
		t.Fatalf("expected no fetch before interval, got %d", fetchCalls["https://example.com/a1"])
	}

	// 65 分钟后巡检并模拟失败：应尝试拉取并记录 last_error
	failNext = true
	s.runAutoRefreshPass(context.Background(), base.Add(65*time.Minute))
	if fetchCalls["https://example.com/a1"] != 2 {
		t.Fatalf("expected 2nd fetch after 65m, got %d", fetchCalls["https://example.com/a1"])
	}

	// 失败后 10 分钟再巡检：应按间隔退避，不重复请求
	failNext = false
	s.runAutoRefreshPass(context.Background(), base.Add(75*time.Minute))
	if fetchCalls["https://example.com/a1"] != 2 {
		t.Fatalf("expected backoff after failure, got %d", fetchCalls["https://example.com/a1"])
	}

	// 再过 60 分钟（总计 130 分钟）：退避结束，恢复拉取成功
	s.runAutoRefreshPass(context.Background(), base.Add(130*time.Minute))
	if fetchCalls["https://example.com/a1"] != 3 {
		t.Fatalf("expected 3rd fetch after backoff, got %d", fetchCalls["https://example.com/a1"])
	}
}

func TestAirportRefreshAutoDeletesDisappearedNodes(t *testing.T) {
	c, s := newAPI(t)

	subRound1 := "vless://11111111-1111-1111-1111-111111111111@hk.example.com:443?security=tls&sni=hk.example.com#HK-01\n" +
		"vless://22222222-2222-2222-2222-222222222222@jp.example.com:443?security=tls&sni=jp.example.com#JP-01\n"
	subRound2 := "vless://11111111-1111-1111-1111-111111111111@hk.example.com:443?security=tls&sni=hk.example.com#HK-01\n"

	currentSub := subRound1
	s.FetchAirport = func(_ context.Context, _ string) ([]byte, error) {
		return []byte(currentSub), nil
	}

	c.do(http.MethodPost, "/api/airports", map[string]any{
		"name": "ap",
		"url":  "https://example.com/sub",
	}, nil, http.StatusCreated)
	c.do(http.MethodPost, "/api/airports/1/refresh", nil, nil, http.StatusOK)

	// 第二轮上游移除 JP-01，刷新应自动删除而不是保留 missing=1。
	currentSub = subRound2
	raw := c.do(http.MethodPost, "/api/airports/1/refresh", nil, nil, http.StatusOK)
	var stats struct {
		Matched int `json:"matched"`
		Missing int `json:"missing"`
	}
	if err := json.Unmarshal(raw, &stats); err != nil {
		t.Fatal(err)
	}
	if stats.Matched != 1 || stats.Missing != 1 {
		t.Fatalf("expected matched=1 missing=1, got %+v", stats)
	}

	nodesRaw := c.do(http.MethodGet, "/api/nodes", nil, nil, http.StatusOK)
	var nodes []nodeView
	if err := json.Unmarshal(nodesRaw, &nodes); err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].RawName != "HK-01" || nodes[0].IsNew {
		t.Fatalf("expected only HK-01 (isNew=false) left, got %+v", nodes)
	}
}

func TestSubscriptionOutputFollowsReorderedNodes(t *testing.T) {
	c, s := newAPI(t)
	s.FetchAirport = func(_ context.Context, _ string) ([]byte, error) {
		return []byte(
			"vless://11111111-1111-1111-1111-111111111111@hk.example.com:443?security=tls&sni=hk.example.com#HK-01\n" +
				"vless://22222222-2222-2222-2222-222222222222@jp.example.com:443?security=tls&sni=jp.example.com#JP-01\n",
		), nil
	}
	c.do(http.MethodPost, "/api/airports", map[string]any{"name": "ap", "url": "https://example.com/sub"}, nil, http.StatusCreated)
	c.do(http.MethodPost, "/api/airports/1/refresh", nil, nil, http.StatusOK)

	rawTpl := c.do(http.MethodPost, "/api/templates", map[string]any{
		"name":    "tpl",
		"content": "proxies: []\nproxy-groups:\n  - name: PROXY\n    type: select\n    proxies:\n      # subs:nodes\nrules:\n  - MATCH,PROXY\n",
	}, nil, http.StatusCreated)
	var tpl struct{ ID int64 }
	_ = json.Unmarshal(rawTpl, &tpl)

	rawProf := c.do(http.MethodPost, "/api/profiles", map[string]any{
		"name":       "p1",
		"templateId": tpl.ID,
		"namingRule": map[string]any{"segments": []string{"rawName"}, "separator": "-", "zeroPad": 2, "upperProtocol": true},
	}, nil, http.StatusCreated)
	var prof struct {
		ID    int64
		Token string
	}
	_ = json.Unmarshal(rawProf, &prof)

	// 按逆序保存节点（JP-01 在前，HK-01 在后）
	c.do(http.MethodPut, "/api/profiles/1/nodes", map[string]any{
		"items": []map[string]any{
			{"kind": "node", "refId": 2},
			{"kind": "node", "refId": 1},
		},
	}, nil, http.StatusOK)

	req := httptest.NewRequest(http.MethodGet, "/s/"+prof.Token+"?target=mihomo", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	jpPos := strings.Index(body, "JP 01")
	hkPos := strings.Index(body, "HK 01")
	if jpPos < 0 || hkPos < 0 || jpPos > hkPos {
		t.Fatalf("expected JP-01 before HK-01 in subscription output, got:\n%s", body)
	}
}

