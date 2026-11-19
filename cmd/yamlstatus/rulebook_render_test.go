package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRulebookMarkdownRendering(t *testing.T) {
	t.Parallel()
	source := []byte("# Räkor 日本語\n\n" +
		"| Type | Value |\n| --- | --- |\n| rune | λ |\n\n" +
		"> A **rule** with `Result[T, E]`.\n\n" +
		"- outer\n  - nested\n\n" +
		"```sec\nfn A() bool { return 1 < 2 && 3 > 2 }\n```\n\n" +
		"[book](../types/contracts.md#12) ![diagram](images/example.png)\n\n" +
		"~~retired~~\n\n<details><summary>Details</summary>Text<br>more</details>\n\n" +
		"<a id=\"explicit-anchor\"></a>\n")
	original := bytes.Clone(source)
	got, err := renderRulebookBody("book.md", source)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<h1>Räkor 日本語</h1>", "<table>", "<td>λ</td>",
		"<blockquote>", "<strong>rule</strong>", "<code>Result[T, E]</code>",
		"<li>nested</li>", `class="language-sec"`, "1 &lt; 2 &amp;&amp; 3 &gt; 2",
		`href="../types/contracts.md#12"`, `src="images/example.png"`,
		"<del>retired</del>", "<details>", "<summary>Details</summary>", "<br>",
		`id="explicit-anchor"`,
	} {
		if !bytes.Contains(got, []byte(want)) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
	if !bytes.Equal(source, original) {
		t.Fatal("renderer modified its source")
	}
	again, err := renderRulebookBody("book.md", source)
	if err != nil || !bytes.Equal(got, again) {
		t.Fatalf("rendering is not deterministic: %v", err)
	}
}

func TestRulebookRenderingSanitizesFinalHTML(t *testing.T) {
	t.Parallel()
	cases := []string{
		`<script>alert(1)</script>`,
		`<img src="ok.png" onerror="alert(1)">`,
		`<a href="javascript:alert(1)">bad</a>`,
		`<a href="jav&#x61;script:alert(1)">bad</a>`,
		`[bad](javascript:alert%281%29)`,
		`![bad](data:text/html,evil)`,
		`<iframe src="https://example.com"></iframe>`,
		`<svg onload="alert(1)"><a href="javascript:alert(1)">bad</a></svg>`,
		`<p style="background:url(javascript:alert(1))">text</p>`,
		"```sec\n<script>alert(1)</script>\n```",
	}
	for _, source := range cases {
		t.Run(source, func(t *testing.T) {
			got, err := renderRulebookBody("unsafe.md", []byte(source))
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"<script", "<iframe", "<svg", "onerror=", "onload=", "javascript:", "data:text/html", "style="} {
				if strings.Contains(strings.ToLower(string(got)), forbidden) {
					t.Errorf("unsafe %q survived: %s", forbidden, got)
				}
			}
			if strings.HasPrefix(source, "```") && !bytes.Contains(got, []byte("&lt;script&gt;alert(1)&lt;/script&gt;")) {
				t.Fatalf("code example was lost: %s", got)
			}
		})
	}
}

func TestRulebookPlainTextAndInvalidFormats(t *testing.T) {
	t.Parallel()
	source := []byte("MLIR\n====\n\n<script>& λ\n  %x = literal\n")
	got, err := renderRulebookBody("mlir.TXT", source)
	if err != nil {
		t.Fatal(err)
	}
	want := "<pre>MLIR\n====\n\n&lt;script&gt;&amp; λ\n  %x = literal\n</pre>\n"
	if string(got) != want {
		t.Fatalf("plain text changed: %s", got)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"invalid.md", []byte{0xff}},
		{"invalid.txt", []byte{0xff}},
		{"book.yaml", []byte("title: X")},
	} {
		if got, err := renderRulebookBody(tc.name, tc.data); err == nil || got != nil || !strings.Contains(err.Error(), tc.name) {
			t.Errorf("expected contextual error without partial HTML for %s: %s, %v", tc.name, got, err)
		}
	}
	if got, err := renderRulebookBody("empty.md", nil); err != nil || len(got) != 0 {
		t.Fatalf("empty Markdown: %s, %v", got, err)
	}
}

// Exercise real rulebooks without declaring every file a publication candidate;
// canonical inventory selection remains a separate governance requirement.
func TestRulebookRendererRepositoryBooks(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"README.md", "types/contracts.md", "tooling/diagnostics.md",
		"mlir/sec_mlir_dialect.md", "mlir/mlir.txt", "mlir/mlir-optimize.txt",
	} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("..", "..", "rules", filepath.FromSlash(name)))
			if err != nil {
				t.Fatal(err)
			}
			got, err := renderRulebookBody(name, source)
			if err != nil || len(got) == 0 {
				t.Fatalf("real rulebook rendering failed: %v", err)
			}
		})
	}
}
