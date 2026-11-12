package project

import (
	"strings"
	"testing"
)

// The project declares typed parameters; a Target override keeps the type
// and cannot introduce a parameter.
// Rules: rules/projects/projects.md — § 24.
func TestParseManifestParameters(t *testing.T) {
	manifest := `[project]
name = "shop"

[parameters]
audit = true
level = 2
name = "Shop"
ratio = 0.5

[target.shop]
kind = "command"
source = "src"

[target.shop.parameters]
audit = false
level = "high"
extra = true
`
	parameters, problems := ParseManifestParameters(manifest, "shop")
	if parameters["audit"].Kind != "bool" || parameters["audit"].Bool || parameters["level"].Kind != "integer" || parameters["level"].Text != "2" ||
		parameters["name"].Text != "Shop" || parameters["ratio"].Kind != "float" {
		t.Errorf("parameters = %+v", parameters)
	}
	text := []string{}
	for _, problem := range problems {
		text = append(text, problem.Error())
	}
	joined := strings.Join(text, "\n")
	if len(problems) != 2 || !strings.Contains(joined, "overrides parameter extra, which [parameters] does not declare") ||
		!strings.Contains(joined, "overrides integer parameter level with a different type") {
		t.Errorf("problems = %v", problems)
	}
	if base, _ := ParseManifestParameters(manifest, ""); !base["audit"].Bool {
		t.Errorf("project-level value = %+v", base)
	}
}
