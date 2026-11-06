package main

import (
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkAnalyzeStdlibDocument measures one complete LSP analysis of a
// stdlib document, including core loading, so responsiveness regressions are
// reproducible: go test ./cmd/lsp -run '^$' -bench AnalyzeStdlibDocument
// -benchmem (add -cpuprofile/-memprofile to profile).
// Rules: rules/tooling/lsp.md — "Responsiveness model".
func BenchmarkAnalyzeStdlibDocument(b *testing.B) {
	root, err := filepath.Abs("../..")
	if err != nil {
		b.Fatal(err)
	}
	path := filepath.Join(root, "sec", "stdlib", "net", "http", "methods.sec")
	data, err := os.ReadFile(path)
	if err != nil {
		b.Skip(err)
	}
	uri := "file://" + path
	b.ReportAllocs()
	for b.Loop() {
		analyze(uri, string(data))
	}
}
