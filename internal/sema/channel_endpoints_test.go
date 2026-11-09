package sema

import (
	"os"
	"strings"
	"testing"
)

// Canonical endpoints are stored fields, with normal copy prohibition and
// independent field moves; legacy spellings name the exact replacement.
// Rules: rules/concurrency/channels.md §§3(3),15,21,57(3),62–63.
func TestCanonicalChannelEndpoints(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"channel_endpoints_valid.sec", 0}, {"channel_endpoints_invalid.sec", 7},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		_, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != tc.errors {
			t.Fatalf("%s: %v", tc.file, errors)
		}
		if tc.errors > 0 {
			for i, want := range []string{"use Tx", "use Rx", "cannot copy", "moved", "moved", "moved", "moved"} {
				if !strings.Contains(errors[i].Message, want) {
					t.Fatalf("%d: %v, want %s", i, errors[i], want)
				}
			}
		}
	}
}
