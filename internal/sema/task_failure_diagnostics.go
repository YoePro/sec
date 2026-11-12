package sema

import (
	"fmt"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

const taskFailureDomains = "TaskSpawnError describes failed creation before a task exists. For an existing task, TaskOutcome[T] distinguishes Completed(T), Cancelled without a value, Panicked(PanicInfo), and Failed(TaskError). TaskError belongs to the execution/runtime failure of an already-created task. Completed(Err(E)) is a normal task-function return and preserves the application error inside T."

// TaskTypeExplanation projects canonical task failure/termination type facts
// into diagnostics and tooling. It never derives execution outcomes from an
// error's spelling, nor unwraps the callable's complete return type.
// Rules: rules/concurrency/tasks.md — §§5(8–9),11–15,30(1),(8–9),31(3–9);
// rules/concurrency/await.md — §§4–6.
func TaskTypeExplanation(typ Type) string {
	typ = dereferenceType(typ)
	// Result carriers can be synthesized without an intrinsic marker; the
	// canonical creation-error identity supplies the required provenance.
	if typ.Kind == ResultType && len(typ.TypeArgs) == 2 && typ.TypeArgs[0].Kind == StructType && typ.TypeArgs[0].Name == "Task" && typ.TypeArgs[1].Intrinsic && typ.TypeArgs[1].Kind == EnumType && typ.TypeArgs[1].Name == "TaskSpawnError" {
		return fmt.Sprintf("%s describes task creation. Use ordinary try or match on Ok/Err to obtain the owning %s before await. ", typeDisplayName(typ), typeDisplayName(typ.TypeArgs[0])) + taskFailureDomains
	}
	if !typ.Intrinsic {
		return ""
	}
	switch {
	case typ.Kind == EnumType && typ.Name == "TaskSpawnError":
		return "TaskSpawnError is a creation failure: no task was created. " + taskFailureDomains
	case typ.Kind == EnumType && typ.Name == "TaskError":
		return "TaskError is an execution/runtime failure after successful creation. InvalidConfiguration belongs to TaskSpawnError.InvalidConfiguration. " + taskFailureDomains
	case typ.Kind == UnionType && typ.Name == "TaskOutcome":
		completed := "Completed carries the callable's complete return type T."
		if len(typ.TypeArgs) == 1 {
			if typ.TypeArgs[0].Kind == VoidType {
				completed = "TaskOutcome[void].Completed carries no value; the other three outcome categories remain distinct."
			} else {
				completed = fmt.Sprintf("%s.Completed carries the complete %s return value.", typeDisplayName(typ), typeDisplayName(typ.TypeArgs[0]))
			}
		}
		return completed + " Match Completed, Cancelled, Panicked and Failed explicitly; ordinary try does not unwrap TaskOutcome. " + taskFailureDomains
	}
	return ""
}

// taskFailureMismatchHelp retains both expected and actual task layers in a
// type error. The ordinary compatibility checker remains the sole authority
// deciding whether there is an error at all.
// Rules: rules/concurrency/tasks.md — §§11(6–9),12(2–6),15(4–7),30(8–9).
func taskFailureMismatchHelp(expected, actual Type) string {
	expectedHelp, actualHelp := TaskTypeExplanation(expected), TaskTypeExplanation(actual)
	if expectedHelp == "" && actualHelp == "" {
		return ""
	}
	help := fmt.Sprintf("The destination requires %s; the expression has %s. ", typeDisplayName(expected), typeDisplayName(actual))
	if actualHelp != "" {
		return help + actualHelp
	}
	return help + expectedHelp
}

// addTaskCategoryError explains invalid canonical task outcome construction
// and error categories without changing unrelated enum/union diagnostics or
// inferring a runtime outcome. Missing-payload S1151 keeps its existing ID.
// Rules: rules/concurrency/tasks.md — §§11–15,30(1),(8–9);
// rules/declarations/unions.md — Construction.
func (a *Analyzer) addTaskCategoryError(token lexer.Token, owner Type, format string, args ...any) {
	if help := TaskTypeExplanation(owner); help != "" {
		a.addErrorAtTokenWithMetadata(token, diagnostics.TaskFailureDomainMismatch, help, format, args...)
		return
	}
	a.addErrorAtToken(token, format, args...)
}

// addTaskTypeMismatchError adds task-domain guidance at construction and
// terminal-return sites whose ordinary diagnostics otherwise stay unchanged.
// Rules: rules/concurrency/tasks.md — §30(1),(8–9).
func (a *Analyzer) addTaskTypeMismatchError(token lexer.Token, expected, actual Type, format string, args ...any) {
	if help := taskFailureMismatchHelp(expected, actual); help != "" {
		a.addErrorAtTokenWithMetadata(token, diagnostics.TaskFailureDomainMismatch, help, format, args...)
		return
	}
	a.addErrorAtToken(token, format, args...)
}
