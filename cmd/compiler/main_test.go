package main

import "testing"

func TestStdlibModuleName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "fmt", want: "fmt"},
		{path: "std/fmt", want: "fmt"},
		{path: "io", want: "io"},
		{path: "std/io", want: "io"},
	}

	for _, tt := range tests {
		if got := stdlibModuleName(tt.path); got != tt.want {
			t.Fatalf("stdlibModuleName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestStdlibModulePath(t *testing.T) {
	got := stdlibModulePath("fmt")
	want := "sec/stdlib/fmt/fmt.sec"
	if got != want {
		t.Fatalf("stdlibModulePath(%q) = %q, want %q", "fmt", got, want)
	}
}
