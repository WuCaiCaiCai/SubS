package profile

import "testing"

func TestCleanRawName(t *testing.T) {
	cases := map[string]string{
		"🇭🇰 香港 01 | IPLC":  "香港 01 IPLC",
		"  JP-Tokyo__02  ": "JP Tokyo 02",
		"美国 · 洛杉矶":         "美国 洛杉矶",
	}
	for in, want := range cases {
		if got := CleanRawName(in); got != want {
			t.Errorf("CleanRawName(%q)=%q want %q", in, got, want)
		}
	}
}

func TestRegionMapping(t *testing.T) {
	cases := map[string]string{
		"香港 IPLC 01":        "HK",
		"🇭🇰HKG 01":          "HK",
		"🇹🇼TWN 01":          "TW",
		"🇸🇬SGP 01":          "SG",
		"🇺🇸USA.LAX 01":      "US",
		"🇯🇵 Tokyo Premium":  "JP",
		"🇯🇵 01":             "JP",
		"新加坡 狮城":            "SG",
		"Los Angeles US":    "US",
		"Frankfurt DE":      "DE",
		"未知地区节点":            "",
		"Node 01":           "",
		"Free Cloud Plus":   "",
		"Online Test 02":    "",
		"Unlimited Traffic": "",
	}
	for in, want := range cases {
		if got := Region(in); got != want {
			t.Errorf("Region(%q)=%q want %q", in, got, want)
		}
	}
}

func TestApplyRule(t *testing.T) {
	r := DefaultRule()
	got := r.Apply(NodeInput{RawName: "🇭🇰 香港 01", Source: "MyAirport", Protocol: "hy2", Index: 3})
	want := "03-HK-HY2-香港 01"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	rWithFlagAndSource, err := ParseRule(`{"segments":["flag","source","region","rawName"],"separator":"-","zeroPad":2,"upperProtocol":true}`)
	if err != nil {
		t.Fatal(err)
	}
	got2 := rWithFlagAndSource.Apply(NodeInput{RawName: "香港 01", Source: "主力机场", Protocol: "vless", Index: 1})
	if got2 != "🇭🇰-主力机场-HK-香港 01" {
		t.Fatalf("got %q", got2)
	}
	if RegionFlag("未知节点") != "" {
		t.Fatalf("expected empty flag for unknown region")
	}
}

func TestOverrideAndEmptySegments(t *testing.T) {
	r := DefaultRule()
	if got := r.Apply(NodeInput{RawName: "x", Protocol: "vless", Override: "自定义名"}); got != "自定义名" {
		t.Fatalf("override ignored: %q", got)
	}
	// Region empty -> segment dropped, no double separator.
	got := r.Apply(NodeInput{RawName: "无名节点", Protocol: "tuic", Index: 1})
	if got != "01-TUIC-无名节点" {
		t.Fatalf("got %q", got)
	}
}

func TestDedupe(t *testing.T) {
	got := Dedupe([]string{"a", "a", "a", "b"})
	want := []string{"a", "a-2", "a-3", "b"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestParseRuleValidation(t *testing.T) {
	if _, err := ParseRule(`{"segments":[]}`); err == nil {
		t.Error("empty segments should fail")
	}
	if _, err := ParseRule(`{"segments":["bogus"]}`); err == nil {
		t.Error("unknown segment should fail")
	}
	r, err := ParseRule(`{"segments":["index"],"zeroPad":3}`)
	if err != nil || r.ZeroPad != 3 || r.Separator != "-" {
		t.Fatalf("defaults not applied: %+v %v", r, err)
	}
}

func TestTokenRoundTrip(t *testing.T) {
	tok, err := NewToken()
	if err != nil || len(tok) < 20 {
		t.Fatalf("bad token %q %v", tok, err)
	}
	if HashToken(tok) == tok {
		t.Fatal("hash must not equal token")
	}
}

func TestRedactURL(t *testing.T) {
	got := RedactURL("https://user:pass@example.com/sub?token=abc")
	if got != "https://***@example.com/sub?token=abc" {
		t.Fatalf("got %q", got)
	}
}
