package sema

// TestingNamespaceMember is one source-visible overload of the compiler-known
// testing namespace that Sema validates. Tooling presents exactly these
// members instead of re-deriving the surface from the rulebook.
type TestingNamespaceMember struct {
	Name      string
	Kind      TestingOperationKind
	Signature string
	Summary   string
}

// TestingNamespaceMembers returns the implemented canonical 0.1 testing
// surface in rulebook order, one entry per overload. testing.Run and
// testing.TestResult are not listed until Sema implements them.
//
// Rules:
//   - rules/tooling/testing.md — §11.4 "Canonical 0.1 surface", §§12-18
func TestingNamespaceMembers() []TestingNamespaceMember {
	return []TestingNamespaceMember{
		{"Pass", TestingOperationPass, "Pass()", "Ends the current test successfully."},
		{"Fail", TestingOperationFail, "Fail(message: string)", "Ends the current test as failed with a message."},
		{"Skip", TestingOperationSkip, "Skip(message: string)", "Ends the current test as skipped with a reason."},
		{"Log", TestingOperationLog, "Log(message: string)", "Records test context without changing the outcome."},
		{"Expect", TestingOperationExpect, "Expect(condition: bool)", "Records a failure and continues when the condition is false."},
		{"Expect", TestingOperationExpect, "Expect(condition: bool, message: string)", "Records a failure with a message and continues when the condition is false."},
		{"Require", TestingOperationRequire, "Require(condition: bool)", "Fails and ends the current test when the condition is false."},
		{"Require", TestingOperationRequire, "Require(condition: bool, message: string)", "Fails with a message and ends the current test when the condition is false."},
		{"ExpectEqual", TestingOperationExpectEqual, "ExpectEqual(expected, actual)", "Records a failure and continues when the values differ."},
		{"ExpectEqual", TestingOperationExpectEqual, "ExpectEqual(expected, actual, message: string)", "Records a failure with a message and continues when the values differ."},
		{"RequireEqual", TestingOperationRequireEqual, "RequireEqual(expected, actual)", "Fails and ends the current test when the values differ."},
		{"RequireEqual", TestingOperationRequireEqual, "RequireEqual(expected, actual, message: string)", "Fails with a message and ends the current test when the values differ."},
	}
}
