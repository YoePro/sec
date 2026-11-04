package formatconfig

import (
	"math"
	"testing"
)

// Presentation context affects only a project's adaptive mode. Width resolution
// preserves its immutable settings and never converts an arbitrary uint64 into
// a host-sized int. Missing observations use the project fallback and range.
// Rules: rules/tooling/formatter.md — §4(9–15,20–23).
func TestEffectiveFormatterWidth(t *testing.T) {
	for _, mode := range []string{"fixed", "adaptive"} {
		settings, err := Resolve("sec.toml", "[format]\nwidth_mode = \""+mode+"\"\nline_width = 72\nadaptive_min_width = 64\nadaptive_max_width = 120\nadaptive_fallback_width = 80\n", Overrides{})
		if err != nil {
			t.Fatal(err)
		}
		before := settings
		for _, test := range []struct{ observed, adaptive uint64 }{{0, 80}, {1, 64}, {64, 64}, {100, 100}, {120, 120}, {121, 120}, {math.MaxUint64, 120}} {
			want := uint64(72)
			if mode == "adaptive" {
				want = test.adaptive
			}
			options := settings.EffectiveOptions(test.observed)
			if options.LineWidth != want || settings != before {
				t.Fatal(mode, test, options, settings)
			}
			options.LineWidth = 1
			if settings != before {
				t.Fatal("request options mutated settings")
			}
		}
	}
	for _, fallback := range []string{"1", "999"} {
		settings, err := Resolve("sec.toml", "[format]\nwidth_mode = \"adaptive\"\nadaptive_min_width = 64\nadaptive_max_width = 120\nadaptive_fallback_width = "+fallback+"\n", Overrides{})
		if err != nil {
			t.Fatal(err)
		}
		want := uint64(64)
		if fallback == "999" {
			want = 120
		}
		if got := settings.EffectiveOptions(0).LineWidth; got != want {
			t.Fatal("fallback escaped project range", got)
		}
	}
	if got := Defaults().EffectiveOptions(999); got.LineWidth != 120 || got.Fix {
		t.Fatal("default configuration changed", got)
	}
}
