package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

type initScaffold struct {
	MainPath        string
	SourceDirectory string
	Directories     []string
}

// projectInitLayout selects creation paths; a minimal root command uses source
// "." and does not create application-only cmd/bin/internal directories.
// Rules: rules/projects/projects.md — §14(8),§37(6–8).
func projectInitLayout(projectDir, targetName string, minimal bool) initScaffold {
	if minimal {
		return initScaffold{MainPath: filepath.Join(projectDir, "main.sec"), SourceDirectory: ".", Directories: []string{filepath.Join(projectDir, ".sec")}}
	}
	return initScaffold{
		MainPath:        filepath.Join(projectDir, "cmd", targetName, "main.sec"),
		SourceDirectory: filepath.ToSlash(filepath.Join("cmd", targetName)),
		Directories:     []string{filepath.Join(projectDir, "cmd", targetName), filepath.Join(projectDir, "bin"), filepath.Join(projectDir, ".sec"), filepath.Join(projectDir, "internal")},
	}
}

// defaultMainSource produces the ordinary command entry module.
// Rules: rules/projects/projects.md — §39(3),§14(8).
func defaultMainSource() string {
	return `module main

fn main() int {
	return 0
}
`
}

// defaultSecConfig writes the common identity/version/build model and the
// command Target appropriate to the selected scaffold.
// Rules: rules/projects/projects.md — §§14(8),17,22,26,37(1–8).
func defaultSecConfig(options initCommandOptions, uuid string, targetName string) (string, error) {
	definition, ok := findTargetDefinition(options.Target)
	if !ok || definition.LLVMTriple == "" {
		return "", fmt.Errorf("target %s has no LLVM triple", options.Target.String())
	}

	outputName := sanitizeOutputName(options.ProjectName)
	if outputName == "" {
		return "", fmt.Errorf("project name %q cannot be used as output name", options.ProjectName)
	}

	var b strings.Builder
	fmt.Fprintln(&b, "[project]")
	fmt.Fprintf(&b, "name = %q\n", options.ProjectName)
	fmt.Fprintf(&b, "uuid = %q\n", uuid)
	fmt.Fprintln(&b, "imports = []")
	fmt.Fprintln(&b)
	// A new Project starts at version 0.1.0.0 with the canonical format and
	// no userdefined component (rules/projects/projects.md § 37(1)–(3), § 8, § 10).
	fmt.Fprintln(&b, "[version]")
	fmt.Fprintln(&b, "major = 0")
	fmt.Fprintln(&b, "minor = 1")
	fmt.Fprintln(&b, "revision = 0")
	fmt.Fprintln(&b, "build = 0")
	fmt.Fprintln(&b, `format = "[major].[minor].[revision].[build]"`)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "[build]")
	fmt.Fprintln(&b, `backend = "mlir"`)
	fmt.Fprintf(&b, "profile = %q\n", options.Profile)
	fmt.Fprintln(&b, `diagnostics = "default"`)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "[analysis]")
	fmt.Fprintln(&b, `lsp_depth = "interactive"`)
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "[profile.%s]\n", tomlBareKey(options.Profile))
	fmt.Fprintln(&b, `optimization = "none"`)
	fmt.Fprintln(&b, `debug = "line"`)
	fmt.Fprintln(&b, "strip = false")
	fmt.Fprintln(&b)
	variantName := options.Target.String()
	fmt.Fprintf(&b, "[variant.%s]\n", tomlBareKey(variantName))
	fmt.Fprintf(&b, "os = %q\n", options.Target.OS)
	fmt.Fprintf(&b, "arch = %q\n", options.Target.Arch)
	if definition.LLVMTriple != "" {
		fmt.Fprintf(&b, "llvm_triple = %q\n", definition.LLVMTriple)
	}
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "[target.%s]\n", tomlBareKey(targetName))
	fmt.Fprintln(&b, `kind = "command"`)
	fmt.Fprintf(&b, "source = %q\n", projectInitLayout(options.ProjectDir, targetName, options.Minimal).SourceDirectory)
	fmt.Fprintf(&b, "artifact = %q\n", outputName)
	fmt.Fprintf(&b, "variants = [%q]\n", variantName)
	fmt.Fprintf(&b, "output = %q\n", filepath.ToSlash(filepath.Join("bin", targetName, variantName, options.Profile, outputName)))
	fmt.Fprintln(&b)
	b.WriteString(defaultFormatConfig())
	return b.String(), nil
}

// defaultFormatConfig documents every project formatter key and its canonical
// defaults, with editable alternatives in TOML comments.
// Rules: rules/tooling/formatter.md — §4(5–17),§4(20–21);
// rules/projects/projects.md — §6 "Manifest tables", "[format]".
func defaultFormatConfig() string {
	return `[format]
# Formatting contract version. Supported version: 1 (not the compiler version).
format_version = 1

# Vertical spacing: "structured" separates logical groups with blank lines;
# "compact" reduces optional blank lines. Default: "structured".
vertical_style = "structured"

# Indentation in spaces. Allowed values: 2, 4, 6. Tabs are not used.
indentation_width = 4

# Width policy: "fixed" uses line_width; "adaptive" uses the editor viewport
# within the adaptive limits, or adaptive_fallback_width without a viewport.
width_mode = "fixed"

# Preferred line width in columns (positive integer), not a hard limit.
line_width = 120

# Adaptive limits in columns (positive integers): minimum <= maximum,
# and minimum <= fallback <= maximum. Used when width_mode = "adaptive".
adaptive_min_width = 80
adaptive_max_width = 200
adaptive_fallback_width = 120

# Largest padding in spaces that column alignment may insert; a group needing
# more is not aligned. Default: four indentation widths (16 at width 4).
# max_alignment_padding = 16

# Optional supported syntax corrections: true enables them; false keeps
# formatting limited to layout. Default: false. CLI --fix can opt in.
language_corrections = false
`
}
