package main

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

func TestLexerCLICommandsRejectRetainedTokenDiagnostics(t *testing.T) {
	fixture := "../../testdata/lexer/cli_diagnostics_invalid.sec"
	for _, command := range []string{"lex", "token"} {
		t.Run(command, func(t *testing.T) {
			process := exec.Command(os.Args[0], "-test.run=^TestLexerCLIProcess$", "--", command, fixture)
			process.Env = append(os.Environ(), "SEC_LEXER_CLI_TEST_PROCESS=1")
			output, err := process.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 2 {
				t.Fatalf("exit error = %v, want code 2; output: %s", err, output)
			}
			for _, want := range []string{diagnostics.LexerUnknownEscape, diagnostics.LexerUnsupportedWhitespace, "summary: 3 errors"} {
				if !strings.Contains(string(output), want) {
					t.Errorf("output %q does not contain %q", output, want)
				}
			}
		})
	}
}

func TestLexerCLICommandsReportMalformedBaseIDs(t *testing.T) {
	fixture := "../../testdata/lexer/malformed_base_literals_invalid.sec"
	for _, command := range []string{"lex", "token"} {
		t.Run(command, func(t *testing.T) {
			process := exec.Command(os.Args[0], "-test.run=^TestLexerCLIProcess$", "--", command, fixture)
			process.Env = append(os.Environ(), "SEC_LEXER_CLI_TEST_PROCESS=1")
			output, err := process.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 2 {
				t.Fatalf("exit error = %v, want code 2; output: %s", err, output)
			}
			for _, want := range []string{diagnostics.LexerMalformedBaseLiteral, diagnostics.LexerInvalidBaseDigit, diagnostics.LexerInvalidDigitSeparator, "summary: 5 errors"} {
				if !strings.Contains(string(output), want) {
					t.Errorf("output %q does not contain %q", output, want)
				}
			}
		})
	}
}

func TestLexerCLICommandsReportInvalidDigitSeparators(t *testing.T) {
	fixture := "../../testdata/lexer/digit_separators_invalid.sec"
	for _, command := range []string{"lex", "token"} {
		t.Run(command, func(t *testing.T) {
			process := exec.Command(os.Args[0], "-test.run=^TestLexerCLIProcess$", "--", command, fixture)
			process.Env = append(os.Environ(), "SEC_LEXER_CLI_TEST_PROCESS=1")
			output, err := process.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 2 {
				t.Fatalf("exit error = %v, want code 2; output: %s", err, output)
			}
			if strings.Count(string(output), diagnostics.LexerInvalidDigitSeparator) != 8 || !strings.Contains(string(output), "summary: 8 errors") {
				t.Fatalf("wrong separator diagnostics: %s", output)
			}
		})
	}
}

func TestLexerCLIProcess(t *testing.T) {
	if os.Getenv("SEC_LEXER_CLI_TEST_PROCESS") != "1" {
		return
	}
	os.Args = append(os.Args[:1], os.Args[3:]...)
	flag.CommandLine = flag.NewFlagSet("sec", flag.ExitOnError)
	main()
	os.Exit(0)
}

func TestLexerCLIDiagnosticsIncludeRetainedTokensWithoutDoubleCounting(t *testing.T) {
	input, err := os.ReadFile("../../testdata/lexer/cli_diagnostics_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	l := lexer.NewWithFile(string(input), "cli_diagnostics_invalid.sec")
	illegalTokens := []lexer.Token{}
	for token := l.NextToken(); token.Type != lexer.EOF; token = l.NextToken() {
		if token.Type == lexer.ILLEGAL {
			illegalTokens = append(illegalTokens, token)
		}
	}

	var output bytes.Buffer
	summary := reportLexerCLIDiagnostics(&output, l.Diagnostics(), illegalTokens)
	if summary.Errors != 3 {
		t.Fatalf("errors = %d, want retained escape + Unicode whitespace + unstructured illegal token; output: %s", summary.Errors, output.String())
	}
	for _, want := range []string{
		"lex error: " + diagnostics.LexerUnknownEscape + " at cli_diagnostics_invalid.sec:1:10:",
		"lex error: " + diagnostics.LexerUnsupportedWhitespace + " at cli_diagnostics_invalid.sec:2:1:",
	} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("output %q does not contain %q", output.String(), want)
		}
	}
}

func TestLexerCLIDiagnosticsCountInvalidUTF8Once(t *testing.T) {
	l := lexer.NewWithFile(string([]byte{0xff}), "invalid.sec")
	token := l.NextToken()
	if token.Type != lexer.ILLEGAL {
		t.Fatalf("token = %+v, want ILLEGAL", token)
	}

	var output bytes.Buffer
	summary := reportLexerCLIDiagnostics(&output, l.Diagnostics(), []lexer.Token{token})
	if summary.Errors != 1 {
		t.Fatalf("errors = %d, want 1; output: %s", summary.Errors, output.String())
	}
	if !strings.Contains(output.String(), "lex error: "+diagnostics.LexerInvalidUTF8+" at invalid.sec:1:1:") {
		t.Fatalf("missing invalid UTF-8 diagnostic: %s", output.String())
	}
}

func TestLexerCLIDiagnosticsInsideIllegalTokenCountOnce(t *testing.T) {
	l := lexer.New(`"\`)
	token := l.NextToken()
	if token.Type != lexer.ILLEGAL {
		t.Fatalf("token = %+v, want ILLEGAL", token)
	}

	var output bytes.Buffer
	summary := reportLexerCLIDiagnostics(&output, l.Diagnostics(), []lexer.Token{token})
	if summary.Errors != 1 {
		t.Fatalf("errors = %d, want one incomplete-escape error; output: %s", summary.Errors, output.String())
	}
	if !strings.Contains(output.String(), diagnostics.LexerMalformedEscape+" at 1:2:") {
		t.Fatalf("missing malformed escape diagnostic: %s", output.String())
	}
}
