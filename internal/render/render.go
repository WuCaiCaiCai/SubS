// Package render turns selected nodes into client configuration payloads.
package render

import (
	"encoding/json"
	"fmt"

	"github.com/wucai/subs/internal/protocol"
)

// Node is a resolved, render-ready node.
type Node struct {
	Name   string
	Config protocol.Config
}

// Result is the rendered payload plus nodes skipped for this target.
type Result struct {
	Body    string
	Skipped []string
	Extra   []string
}

// Target constants mirror the UA negotiation table.
const (
	TargetMihomo  = "mihomo"
	TargetSingBox = "sing-box"
	TargetLoon    = "loon"
)

// Supports reports whether a protocol can be rendered for a target.
func Supports(target, proto string) bool {
	switch target {
	case TargetSingBox, TargetLoon:
		return proto != "mieru"
	}
	return true
}

// Render dispatches to the target renderer.
func Render(target string, template string, nodes []Node) (Result, error) {
	switch target {
	case TargetMihomo:
		return renderMihomo(template, nodes)
	case TargetSingBox:
		return renderSingBox(nodes)
	case TargetLoon:
		return renderLoon(nodes)
	}
	return Result{}, fmt.Errorf("未知目标: %s", target)
}

func authString(c protocol.Config, key string) string {
	if v, ok := c.Auth[key].(string); ok {
		return v
	}
	return ""
}

func transportString(c protocol.Config, key string) string {
	if v, ok := c.Transport[key].(string); ok {
		return v
	}
	return ""
}

func transportBool(c protocol.Config, key string) bool {
	v, _ := c.Transport[key].(bool)
	return v
}

func extraString(c protocol.Config, key string) string {
	if v, ok := c.Extra[key].(string); ok {
		return v
	}
	return ""
}

// JSON marshalling helper that keeps insertion order irrelevant but output stable.
func marshal(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
