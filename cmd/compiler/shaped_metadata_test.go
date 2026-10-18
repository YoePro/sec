package main

import (
	"strings"
	"testing"
)

// MD-016 part 1: the shaped metadata types have their canonical source
// surface — positional literals with exact arity and component types,
// get-only Rank/Count, read-only uint-indexed reads (int for Strides),
// infallible Shape/Strides initializers, ShapeError-fallible Axes/AxisList
// initializers, the rank-specific TensorLayout enum with exactly three
// variants and exhaustive switch coverage, and the core ShapeError enum
// (correction §§ 11.1–11.6).
//
// Rules:
//   - rules/collections/shaped-types.md — §§ 4.1, 6.0, 7.1, 15.3, 29.3
//   - rules/corrections/applied/md016-shaped-metadata-declarations-correction-20261018.md — § 11
func TestShapedMetadataSurface(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/shaped_metadata_valid.sec")
	if code != 0 {
		t.Fatalf("valid shaped metadata rejected: %d %s %s", code, stdout, stderr)
	}
	_, stderr, code = runCLIForDiagnostics(t, "check", "../../testdata/sema/shaped_metadata_invalid.sec")
	if code != 3 {
		t.Fatalf("invalid shaped metadata accepted: %d %s", code, stderr)
	}
	for _, want := range []string{
		"[S1173]: Shape[3] literal requires exactly 3 components, got 2",
		"[S1173]: component 0 of Shape[2] must be uint, got int",
		"[S1173]: Shape[1] literal lists its components in order",
		"[S1173]: Axes[3] literal: axis 3 is out of range 0..<3",
		"[S1173]: Axes[3] literal: axis 1 is duplicated (components 1 and 2)",
		"[S1173]: AxisList[2] literal: axis 4 is duplicated (components 0 and 1)",
		"[S1173]: Axes[2] literal needs compile-time constant axes; use try new Axes[2](axes)",
		"[S1173]: components of Shape[2] are read-only",
		"index 2 is out of bounds for Shape[2] with 2 components",
		"new Axes[3] validates its axes and may fail with ShapeError; use try",
		"function RankMismatch must return Shape[3], got Shape[2]",
		"unknown member Len on Shape[2]",
		"TensorLayout has no variant Tiled",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("missing %q in:\n%s", want, stderr)
		}
	}
	if count := strings.Count(stderr, "sema error"); count != 13 {
		t.Errorf("want exactly 13 diagnostics, got %d:\n%s", count, stderr)
	}
}
