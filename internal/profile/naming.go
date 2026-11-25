// Package profile applies naming rules to nodes and produces output names.
package profile

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// NamingRule mirrors profile.naming_rule.
type NamingRule struct {
	Segments      []string `json:"segments"`
	Separator     string   `json:"separator"`
	ZeroPad       int      `json:"zeroPad"`
	UpperProtocol bool     `json:"upperProtocol"`
}

// DefaultRule is the fallback when a profile has no stored rule.
func DefaultRule() NamingRule {
	return NamingRule{Segments: []string{"index", "region", "protocol", "rawName"}, Separator: "-", ZeroPad: 2, UpperProtocol: true}
}

// ParseRule decodes and validates a naming rule JSON document.
func ParseRule(raw string) (NamingRule, error) {
	if strings.TrimSpace(raw) == "" {
		return DefaultRule(), nil
	}
	var r NamingRule
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return NamingRule{}, fmt.Errorf("命名规则不是合法 JSON")
	}
	if len(r.Segments) == 0 {
		return NamingRule{}, fmt.Errorf("命名规则至少需要一个段")
	}
	allowed := map[string]bool{"rawName": true, "source": true, "protocol": true, "index": true, "region": true}
	for _, s := range r.Segments {
		if !allowed[s] {
			return NamingRule{}, fmt.Errorf("未知命名段: %s", s)
		}
	}
	if r.Separator == "" {
		r.Separator = "-"
	}
	if r.ZeroPad <= 0 {
		r.ZeroPad = 2
	}
	return r, nil
}

// NodeInput is the minimal node view the naming engine needs.
type NodeInput struct {
	RawName  string
	Source   string
	Protocol string
	Override string
	Index    int
}

var separators = regexp.MustCompile(`[\s\-_|·•/\\]+`)

// CleanRawName strips emoji, collapses separators and trims the result.
func CleanRawName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if isEmoji(r) {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	out := separators.ReplaceAllString(b.String(), " ")
	out = strings.Join(strings.Fields(out), " ")
	return strings.TrimSpace(out)
}

func isEmoji(r rune) bool {
	return (r >= 0x1F300 && r <= 0x1FAFF) ||
		(r >= 0x1F000 && r <= 0x1F2FF) ||
		(r >= 0x2600 && r <= 0x27BF) ||
		(r >= 0xFE00 && r <= 0xFE0F) ||
		r == 0x200D || r == 0x20E3
}

// Apply evaluates the rule for one node, returning the raw (pre-dedup) name.
func (r NamingRule) Apply(n NodeInput) string {
	if strings.TrimSpace(n.Override) != "" {
		return truncate(n.Override, 64)
	}
	values := map[string]string{
		"rawName":  CleanRawName(n.RawName),
		"source":   CleanRawName(n.Source),
		"protocol": n.Protocol,
		"index":    padIndex(n.Index, r.ZeroPad),
		"region":   Region(n.RawName),
	}
	if r.UpperProtocol {
		values["protocol"] = strings.ToUpper(values["protocol"])
	}
	parts := make([]string, 0, len(r.Segments))
	for _, seg := range r.Segments {
		v := strings.TrimSpace(values[seg])
		if v == "" {
			continue
		}
		parts = append(parts, v)
	}
	return truncate(strings.Join(parts, r.Separator), 64)
}

// Dedupe appends -2/-3 suffixes so that all names are unique.
func Dedupe(names []string) []string {
	seen := map[string]int{}
	out := make([]string, len(names))
	for i, name := range names {
		if _, ok := seen[name]; !ok {
			seen[name] = 1
			out[i] = name
			continue
		}
		for {
			seen[name]++
			candidate := truncate(fmt.Sprintf("%s-%d", name, seen[name]), 64)
			if _, exists := seen[candidate]; !exists {
				seen[candidate] = 1
				out[i] = candidate
				break
			}
		}
	}
	return out
}

func padIndex(i, width int) string {
	if i < 1 {
		i = 1
	}
	return fmt.Sprintf("%0*d", width, i)
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

var regions = []struct {
	words []string
	code  string
}{
	{[]string{"香港", "hongkong", "hong kong", "hk"}, "HK"},
	{[]string{"台湾", "台灣", "taiwan", "tw"}, "TW"},
	{[]string{"日本", "japan", "jp", "tokyo", "osaka"}, "JP"},
	{[]string{"新加坡", "狮城", "singapore", "sg"}, "SG"},
	{[]string{"美国", "united states", "usa", "us", "los angeles", "seattle"}, "US"},
	{[]string{"韩国", "韓國", "korea", "kr", "seoul"}, "KR"},
	{[]string{"德国", "德國", "germany", "de", "frankfurt"}, "DE"},
	{[]string{"英国", "英國", "united kingdom", "uk", "gb", "london"}, "GB"},
	{[]string{"法国", "法國", "france", "fr", "paris"}, "FR"},
	{[]string{"荷兰", "荷蘭", "netherlands", "nl", "amsterdam"}, "NL"},
	{[]string{"加拿大", "canada", "ca", "toronto"}, "CA"},
	{[]string{"澳大利亚", "澳洲", "australia", "au", "sydney"}, "AU"},
	{[]string{"土耳其", "turkey", "tr", "istanbul"}, "TR"},
	{[]string{"印度", "india", "in", "mumbai"}, "IN"},
	{[]string{"巴西", "brazil", "br", "sao paulo"}, "BR"},
}

// Region maps a raw node name to a region code, longest match first.
func Region(rawName string) string {
	lower := strings.ToLower(rawName)
	best, bestLen := "", 0
	for _, r := range regions {
		for _, w := range r.words {
			if strings.Contains(lower, w) && len(w) > bestLen {
				best, bestLen = r.code, len(w)
			}
		}
	}
	return best
}

// NewToken returns a URL-safe subscription token.
func NewToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken produces the lookup hash stored in profile.token_hash.
func HashToken(token string) string {
	sum := sha256Bytes([]byte(token))
	return hex.EncodeToString(sum)
}

// RedactURL masks credentials inside a subscription URL for display.
func RedactURL(raw string) string {
	if raw == "" {
		return ""
	}
	if i := strings.Index(raw, "://"); i >= 0 {
		rest := raw[i+3:]
		if at := strings.Index(rest, "@"); at >= 0 {
			return raw[:i+3] + "***@" + rest[at+1:]
		}
	}
	return raw
}

func sha256Bytes(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}
