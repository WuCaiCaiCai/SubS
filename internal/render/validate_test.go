package render

import (
	"os"
	"testing"
)

func TestValidateRealTemplates(t *testing.T) {
	for _, f := range []string{"../../模板示例/Client.yaml", "../../模板示例/Router.yaml"} {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		issues, err := ValidateTemplate(string(raw))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if len(issues) != 0 {
			t.Errorf("%s: unexpected issues: %+v", f, issues)
		}
	}
}

func TestValidateBrokenTemplate(t *testing.T) {
	issues, _ := ValidateTemplate("port: 7890\nmode: rule\n")
	if len(issues) == 0 {
		t.Fatal("expected missing proxies issue")
	}
	issues, _ = ValidateTemplate("proxies: []\nproxy-groups:\n  - name: A\n    type: select\n    proxies: [Missing]\n")
	found := false
	for _, i := range issues {
		if i.Message == "策略组引用了未定义的目标: Missing" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected undefined target issue, got %+v", issues)
	}
}
