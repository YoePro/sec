package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Rules: rules/projects/projects.md — §23; rules/tooling/lsp.md — Configuration.
func TestAnalysisDepthTOMLSpellings(t *testing.T) {
	for _, text := range []string{
		"[analysis] # project budget\nlsp_depth = 'deep'\n",
		"analysis.lsp_depth = 'deep'\n",
		"analysis = { lsp_depth = 'deep' }\n",
		"[\"analysis\"]\n\"lsp_depth\" = \"deep\"\n",
	} {
		depth, err := parseLSPAnalysisDepth(text)
		if err != nil || depth != sema.AnalysisDeep {
			t.Fatal(text, depth, err)
		}
	}
	for _, text := range []string{"", "[analysis]\n", "[project]\nnotes = '''\n[analysis]\nlsp_depth = \"deep\"\n'''\n"} {
		depth, err := parseLSPAnalysisDepth(text)
		if err != nil || depth != sema.AnalysisInteractive {
			t.Fatal(text, depth, err)
		}
	}
	for _, text := range []string{"[analysis]\nlsp_depth = 3\n", "analysis.lsp_depth = 'unsupported'\n", "[analysis]\nlsp_depth = 'deep'\nlsp_depth = 'interactive'\n"} {
		if _, err := parseLSPAnalysisDepth(text); err == nil {
			t.Fatal("invalid configuration accepted", text)
		}
	}
}

// Rules: rules/tooling/lsp.md — Configuration, Snapshots;
// rules/compiler/compiler_analysis.md — §§55(2),60(5),62(1–3).
func TestAnalysisDepthSnapshotCacheReplacement(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, ".sec", "sec.toml")
	settings, err := os.ReadFile("../../testdata/projects/model/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	base := string(settings)
	if err := os.WriteFile(manifest, []byte(base+"\n[analysis]\nlsp_depth = 'interactive'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../testdata/projects/init/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "cmd", "sec", "main.sec")
	uri, text := uriFromPath(path), string(data)
	enableAnalysisCache()
	defer disableAnalysisCache()
	var previous *sema.Analyzer
	for _, depth := range []sema.AnalysisDepth{sema.AnalysisInteractive, sema.AnalysisStandard, sema.AnalysisDeep, sema.AnalysisInteractive} {
		overlay := sourceOverlay{Sources: map[string]string{manifest: "analysis.lsp_depth = '" + string(depth) + "'\n" + base}}
		parsed := parser.New(lexer.NewWithFile(text, path)).Parse()
		_, analyzer, release := cachedLSPAnalysis(uri, text, parsed.Program, overlay)
		actualDepth, budget := analyzer.AnalysisDepth(), analyzer.AnalysisBudget()
		release()
		if actualDepth != depth || budget != sema.NewAnalyzerWithDepth(depth).AnalysisBudget() {
			t.Fatal(depth, actualDepth, budget)
		}
		if analyzer == previous {
			t.Fatal("incompatible budget reused cached facts", depth)
		}
		previous = analyzer
		parsed = parser.New(lexer.NewWithFile(text, path)).Parse()
		_, again, release := cachedLSPAnalysis(uri, text, parsed.Program, overlay)
		release()
		if again != analyzer {
			t.Fatal("unchanged budget did not reuse snapshot")
		}
	}
	if disk := lspAnalysisDepth(path); disk != sema.AnalysisInteractive {
		t.Fatal("unsaved budget changed disk", disk)
	}
	advanceAnalysisEpoch()
	if err := os.WriteFile(manifest, []byte("analysis.lsp_depth = 'deep'\n"+base), 0644); err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(text, path)).Parse()
	_, analyzer, release := cachedLSPAnalysis(uri, text, parsed.Program, sourceOverlay{})
	release()
	if analyzer.AnalysisDepth() != sema.AnalysisDeep || analyzer == previous {
		t.Fatal("configuration reload retained old budget")
	}
}

// Rules: rules/compiler/compiler_analysis.md — §62(2–3); borrowing.md — safety.
func TestAnalysisDepthKeepsMandatorySafety(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/borrow_authority_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	path, manifest := filepath.Join(root, "main.sec"), filepath.Join(root, ".sec", "sec.toml")
	var baseline []sema.Error
	for _, depth := range []sema.AnalysisDepth{sema.AnalysisInteractive, sema.AnalysisStandard, sema.AnalysisDeep} {
		overlay := sourceOverlay{Sources: map[string]string{manifest: "analysis.lsp_depth = '" + string(depth) + "'\n"}}
		p := parser.New(lexer.NewWithFile(string(data), path))
		parsed := p.Parse()
		if parsed.HasErrors {
			t.Fatal(p.Errors())
		}
		a := newLSPAnalyzerForInputs(uriFromPath(path), parsed.Program, overlay)
		errors := a.Analyze(parsed.Program)
		if len(errors) == 0 || a.AnalysisDepth() != depth {
			t.Fatal(depth, errors)
		}
		if baseline == nil {
			baseline = errors
		} else if !reflect.DeepEqual(errors, baseline) {
			t.Fatal("budget changed mandatory safety", depth, errors, baseline)
		}
	}
}
