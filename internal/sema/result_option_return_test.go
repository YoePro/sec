package sema

import "testing"

// Result success construction supplies its declared T as contextual type to
// Option constructors, including payload-less None.
//
// Rules:
//   - rules/errors/errorhandling.md — §5 "Ok and Err"
//   - rules/errors/errorhandling.md — §5.1 "Direct Option carrier returns"
//   - rules/declarations/unions.md — contextual generic union construction
func TestResultOkContextuallyResolvesOptionConstructors(t *testing.T) {
	errors := analyzeSource(t, `
enum LookupError error {
    Failed,
}

fn Missing() Result[Option[int], LookupError] {
    return Ok(None)
}

fn Found() Result[Option[int], LookupError] {
    return Ok(Some(42))
}

fn FromMatch(value: Option[int]) Result[Option[int], LookupError] {
    match value {
        Some(number) => {
            return Ok(Some(number))
        }
        None => {
            return Ok(None)
        }
    }
}
`)
	if len(errors) != 0 {
		t.Fatalf("contextual Result Option return errors = %+v", errors)
	}
}
