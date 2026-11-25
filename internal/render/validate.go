package render

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Issue is one template validation finding.
type Issue struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// ValidateTemplate runs the checks from 模板占位符与命名规则规范.md §3.
func ValidateTemplate(content string) ([]Issue, error) {
	var issues []Issue
	if strings.TrimSpace(content) == "" {
		return []Issue{{Line: 0, Message: "模板内容为空"}}, nil
	}
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		line := 0
		if ye, ok := err.(*yaml.TypeError); ok {
			line = parseYAMLLine(ye.Errors[0])
		}
		return []Issue{{Line: line, Message: "YAML 语法错误: " + firstLine(err.Error())}}, nil
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return []Issue{{Line: 1, Message: "模板顶层必须是映射"}}, nil
	}
	doc := root.Content[0]

	hasProxiesKey := false
	markerCount := 0
	groupNames := map[string]bool{}
	var unknownMarkers []Issue
	var groupProxiesRefs []Issue

	var walk func(n *yaml.Node, inGroup bool)
	walk = func(n *yaml.Node, inGroup bool) {
		if n.Kind != yaml.MappingNode {
			return
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, val := n.Content[i], n.Content[i+1]
			markers := markerSet(key)
			for name := range markers {
				switch name {
				case "proxies":
					markerCount++
				case "nodes", "names":
				default:
					unknownMarkers = append(unknownMarkers, Issue{Line: key.Line, Message: "未知 subs 标记: subs:" + name})
				}
			}
			if n == doc && key.Value == "proxies" {
				hasProxiesKey = true
			}
			if n == doc && key.Value == "proxy-groups" && val.Kind == yaml.SequenceNode {
				for _, g := range val.Content {
					collectGroup(g, groupNames, &groupProxiesRefs)
				}
			}
			walk(val, inGroup || key.Value == "proxy-groups")
		}
	}
	walk(doc, false)

	if markerCount > 1 {
		issues = append(issues, Issue{Line: 0, Message: fmt.Sprintf("发现 %d 个 # subs:proxies 标记，必须恰好一个", markerCount)})
	}
	if markerCount == 0 && !hasProxiesKey {
		issues = append(issues, Issue{Line: 0, Message: "模板缺少 proxies 键或 # subs:proxies 注入点"})
	}
	issues = append(issues, unknownMarkers...)

	// Group references must resolve to a known group or to injected names.
	for _, ref := range groupProxiesRefs {
		if ref.Message == "" {
			continue
		}
		if !groupNames[ref.Message] && !isBuiltinTarget(ref.Message) {
			issues = append(issues, Issue{Line: ref.Line, Message: "策略组引用了未定义的目标: " + ref.Message})
		}
	}

	// Dry-run: injection must succeed against a representative node set.
	if len(issues) == 0 {
		if _, err := Render(TargetMihomo, content, sampleNodes()); err != nil {
			issues = append(issues, Issue{Line: 0, Message: "试注入失败: " + err.Error()})
		}
	}
	return issues, nil
}

// collectGroup records group names and validates proxies references.
func collectGroup(g *yaml.Node, names map[string]bool, refs *[]Issue) {
	if g.Kind != yaml.MappingNode {
		return
	}
	name := ""
	for i := 0; i+1 < len(g.Content); i += 2 {
		if g.Content[i].Value == "name" {
			name = g.Content[i+1].Value
		}
	}
	if name != "" {
		names[name] = true
	}
	for i := 0; i+1 < len(g.Content); i += 2 {
		if g.Content[i].Value != "proxies" {
			continue
		}
		val := g.Content[i+1]
		if val.Kind != yaml.SequenceNode {
			continue
		}
		for _, item := range val.Content {
			// Injected name lists are rewritten at render time, so a marker on
			// this key means the entries are placeholders.
			if markerSet(g.Content[i])["nodes"] || markerSet(g.Content[i])["names"] {
				continue
			}
			*refs = append(*refs, Issue{Line: item.Line, Message: item.Value})
		}
	}
}

func isBuiltinTarget(name string) bool {
	switch strings.ToUpper(name) {
	case "DIRECT", "REJECT", "PASS", "REJECT-DROP", "COMPATIBLE":
		return true
	}
	return false
}

func sampleNodes() []Node {
	return []Node{
		{Name: "sample-hy2", Config: sampleConfig("hy2")},
		{Name: "sample-vless", Config: sampleConfig("vless")},
	}
}

func parseYAMLLine(msg string) int {
	var line int
	if _, err := fmt.Sscanf(msg, "line %d:", &line); err == nil {
		return line
	}
	return 0
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
