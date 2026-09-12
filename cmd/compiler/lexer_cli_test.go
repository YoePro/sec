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

func TestLexerCLICommandsReportInvalidNumericSuffixes(t *testing.T) {
	fixture := "../../testdata/lexer/invalid_numeric_suffix_invalid.sec"
	for _, command := range []string{"lex", "token"} {
		t.Run(command, func(t *testing.T) {
			process := exec.Command(os.Args[0], "-test.run=^TestLexerCLIProcess$", "--", command, fixture)
			process.Env = append(os.Environ(), "SEC_LEXER_CLI_TEST_PROCESS=1")
			output, err := process.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 2 {
				t.Fatalf("exit error = %v, want code 2; output: %s", err, output)
			}
			if strings.Count(string(output), diagnostics.LexerInvalidNumericSuffix) != 6 || !strings.Contains(string(output), "summary: 6 errors") {
				t.Fatalf("wrong numeric suffix diagnostics: %s", output)
			}
		})
	}
}

func TestLexerCLICommandsReportMissingExponentDigits(t *testing.T) {
	fixture := "../../testdata/lexer/missing_exponent_digits_invalid.sec"
	for _, command := range []string{"lex", "token"} {
		t.Run(command, func(t *testing.T) {
			process := exec.Command(os.Args[0], "-test.run=^TestLexerCLIProcess$", "--", command, fixture)
			process.Env = append(os.Environ(), "SEC_LEXER_CLI_TEST_PROCESS=1")
			output, err := process.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 2 {
				t.Fatalf("exit error = %v, want code 2; output: %s", err, output)
			}
			if strings.Count(string(output), diagnostics.LexerMissingExponentDigits) != 5 || !strings.Contains(string(output), "summary: 5 errors") {
				t.Fatalf("wrong exponent diagnostics: %s", output)
			}
		})
	}
}

func TestLexerCLICommandsReportMalformedDecimalCandidateMatrix(t *testing.T) {
	fixture := "../../testdata/lexer/malformed_decimal_candidates_invalid.sec"
	for _, command := range []string{"lex", "token"} {
		t.Run(command, func(t *testing.T) {
			process := exec.Command(os.Args[0], "-test.run=^TestLexerCLIProcess$", "--", command, fixture)
			process.Env = append(os.Environ(), "SEC_LEXER_CLI_TEST_PROCESS=1")
			output, err := process.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 2 {
				t.Fatalf("exit error = %v, want code 2; output: %s", err, output)
			}
			for id, count := range map[string]int{
				diagnostics.LexerInvalidDigitSeparator: 4,
				diagnostics.LexerInvalidNumericSuffix:  4,
				diagnostics.LexerMissingExponentDigits: 3,
			} {
				if strings.Count(string(output), id) != count {
					t.Errorf("%s count in output = %d, want %d: %s", id, strings.Count(string(output), id), count, output)
				}
			}
			if !strings.Contains(string(output), "summary: 11 errors") {
				t.Errorf("output lacks eleven-error summary: %s", output)
			}
		})
	}
}

func TestLexerCLICommandsReportUnterminatedBlockComment(t *testing.T) {
	fixture := "../../testdata/lexer/unterminated_block_comment_invalid.sec"
	for _, command := range []string{"lex", "token"} {
		t.Run(command, func(t *testing.T) {
			process := exec.Command(os.Args[0], "-test.run=^TestLexerCLIProcess$", "--", command, fixture)
			process.Env = append(os.Environ(), "SEC_LEXER_CLI_TEST_PROCESS=1")
			output, err := process.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 2 {
				t.Fatalf("exit error = %v, want code 2; output: %s", err, output)
			}
			if strings.Count(string(output), diagnostics.LexerUnterminatedBlockComment) != 1 || !strings.Contains(string(output), "summary: 1 error") {
				t.Fatalf("wrong unterminated comment diagnostics: %s", output)
			}
		})
	}
}

func TestLexerCLICommandsReportUnterminatedOrdinaryString(t *testing.T) {
	fixture := "../../testdata/lexer/unterminated_ordinary_string_invalid.sec"
	for _, command := range []string{"lex", "token"} {
		t.Run(command, func(t *testing.T) {
			process := exec.Command(os.Args[0], "-test.run=^TestLexerCLIProcess$", "--", command, fixture)
			process.Env = append(os.Environ(), "SEC_LEXER_CLI_TEST_PROCESS=1")
			output, err := process.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 2 {
				t.Fatalf("exit error = %v, want code 2; output: %s", err, output)
			}
			if strings.Count(string(output), diagnostics.LexerUnterminatedOrdinaryString) != 1 || !strings.Contains(string(output), "summary: 1 error") {
				t.Fatalf("wrong unterminated ordinary string diagnostics: %s", output)
			}
		})
	}
}

func TestLexerCLICommandsReportUnterminatedRawString(t *testing.T) {
	fixture := "../../testdata/lexer/unterminated_raw_string_invalid.sec"
	for _, command := range []string{"lex", "token"} {
		t.Run(command, func(t *testing.T) {
			process := exec.Command(os.Args[0], "-test.run=^TestLexerCLIProcess$", "--", command, fixture)
			process.Env = append(os.Environ(), "SEC_LEXER_CLI_TEST_PROCESS=1")
			output, err := process.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 2 {
				t.Fatalf("exit error = %v, want code 2; output: %s", err, output)
			}
			if strings.Count(string(output), diagnostics.LexerUnterminatedRawString) != 1 || !strings.Contains(string(output), "summary: 1 error") {
				t.Fatalf("wrong unterminated raw string diagnostics: %s", output)
			}
		})
	}
}

func TestLexerCLICommandsReportUnterminatedCharacterLiteral(t *testing.T) {
	fixture := "../../testdata/lexer/unterminated_character_literal_invalid.sec"
	for _, command := range []string{"lex", "token"} {
		t.Run(command, func(t *testing.T) {
			process := exec.Command(os.Args[0], "-test.run=^TestLexerCLIProcess$", "--", command, fixture)
			process.Env = append(os.Environ(), "SEC_LEXER_CLI_TEST_PROCESS=1")
			output, err := process.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 2 {
				t.Fatalf("exit error = %v, want code 2; output: %s", err, output)
			}
			if strings.Count(string(output), diagnostics.LexerUnterminatedCharacterLiteral) != 1 || !strings.Contains(string(output), "summary: 1 error") {
				t.Fatalf("wrong unterminated character literal diagnostics: %s", output)
			}
		})
	}
}

func TestLexerCLICommandsReportUnterminatedInterpolatedString(t *testing.T) {
	fixture := "../../testdata/lexer/unterminated_interpolated_string_invalid.sec"
	for _, command := range []string{"lex", "token"} {
		t.Run(command, func(t *testing.T) {
			process := exec.Command(os.Args[0], "-test.run=^TestLexerCLIProcess$", "--", command, fixture)
			process.Env = append(os.Environ(), "SEC_LEXER_CLI_TEST_PROCESS=1")
			output, err := process.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 2 {
				t.Fatalf("exit error = %v, want code 2; output: %s", err, output)
			}
			if strings.Count(string(output), diagnostics.LexerUnterminatedInterpolatedString) != 1 || !strings.Contains(string(output), "summary: 1 error") {
				t.Fatalf("wrong unterminated interpolated string diagnostics: %s", output)
			}
		})
	}
}

func TestLexerCLICommandsReportInvalidSourceCharacter(t *testing.T) {
	fixture := "../../testdata/lexer/invalid_source_character_invalid.sec"
	for _, command := range []string{"lex", "token"} {
		t.Run(command, func(t *testing.T) {
			process := exec.Command(os.Args[0], "-test.run=^TestLexerCLIProcess$", "--", command, fixture)
			process.Env = append(os.Environ(), "SEC_LEXER_CLI_TEST_PROCESS=1")
			output, err := process.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 2 {
				t.Fatalf("exit error = %v, want code 2; output: %s", err, output)
			}
			if strings.Count(string(output), diagnostics.LexerInvalidSourceCharacter) != 2 || !strings.Contains(string(output), "summary: 2 errors") {
				t.Fatalf("wrong invalid source character diagnostics: %s", output)
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
