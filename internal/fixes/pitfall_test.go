package fixes

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestPitfallEditRequiresCompleteCertificate checks revocation at the shared
// edit boundary, exact source identity and refusal of heuristic suggestions.
// Rules: rules/analysis/pitfall_analysis.md — "Fix safety", "Corrective actions";
// rules/tooling/lsp.md — "Safe fixes".
func TestPitfallEditRequiresCompleteCertificate(t *testing.T) {
	file := "../../testdata/sema/pitfall_lsp_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	p := parser.New(lexer.NewWithFile(source, file))
	a := sema.NewAnalyzer()
	if errors := a.Analyze(p.ParseProgram()); len(errors) != 0 {
		t.Fatal(errors)
	}
	finding := a.PitfallAnalysis().Findings()[0]
	action := finding.Actions[0]
	if _, ok := Pitfall(source, file, finding, action); !ok {
		t.Fatal("valid certified edit rejected", finding)
	}
	for _, revoke := range []func(*sema.PitfallSuggestedAction){
		func(v *sema.PitfallSuggestedAction) { v.Kind = sema.PitfallSuggestedEdit },
		func(v *sema.PitfallSuggestedAction) { v.Safety.EvaluationOrder = "" },
		func(v *sema.PitfallSuggestedAction) { v.Safety.EvaluationCount = "" },
		func(v *sema.PitfallSuggestedAction) { v.Safety.Effects = "" },
		func(v *sema.PitfallSuggestedAction) { v.Safety.Failure = "" },
		func(v *sema.PitfallSuggestedAction) { v.Safety.Ownership = "" },
		func(v *sema.PitfallSuggestedAction) { v.Safety.Borrow = "" },
		func(v *sema.PitfallSuggestedAction) { v.Safety.ControlFlow = "" },
		func(v *sema.PitfallSuggestedAction) { v.Safety.Rule = sema.PitfallWrongBoundSource },
		func(v *sema.PitfallSuggestedAction) { v.Replacement = "false" },
		func(v *sema.PitfallSuggestedAction) { v.Source.Column++ },
	} {
		broken := action
		revoke(&broken)
		if edit, ok := Pitfall(source, file, finding, broken); ok {
			t.Fatal("incomplete certificate accepted", edit, broken)
		}
	}
	for _, changed := range []string{strings.ReplaceAll(source, "flag == true", "flag == false"), strings.ReplaceAll(source, "flag == true", "flag ==")} {
		if edit, ok := Pitfall(changed, file, finding, action); ok {
			t.Fatal("obsolete/recovered source accepted", edit)
		}
	}
	if _, ok := Pitfall(source, "other.sec", finding, action); ok {
		t.Fatal("wrong source accepted")
	}
}
