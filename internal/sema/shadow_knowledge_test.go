package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// MD-113: register impl members publish shadow-knowledge summaries, and uses
// on a hardware binding need their requirements proven by an observation, an
// ordinary write or an IsShadowKnown guard, along every path; a specialized
// action never establishes knowledge.
// Rules: rules/platform/hardware-register-access.md — § 9.8 (Amendment MD-113);
// rules/corrections/applied/MD-113-shadow-knowledge-across-calls-decision-20261030.md — §§ 5, 7, 8, 11, 12(2)-12(3).
func TestShadowKnowledgeAnalysis(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/shadow_knowledge.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errs := analyzeSourceWithAnalyzerRaw(t, string(data))
	for member, want := range map[string]ShadowKnowledgeSummary{
		"Control.GetMode":      {Requires: []string{"Mode"}, Ensures: []string{}, EnsuresWhenTrue: []string{}, Invalidates: []string{}},
		"Control.HasKnownMode": {Requires: []string{}, Ensures: []string{}, EnsuresWhenTrue: []string{"Mode"}, Invalidates: []string{}},
		"Control.TryGetMode":   {Requires: []string{}, Ensures: []string{}, EnsuresWhenTrue: []string{}, Invalidates: []string{}},
		"Control.Refresh":      {Requires: []string{}, Ensures: []string{"Clear", "Mode"}, EnsuresWhenTrue: []string{}, Invalidates: []string{}},
		"Control.Reset":        {Requires: []string{}, Ensures: []string{"Mode"}, EnsuresWhenTrue: []string{}, Invalidates: []string{}},
		"Control.Ack":          {Requires: []string{}, Ensures: []string{}, EnsuresWhenTrue: []string{}, Invalidates: []string{}},
		"Control.ResetThenGet": {Requires: []string{}, Ensures: []string{"Mode"}, EnsuresWhenTrue: []string{}, Invalidates: []string{}},
		"Control.Overwrite":    {Requires: []string{}, Ensures: []string{}, EnsuresWhenTrue: []string{}, Invalidates: []string{"Clear", "Mode"}},
		"Control.Commit":       {Requires: []string{}, Ensures: []string{}, EnsuresWhenTrue: []string{}, Invalidates: []string{"Clear"}},
	} {
		got, ok := analyzer.ShadowKnowledgeSummaryOf(member)
		if !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %+v (%v), want %+v", member, got, ok, want)
		}
	}
	wants := []struct {
		line int
		text string
	}{
		{65, "shadow field Mode of control may be unknown here: Control.GetMode requires it"},
		{70, "shadow field Mode of control may be unknown here: Control.GetMode requires it"},
		{82, "shadow field Mode of control may be unknown here: Control.GetMode requires it"},
		{88, "shadow field Mode of control may be unknown here: Control.GetMode requires it"},
	}
	if len(errs) != len(wants) {
		t.Fatalf("errors = %v", errs)
	}
	for i, want := range wants {
		if errs[i].Line != want.line || !strings.Contains(errs[i].Message, want.text) {
			t.Errorf("error %d = %v, want %q at %d", i, errs[i], want.text, want.line)
		}
	}
}
