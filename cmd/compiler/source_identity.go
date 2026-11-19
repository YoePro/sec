package main

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"sec/internal/ir/semantic"
	"sec/internal/project"
)

// canonicalizeIRSources gives a built Semantic IR module location-independent
// source identities through the same resolver that assigns module
// identities: the entry directory is the anonymous root, Project sources are
// Project-relative and compiler-provided sources are root-qualified. Compiler
// diagnostics keep the path the user wrote: later pipeline failures that
// quote an identity are mapped back through cliDiagnostics.
// Rules: rules/projects/projects.md — §§ 34, 50(1); rules/compiler/semantic_ir.md — § 5(1).
func canonicalizeIRSources(module *semantic.Module, inputFile string) error {
	resolver := project.NewSourceModuleResolver(findCompilerSourceRoot([]string{inputFile}), filepath.Dir(inputFile))
	original, err := semantic.CanonicalizeSourceFiles(module, resolver.SourceIdentity)
	if err != nil {
		return err
	}
	if cliDiagnostics.sourcePaths == nil {
		cliDiagnostics.sourcePaths = map[string]string{}
	}
	for identity, path := range original {
		cliDiagnostics.sourcePaths[identity] = path
	}
	return nil
}

// userSourcePaths rewrites each source identity that a pipeline failure
// quotes as `identity:line` back to the path the user wrote.
func (r *diagnosticReporter) userSourcePaths(text string) string {
	if len(r.sourcePaths) == 0 {
		return text
	}
	identities := make([]string, 0, len(r.sourcePaths))
	for identity := range r.sourcePaths {
		identities = append(identities, identity)
	}
	sort.Slice(identities, func(i, j int) bool { return len(identities[i]) > len(identities[j]) })
	for _, identity := range identities {
		if identity == r.sourcePaths[identity] {
			continue
		}
		pattern := regexp.MustCompile(`(^|[\s("'])` + regexp.QuoteMeta(identity) + `(:\d+)`)
		replacement := strings.ReplaceAll(r.sourcePaths[identity], "$", "$$")
		text = pattern.ReplaceAllString(text, "${1}"+replacement+"${2}")
	}
	return text
}

// userSourceFile maps one source identity back to the path the user wrote.
func (r *diagnosticReporter) userSourceFile(file string) string {
	if original, ok := r.sourcePaths[file]; ok {
		return original
	}
	return file
}
