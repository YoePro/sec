package debuginfo

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestCanonicalDebugLevels verifies the complete finite domain and preserves
// the same selected level through internal metadata encoding/decoding.
// Rules: rules/compiler/debug_information.md — §§3(1)-(4),4(1)-(4),70.
func TestCanonicalDebugLevels(t *testing.T) {
	expected := map[DebugInformationLevel]string{None: "None", LineTables: "LineTables", Full: "Full"}
	for value := 0; value <= 255; value++ {
		level := DebugInformationLevel(value)
		name, valid := expected[level]
		err := level.Validate()
		if (err == nil) != valid {
			t.Fatalf("level %d validity: %v", value, err)
		}
		if !valid {
			if data, err := json.Marshal(level); err == nil || data != nil {
				t.Fatalf("invalid level encoded successfully: %d %s %v", value, data, err)
			}
			if level.String() == None.String() {
				t.Fatalf("invalid level silently became None: %d", value)
			}
			continue
		}
		if level.String() != name {
			t.Fatal(level, name)
		}
		data, err := json.Marshal(level)
		if err != nil || string(data) != `"`+name+`"` {
			t.Fatal(string(data), err)
		}
		var decoded DebugInformationLevel
		if err := json.Unmarshal(data, &decoded); err != nil || decoded != level {
			t.Fatal(level, decoded, err)
		}
	}
}

// TestDebugLevelRejectsNoncanonicalMetadata prevents additional tiers, implicit
// aliases and missing selections from silently downgrading existing metadata.
// Rules: rules/compiler/debug_information.md — §§3(3)-(4),70.
func TestDebugLevelRejectsNoncanonicalMetadata(t *testing.T) {
	for _, text := range []string{"", "Limited", "LineDirectivesOnly", "DWARF", "CodeView", "none", "full", "Line Tables", " Full", "Full ", "1"} {
		level := Full
		if err := level.UnmarshalText([]byte(text)); err == nil || level != Full || !strings.Contains(err.Error(), "None, LineTables or Full") {
			t.Fatalf("%q: %v, level %v", text, err, level)
		}
	}
	for _, text := range []string{`0`, `1`, `2`, `3`, `"Limited"`, `true`} {
		level := Full
		if err := json.Unmarshal([]byte(text), &level); err == nil || level != Full {
			t.Fatalf("%s: %v, level %v", text, err, level)
		}
	}
}
