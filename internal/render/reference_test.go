package render

import (
	"os"
	"testing"

	"github.com/WuCaiCaiCai/SubS/internal/protocol"
	"gopkg.in/yaml.v3"
)

// TestCompareWithReferenceFile: 用户提供的真实订阅文件（脱敏副本
// testdata/reference.yaml）做基准——解析其中的节点，再用整份文件当模板
// 渲染，输出应与基准保持一致。凭据/域名已替换，字段结构与协议特征保留。
func TestCompareWithReferenceFile(t *testing.T) {
	ref, err := os.ReadFile("testdata/reference.yaml")
	if err != nil {
		t.Skip("参考文件不存在，跳过")
	}

	parsed, perrs, _ := protocol.ParseSubscription(ref)
	if len(perrs) > 0 {
		t.Errorf("解析错误 %d 个:", len(perrs))
		for _, e := range perrs {
			t.Errorf("  line %d: %s", e.Line, e.Reason)
		}
	}

	var refDoc map[string]any
	if err := yaml.Unmarshal(ref, &refDoc); err != nil {
		t.Fatal(err)
	}
	refProxies := refDoc["proxies"].([]any)
	if len(parsed) != len(refProxies) {
		t.Fatalf("节点数不一致: 解析=%d 参考=%d", len(parsed), len(refProxies))
	}

	var nodes []Node
	for _, p := range parsed {
		nodes = append(nodes, Node{Name: p.Name, Config: p.Config})
	}

	res, err := Render(TargetMihomo, string(ref), nodes)
	if err != nil {
		t.Fatal(err)
	}

	var outDoc map[string]any
	if err := yaml.Unmarshal([]byte(res.Body), &outDoc); err != nil {
		t.Fatalf("渲染输出无法解析: %v", err)
	}
	outProxies := outDoc["proxies"].([]any)

	// 逐节点逐字段对比
	for i, rp := range refProxies {
		rm := rp.(map[string]any)
		om := outProxies[i].(map[string]any)
		if rm["name"] != om["name"] {
			t.Errorf("节点%d name: 参考=%v 输出=%v", i, rm["name"], om["name"])
			continue
		}
		name := rm["name"]
		for k, rv := range rm {
			// 已知差异豁免：mihomo 的 hysteria2 凭据字段就是 password，
			// 参考文件同写的 auth 是另一内核的遗留别名，mihomo 忽略之，不补。
			if k == "auth" && om["type"] == "hysteria2" {
				continue
			}
			ov, ok := om[k]
			if !ok {
				t.Errorf("节点[%v] 缺字段 %q（参考值=%v）", name, k, rv)
				continue
			}
			if !deepEqual(rv, ov) {
				t.Errorf("节点[%v] 字段 %q: 参考=%v 输出=%v", name, k, rv, ov)
			}
		}
		for k := range om {
			if _, ok := rm[k]; !ok {
				t.Errorf("节点[%v] 多出字段 %q=%v", name, k, om[k])
			}
		}
	}

	// proxy-groups / rules 应原样保留
	if !deepEqual(refDoc["proxy-groups"], outDoc["proxy-groups"]) {
		t.Error("proxy-groups 与参考不一致")
	}
	if !deepEqual(refDoc["rules"], outDoc["rules"]) {
		t.Error("rules 与参考不一致")
	}

	// 顶层键完整性
	for k := range refDoc {
		if k == "proxies" || k == "proxy-groups" || k == "rules" {
			continue
		}
		if _, ok := outDoc[k]; !ok {
			t.Errorf("丢失顶层键 %q", k)
		}
	}
}

func deepEqual(a, b any) bool {
	ab, _ := yaml.Marshal(a)
	bb, _ := yaml.Marshal(b)
	return string(ab) == string(bb)
}
