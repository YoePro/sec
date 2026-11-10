package main

import (
	"os"
	"strings"
	"testing"
)

// TestOrdinaryReceiverContractLSP shares compiler whole-self restrictions while retaining valid callable/member transfers.
// Rules: rules/memory/ownership.md — §9; rules/tooling/lsp.md — diagnostics.
func TestOrdinaryReceiverContractLSP(t *testing.T) {
	uri := "file:///ordinary-receivers/main.sec"
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"ordinary_receiver_contract_valid.sec", 0}, {"ordinary_receiver_contract_invalid.sec", 3},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity != 1 {
				continue
			}
			count++
			if !strings.Contains(d.Message, "ordinary method cannot consume complete self") {
				t.Fatal(d)
			}
		}
		if count != tc.errors {
			t.Fatal(tc.file, count)
		}
	}
}
