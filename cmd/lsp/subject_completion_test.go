package main

import (
	"strings"
	"testing"
)

const subjectCompletionSource = `module main

type FileMode enum {
    OpenExisting,
    CreateNew,
    Truncate,
}

type Figure union {
    Circle(int),
    Square(int),
    Point,
}

type File struct {
    OpenMode: FileMode,
    Fallback: FileMode,
    Size: int,
}

impl File {
    fn DefaultMode() FileMode {
        return FileMode.OpenExisting
    }

    fn Check(shape: Figure, size: Option[int]) int {
        @
        switch self.OpenMode {
        case FileMode.CreateNew:
            return 1
        ~
        }
        let area := match shape {
            Circle(radius) => radius
            ^
        }
        let known := match size {
            Some(value) => value
            None => 0
        }
        return area + known
    }
}
`

func subjectCompletionLabels(t *testing.T, comparison string, caseLine string, arm string, marker string) string {
	t.Helper()
	source := strings.Replace(subjectCompletionSource, "@", comparison, 1)
	source = strings.Replace(source, "~", caseLine, 1)
	source = strings.Replace(source, "^", arm, 1)
	return strings.Join(completionLabelsAt(t, source, marker), ",")
}

func TestComparisonCompletionOffersOnlyLeftOperandType(t *testing.T) {
	if got := subjectCompletionLabels(t, "if self.OpenMode == |", "", "", "|"); got != "File.DefaultMode,FileMode.CreateNew,FileMode.OpenExisting,FileMode.Truncate" {
		t.Fatalf("== completion = %s", got)
	}
	if got := subjectCompletionLabels(t, "if self.OpenMode != self.|", "", "", "|"); got != "DefaultMode,Fallback,OpenMode" {
		t.Fatalf("!= self. completion = %s", got)
	}
}

func TestSwitchCaseCompletionOffersUncoveredSubjectMembers(t *testing.T) {
	if got := subjectCompletionLabels(t, "", "case |", "", "|"); got != "File.DefaultMode,FileMode.OpenExisting,FileMode.Truncate" {
		t.Fatalf("case completion = %s", got)
	}
}

func TestMatchArmCompletionOffersUncoveredVariants(t *testing.T) {
	if got := subjectCompletionLabels(t, "", "", "|", "|"); got != "Point,Square,_" {
		t.Fatalf("match arm completion = %s", got)
	}
}

func TestBlankLineOutsideSwitchAndMatchKeepsGlobalCompletion(t *testing.T) {
	source := "module main\n\nfn main() void {\n    let value := 1\n    |\n}\n"
	labels := completionLabelsAt(t, source, "|")
	found := false
	for _, label := range labels {
		found = found || label == "let"
	}
	if !found {
		t.Fatalf("global completion lost keywords: %v", labels)
	}
}

func TestMatchArmCompletionOffersOptionAndResultVariants(t *testing.T) {
	source := "module main\n\nfn Read(size: Option[int], parsed: Result[int, StringError]) int {\n    let known := match size {\n        Some(value) => value\n        |\n    }\n    let number := match parsed {\n        @\n    }\n    return known + number\n}\n"
	got := strings.Join(completionLabelsAt(t, strings.Replace(source, "@", "", 1), "|"), ",")
	if got != "None,_" {
		t.Fatalf("Option arm completion = %s", got)
	}
	got = strings.Join(completionLabelsAt(t, strings.Replace(source, "|", "", 1), "@"), ",")
	if got != "Err,Ok,_" {
		t.Fatalf("Result arm completion = %s", got)
	}
}
