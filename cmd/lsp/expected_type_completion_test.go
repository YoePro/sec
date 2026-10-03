package main

import (
	"sort"
	"strings"
	"testing"
)

func completionLabelsAt(t *testing.T, source string, marker string) []string {
	t.Helper()
	offset := strings.Index(source, marker)
	if offset < 0 {
		t.Fatalf("missing marker %q", marker)
	}
	text := strings.Replace(source, marker, "", 1)
	labels := []string{}
	for _, item := range completeSource("", text, offset) {
		labels = append(labels, item.Label)
	}
	sort.Strings(labels)
	return labels
}

const expectedTypeCompletionSource = `module main

type FileMode enum {
    Read,
    Write,
}

type File struct {
    _mode: FileMode,
    _fallback: FileMode,
    _path: string,
    _size: int,
}

impl File {
    fn DefaultMode() FileMode {
        return FileMode.Read
    }

    fn Length() int {
        return self._size
    }

    property Mode: FileMode {
        get {
            return self.|
        }
    }

    fn Describe() string {
        let size := self.@
        return self._path
    }
}
`

func TestMemberCompletionInPropertyGetterReturnOffersOnlyExpectedType(t *testing.T) {
	source := strings.Replace(expectedTypeCompletionSource, "@", "", 1)
	got := strings.Join(completionLabelsAt(t, source, "|"), ",")
	if got != "DefaultMode,_fallback,_mode" {
		t.Fatalf("getter return completion = %s", got)
	}
}

func TestMemberCompletionOutsideReturnOffersAllMembers(t *testing.T) {
	source := strings.Replace(expectedTypeCompletionSource, "|", "", 1)
	labels := completionLabelsAt(t, source, "@")
	for _, want := range []string{"_mode", "_path", "_size", "Length", "Describe", "Mode"} {
		found := false
		for _, label := range labels {
			found = found || label == want
		}
		if !found {
			t.Fatalf("missing %s in %v", want, labels)
		}
	}
}

func TestMemberCompletionInFunctionReturnFiltersByReturnType(t *testing.T) {
	source := strings.Replace(strings.Replace(expectedTypeCompletionSource, "|", "", 1), "@", "", 1)
	source = strings.Replace(source, "return self._path", "return self.#", 1)
	got := strings.Join(completionLabelsAt(t, source, "#"), ",")
	if got != "Describe,_path" {
		t.Fatalf("string return completion = %s", got)
	}
}
