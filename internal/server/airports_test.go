package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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
