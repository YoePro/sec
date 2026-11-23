package main

import (
	"os"
	"strings"
	"testing"
)

// Editor diagnostics preserve independent later errors while the incomplete
// lambda signature remains invalid, without pretending its return is void.
// Rules: rules/compiler/parser_recovery.md §§14.7,15,46.2,82.2;
// rules/tooling/lsp.md — Recovery nodes.
func TestLambdaReturnRecoveryLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/lambda_return_recovery/missing_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	syntax, independent := 0, 0
	for _, d := range analyze("file:///lambda-recovery/main.sec", string(data)) {
		if d.Severity != 1 {
			continue
		}
		if strings.Contains(d.Message, "lambda return type is required") {
			syntax++
		} else {
			independent++
		}
	}
	if syntax != 2 || independent != 2 {
		t.Fatalf("syntax %d independent %d", syntax, independent)
	}
}
