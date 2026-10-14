package main

import "strings"

// legacyMLIRRuntimeLibraries materializes the legacy floating-remainder runtime
// dependency after the generated object and before native linking. LLVM may
// lower frem to fmod/fmodf: Linux and FreeBSD provide these in libm, whereas
// Darwin and Windows provide them through their default system/C runtime.
// This uses the selected target OS, never the compiler host OS.
//
// Rules:
//   - rules/foundations/operators.md — "Floating remainder"
//   - rules/compiler/linking.md — §§17,28,56 (runtime requirements and ordering)
func legacyMLIRRuntimeLibraries(text, targetOS string) []string {
	if targetOS != "linux" && targetOS != "freebsd" {
		return nil
	}
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 3 && strings.HasPrefix(fields[0], "%") && fields[1] == "=" && fields[2] == "llvm.frem" {
			return []string{"-lm"}
		}
	}
	return nil
}
