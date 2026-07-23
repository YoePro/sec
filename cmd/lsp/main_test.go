package main

import "testing"

func TestFormatSourceIndentsSwitchCaseBodies(t *testing.T) {
	input := `module main

fn Classify(value: int) int {
switch value {
case 0:
return 0
case 1, 2:
if value == 1 {
return 10
}
return 20
default:
// Unknown value.
return -1
}
}
`

	want := `module main

fn Classify(value: int) int {
    switch value {
        case 0:
            return 0
        case 1, 2:
            if value == 1 {
                return 10
            }
            return 20
        default:
            // Unknown value.
            return -1
    }
}
`

	if got := formatSource(input); got != want {
		t.Fatalf("formatSource() mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestFormatSourceIndentsNestedSwitchCaseBodies(t *testing.T) {
	input := `fn Nested(outer: int, inner: bool) int {
switch outer {
case 1:
switch inner {
case true:
return 10
case false:
return 11
}
default:
return 0
}
}
`

	want := `fn Nested(outer: int, inner: bool) int {
    switch outer {
        case 1:
            switch inner {
                case true:
                    return 10
                case false:
                    return 11
            }
        default:
            return 0
    }
}
`

	if got := formatSource(input); got != want {
		t.Fatalf("formatSource() mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}
