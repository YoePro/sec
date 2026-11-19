package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCLIHelpAliases(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-?"}, "Usage: yamlstatus [options]"},
		{[]string{"help"}, "Usage: yamlstatus [options]"},
		{[]string{"--help"}, "Usage: yamlstatus [options]"},
		{[]string{"help", "books"}, "--outdir"},
		{[]string{"books", "-?"}, "--outdir"},
		{[]string{"books", "--help"}, "--outdir"},
		{[]string{"help", "check"}, "yamlstatus check"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			var out, diag bytes.Buffer
			if err := runCLI(test.args, &out, &diag); err != nil {
				t.Fatal(err)
			}
			if out.Len() != 0 || !strings.Contains(diag.String(), test.want) {
				t.Fatalf("unexpected help: stdout=%q stderr=%q", out.String(), diag.String())
			}
		})
	}
	for _, args := range [][]string{{"help", "unknown"}, {"help", "books", "extra"}} {
		var out, diag bytes.Buffer
		if err := runCLI(args, &out, &diag); err == nil {
			t.Fatalf("accepted invalid help request %v", args)
		}
	}
}
