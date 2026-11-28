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
	allowed := map[string]bool{"rawName": true, "source": true, "protocol": true, "index": true, "region": true, "flag": true}
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
		"flag":     RegionFlag(n.RawName),
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
	{[]string{"香港", "hongkong", "hong kong", "hk", "hkg", "深港", "沪港", "京港", "广港"}, "HK"},
	{[]string{"澳门", "澳門", "macau", "macao", "mo", "mfm"}, "MO"},
	{[]string{"台湾", "台灣", "台北", "新北", "彰化", "高雄", "台中", "taiwan", "taipei", "tw", "twn", "tpe"}, "TW"},
	{[]string{"日本", "东京", "東京", "大阪", "埼玉", "名古屋", "福冈", "沪日", "深日", "京日", "广日", "japan", "tokyo", "osaka", "saitama", "jp", "jpn", "tyo", "osa", "nrt"}, "JP"},
	{[]string{"新加坡", "狮城", "獅城", "坡县", "singapore", "sg", "sgp", "sin"}, "SG"},
	{[]string{"美国", "美國", "洛杉矶", "西雅图", "圣何塞", "硅谷", "达拉斯", "纽约", "芝加哥", "迈阿密", "亚特兰大", "阿什本", "凤凰城", "弗里蒙特", "united states", "america", "los angeles", "seattle", "san jose", "silicon valley", "dallas", "new york", "chicago", "miami", "atlanta", "ashburn", "phoenix", "fremont", "usa", "us", "lax", "sjc", "sea", "nyc", "dfw", "ord", "iad", "mia", "atl"}, "US"},
	{[]string{"韩国", "韓國", "首尔", "首爾", "釜山", "春川", "korea", "seoul", "busan", "kr", "kor", "icn", "sel"}, "KR"},
	{[]string{"德国", "德國", "法兰克福", "柏林", "慕尼黑", "纽伦堡", "germany", "frankfurt", "berlin", "munich", "de", "deu", "fra", "ber", "muc"}, "DE"},
	{[]string{"英国", "英國", "伦敦", "曼彻斯特", "united kingdom", "britain", "england", "london", "manchester", "uk", "gb", "gbr", "lon", "lhr"}, "GB"},
	{[]string{"法国", "法國", "巴黎", "马赛", "france", "paris", "marseille", "fr", "par", "cdg"}, "FR"},
	{[]string{"荷兰", "荷蘭", "阿姆斯特丹", "netherlands", "holland", "amsterdam", "nl", "nld", "ams"}, "NL"},
	{[]string{"加拿大", "多伦多", "温哥华", "蒙特利尔", "canada", "toronto", "vancouver", "montreal", "ca", "can", "yyz", "yvr"}, "CA"},
	{[]string{"澳大利亚", "澳洲", "悉尼", "墨尔本", "australia", "sydney", "melbourne", "au", "aus", "syd", "mel"}, "AU"},
	{[]string{"新西兰", "紐西蘭", "奥克兰", "new zealand", "auckland", "nz", "nzl", "akl"}, "NZ"},
	{[]string{"土耳其", "伊斯坦布尔", "turkey", "turkiye", "istanbul", "tr", "tur", "ist"}, "TR"},
	{[]string{"印度", "孟买", "新德里", "班加罗尔", "india", "mumbai", "delhi", "bangalore", "in", "ind", "bom", "del"}, "IN"},
	{[]string{"巴西", "圣保罗", "brazil", "sao paulo", "br", "bra", "gru"}, "BR"},
	{[]string{"俄罗斯", "莫斯科", "圣彼得堡", "伯力", "russia", "moscow", "petersburg", "ru", "rus", "mow", "svo"}, "RU"},
	{[]string{"马来西亚", "吉隆坡", "malaysia", "kuala lumpur", "my", "mys", "kul"}, "MY"},
	{[]string{"泰国", "曼谷", "thailand", "bangkok", "th", "tha", "bkk"}, "TH"},
	{[]string{"越南", "胡志明", "河内", "vietnam", "ho chi minh", "hanoi", "vn", "vnm", "sgn", "han"}, "VN"},
	{[]string{"菲律宾", "马尼拉", "philippines", "manila", "ph", "phl", "mnl"}, "PH"},
	{[]string{"印度尼西亚", "印尼", "雅加达", "indonesia", "jakarta", "id", "idn", "cgk"}, "ID"},
	{[]string{"阿联酋", "迪拜", "阿布扎比", "uae", "dubai", "abu dhabi", "ae", "are", "dxb"}, "AE"},
	{[]string{"瑞士", "苏黎世", "日内瓦", "switzerland", "zurich", "geneva", "ch", "che", "zrh"}, "CH"},
	{[]string{"瑞典", "斯德哥尔摩", "sweden", "stockholm", "se", "swe", "arn"}, "SE"},
	{[]string{"意大利", "米兰", "罗马", "italy", "milan", "rome", "it", "ita", "mil", "mxp", "rom"}, "IT"},
	{[]string{"西班牙", "马德里", "巴塞罗那", "spain", "madrid", "barcelona", "es", "esp", "mad", "bcn"}, "ES"},
	{[]string{"波兰", "华沙", "poland", "warsaw", "pl", "pol", "waw"}, "PL"},
	{[]string{"芬兰", "赫尔辛基", "finland", "helsinki", "fi", "fin", "hel"}, "FI"},
	{[]string{"挪威", "奥斯陆", "norway", "oslo", "no", "nor", "osl"}, "NO"},
	{[]string{"丹麦", "哥本哈根", "denmark", "copenhagen", "dk", "dnk", "cph"}, "DK"},
	{[]string{"爱尔兰", "都柏林", "ireland", "dublin", "ie", "irl", "dub"}, "IE"},
	{[]string{"奥地利", "维也纳", "austria", "vienna", "at", "aut", "vie"}, "AT"},
	{[]string{"葡萄牙", "里斯本", "portugal", "lisbon", "pt", "prt", "lis"}, "PT"},
	{[]string{"希腊", "雅典", "greece", "athens", "gr", "grc", "ath"}, "GR"},
	{[]string{"捷克", "布拉格", "czech", "prague", "cz", "cze", "prg"}, "CZ"},
	{[]string{"匈牙利", "布达佩斯", "hungary", "budapest", "hu", "hun", "bud"}, "HU"},
	{[]string{"罗马尼亚", "布加勒斯特", "romania", "bucharest", "ro", "rou", "otp"}, "RO"},
	{[]string{"保加利亚", "索非亚", "bulgaria", "sofia", "bg", "bgr", "sof"}, "BG"},
	{[]string{"乌克兰", "基辅", "ukraine", "kyiv", "kiev", "ua", "ukr", "iev"}, "UA"},
	{[]string{"以色列", "特拉维夫", "israel", "tel aviv", "il", "isr", "tlv"}, "IL"},
	{[]string{"卢森堡", "luxembourg", "lu", "lux"}, "LU"},
	{[]string{"南非", "约翰内斯堡", "south africa", "johannesburg", "za", "zaf", "jnb"}, "ZA"},
	{[]string{"阿根廷", "布宜诺斯艾利斯", "argentina", "buenos aires", "ar", "arg", "eze"}, "AR"},
	{[]string{"智利", "圣地亚哥", "chile", "santiago", "cl", "chl", "scl"}, "CL"},
	{[]string{"墨西哥", "mexico", "mx", "mex"}, "MX"},
}

func isASCIILetter(b byte) bool {
	return b >= 'a' && b <= 'z'
}

func isASCIIWord(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isASCIILetter(s[i]) {
			return false
		}
	}
	return len(s) > 0
}

// matchWord checks whether word appears in lower; short ASCII codes (<=3 chars)
// require non-letter boundaries so "node" does not match "de" or "free" match "fr".
func matchWord(lower, word string) bool {
	if len(word) > 3 || !isASCIIWord(word) {
		return strings.Contains(lower, word)
	}
	start := 0
	for {
		idx := strings.Index(lower[start:], word)
		if idx < 0 {
			return false
		}
		abs := start + idx
		leftOK := abs == 0 || !isASCIILetter(lower[abs-1])
		rightPos := abs + len(word)
		rightOK := rightPos == len(lower) || !isASCIILetter(lower[rightPos])
		if leftOK && rightOK {
			return true
		}
		start = abs + 1
	}
}

func extractFlagCode(s string) string {
	var prev rune
	for _, r := range s {
		if r >= 0x1F1E6 && r <= 0x1F1FF {
			if prev != 0 {
				return string([]byte{'A' + byte(prev-0x1F1E6), 'A' + byte(r-0x1F1E6)})
			}
			prev = r
		} else {
			prev = 0
		}
	}
	return ""
}

// Region maps a raw node name to a region code, longest match first,
// falling back to any flag emoji present in the name.
func Region(rawName string) string {
	lower := strings.ToLower(rawName)
	best, bestLen := "", 0
	for _, r := range regions {
		for _, w := range r.words {
			if matchWord(lower, w) && len(w) > bestLen {
				best, bestLen = r.code, len(w)
			}
		}
	}
	if best != "" {
		return best
	}
	return extractFlagCode(rawName)
}

// RegionFlag maps a raw node name to its Unicode regional indicator flag emoji.
func RegionFlag(rawName string) string {
	code := Region(rawName)
	if len(code) != 2 || code[0] < 'A' || code[0] > 'Z' || code[1] < 'A' || code[1] > 'Z' {
		return ""
	}
	return string([]rune{0x1F1E6 + rune(code[0]-'A'), 0x1F1E6 + rune(code[1]-'A')})
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
