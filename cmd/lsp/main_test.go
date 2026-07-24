package main

import (
	"strings"
	"testing"
)

func TestAnalyzeLoadsFmtPackageAndTransitiveImports(t *testing.T) {
	source := `module main

import "fmt"

fn main() void {
    fmt.Println("Hello")
}
`

	diagnostics := analyze("file:///tmp/sec-lsp-import-test/main.sec", source)
	if len(diagnostics) == 0 {
		return
	}
	messages := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		messages = append(messages, diagnostic.Message)
	}
	t.Fatalf("analyze returned diagnostics for fmt package and transitive io import:\n%s", strings.Join(messages, "\n"))
}

func TestAnalyzeRecognizesBitBackedEnumSyntax(t *testing.T) {
	source := `module main

enum Flag: bit {
    Clear = 0,
    Set = 1,
}

enum Mode: bit[2] {
    Off = 0,
    On = 1,
}
`

	diagnostics := analyze("file:///tmp/sec-lsp-bit-test/main.sec", source)
	if len(diagnostics) == 0 {
		return
	}
	messages := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		messages = append(messages, diagnostic.Message)
	}
	t.Fatalf("analyze returned diagnostics for valid bit-backed enums:\n%s", strings.Join(messages, "\n"))
}

func TestAnalyzeGroupedPackageImportsAndShortNames(t *testing.T) {
	source := `module main

import (
    "platform/linux/amd64"
    "fmt"
)

fn main() int {
    fmt.Println("grouped")
    return int(amd64.sysCall.Write)
}
`

	diagnostics := analyze("file:///tmp/sec-lsp-grouped-import-test/main.sec", source)
	if len(diagnostics) == 0 {
		return
	}
	messages := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		messages = append(messages, diagnostic.Message)
	}
	t.Fatalf("analyze returned diagnostics for grouped package imports:\n%s", strings.Join(messages, "\n"))
}

func TestFormatSourceIndentsSwitchCaseBodies(t *testing.T) {
	input := `module main

fn Classify(value: int) int {
switch value {
case 0:
return 0
case 1, 2:
if value == 1 {
return 10
}
return 20
default:
// Unknown value.
return -1
}
}
`

	want := `module main

fn Classify(value: int) int {
    switch value {
        case 0:
            return 0
        case 1, 2:
            if value == 1 {
                return 10
            }
            return 20
        default:
            // Unknown value.
            return -1
    }
}
`

	if got := formatSource(input); got != want {
		t.Fatalf("formatSource() mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestFormatSourceIndentsGroupedImports(t *testing.T) {
	input := `module main

import (
"fmt"
sys "platform/linux/amd64"
)
`

	want := `module main

import (
    "fmt"
    sys "platform/linux/amd64"
)
`

	if got := formatSource(input); got != want {
		t.Fatalf("formatSource() mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestFormatSourceIndentsNestedSwitchCaseBodies(t *testing.T) {
	input := `fn Nested(outer: int, inner: bool) int {
switch outer {
case 1:
switch inner {
case true:
return 10
case false:
return 11
}
default:
return 0
}
}
`

	want := `fn Nested(outer: int, inner: bool) int {
    switch outer {
        case 1:
            switch inner {
                case true:
                    return 10
                case false:
                    return 11
            }
        default:
            return 0
    }
}
`

	if got := formatSource(input); got != want {
		t.Fatalf("formatSource() mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}
