package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"sec/internal/platform/cbindings"
	"sec/internal/project"
)

// cBindingsUsage lists the implemented `sec platform c-bindings` commands.
const cBindingsUsage = `usage: sec platform c-bindings init --id ID --version VERSION --source-root DIR [--file FILE] [--json]
       sec platform c-bindings show [--file FILE] [--json]
       sec platform c-bindings validate [--file FILE] [--variant NAME] [--json]
       sec platform c-bindings resolve --variant NAME [--file FILE] [--json]
       sec platform c-bindings package set [--id ID] [--version VERSION] [--source-root DIR] [--file FILE] [--if-match SHA256] [--json]
       sec platform c-bindings environment add NAME --os OS --arch ARCH --abi ABI --c-runtime RUNTIME [--c-runtime-versions RANGE] [--file FILE] [--if-match SHA256] [--json]
       sec platform c-bindings environment set NAME [--os OS] [--arch ARCH] [--abi ABI] [--c-runtime RUNTIME] [--c-runtime-versions RANGE | --clear-c-runtime-versions] [--file FILE] [--if-match SHA256] [--json]
       sec platform c-bindings environment remove NAME [--file FILE] [--if-match SHA256] [--json]
       sec platform c-bindings module add PATH (--common | --environment NAME) [--file FILE] [--if-match SHA256] [--json]
       sec platform c-bindings module remove PATH (--common | --environment NAME) [--file FILE] [--if-match SHA256] [--json]`

// cBindingsResult is the sec.c-bindings-cli JSON document (cli.md § 8(3)).
type cBindingsResult struct {
	Format      string                 `json:"format"`
	Version     int                    `json:"version"`
	Operation   string                 `json:"operation"`
	Status      string                 `json:"status"`
	Changed     bool                   `json:"changed"`
	Reason      string                 `json:"reason,omitempty"`
	File        string                 `json:"file"`
	BeforeSHA   *string                `json:"before_sha256,omitempty"`
	AfterSHA    *string                `json:"after_sha256,omitempty"`
	FileSHA     *string                `json:"file_sha256,omitempty"`
	Scope       string                 `json:"scope,omitempty"`
	Resolvable  *bool                  `json:"resolvable,omitempty"`
	Package     *cBindingsPackageJSON  `json:"package,omitempty"`
	Binding     *cbindings.Resolution  `json:"binding,omitempty"`
	Diagnostics []cbindings.Diagnostic `json:"diagnostics"`
}

type cBindingsPackageJSON struct {
	SchemaVersion int                                 `json:"schema_version"`
	ID            string                              `json:"id"`
	Version       string                              `json:"version"`
	SourceRoot    string                              `json:"source_root"`
	CommonModules []string                            `json:"common_modules"`
	Environments  map[string]cBindingsEnvironmentJSON `json:"environments"`
}

type cBindingsEnvironmentJSON struct {
	OS               string   `json:"os"`
	Arch             string   `json:"arch"`
	ABI              string   `json:"abi"`
	CRuntime         string   `json:"c_runtime"`
	CRuntimeVersions string   `json:"c_runtime_versions,omitempty"`
	Modules          []string `json:"modules"`
}

// cBindingsResolutionReport is the sec.c-bindings-resolution document
// (MD-021 correction § 8.9(3)).
type cBindingsResolutionReport struct {
	Format      string                 `json:"format"`
	Version     int                    `json:"version"`
	Status      string                 `json:"status"`
	Variant     string                 `json:"variant"`
	Target      map[string]string      `json:"target"`
	Binding     *cbindings.Resolution  `json:"binding"`
	Diagnostics []cbindings.Diagnostic `json:"diagnostics"`
}

// runPlatformCommand dispatches `sec platform <group> ...` and returns the
// exit status: 0 success, 1 usage or I/O failure, 2 rejected by validation
// or resolution.
// Rules: rules/tooling/cli.md — §§ 3, 5.6, 8; MD-021 correction §§ 4, 5, 8.9.
func runPlatformCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 || args[0] != "c-bindings" {
		fmt.Fprintln(stderr, cBindingsUsage)
		return 1
	}
	operation := args[1]
	rest := args[2:]
	if operation == "package" || operation == "environment" || operation == "module" {
		if len(rest) == 0 {
			fmt.Fprintf(stderr, "sec platform c-bindings %s: missing subcommand\n%s\n", operation, cBindingsUsage)
			return 1
		}
		operation, rest = operation+"."+rest[0], rest[1:]
	}
	var positional string
	if operation != "package.set" && strings.Contains(operation, ".") {
		if len(rest) == 0 || strings.HasPrefix(rest[0], "--") {
			fmt.Fprintf(stderr, "sec platform c-bindings %s: missing NAME or PATH\n%s\n", strings.ReplaceAll(operation, ".", " "), cBindingsUsage)
			return 1
		}
		positional, rest = rest[0], rest[1:]
	}
	options, err := parseCBindingsOptions(rest)
	if err != nil {
		fmt.Fprintf(stderr, "sec platform c-bindings %s: %v\n%s\n", operation, err, cBindingsUsage)
		return 1
	}
	switch operation {
	case "init":
		return runCBindingsInit(options, stdout, stderr)
	case "show":
		return runCBindingsShow(options, stdout, stderr)
	case "validate":
		return runCBindingsValidate(options, stdout, stderr)
	case "resolve":
		return runCBindingsResolve(options, stdout, stderr)
	case "package.set", "environment.add", "environment.set", "environment.remove", "module.add", "module.remove":
		op, err := cBindingsOperation(operation, positional, options)
		if err != nil {
			fmt.Fprintf(stderr, "sec platform c-bindings %s: %v\n%s\n", strings.ReplaceAll(operation, ".", " "), err, cBindingsUsage)
			return 1
		}
		return runCBindingsMutation(op, options, stdout, stderr)
	}
	fmt.Fprintf(stderr, "unknown c-bindings command %q\n%s\n", operation, cBindingsUsage)
	return 1
}

type cBindingsOptions struct {
	file, id, version, sourceRoot, variant    string
	os, arch, abi, cRuntime, cRuntimeVersions string
	environment, ifMatch                      string
	json, common, clearCRuntimeVersions       bool
	given                                     map[string]bool
}

// parseCBindingsOptions reads the flags shared by the read-only commands.
// Rules: rules/tooling/cli.md — § 8(1).
func parseCBindingsOptions(args []string) (cBindingsOptions, error) {
	options := cBindingsOptions{file: cbindings.DefaultFile, given: map[string]bool{}}
	for index := 0; index < len(args); index++ {
		flag := args[index]
		if switches := map[string]*bool{"--json": &options.json, "--common": &options.common,
			"--clear-c-runtime-versions": &options.clearCRuntimeVersions}[flag]; switches != nil {
			*switches = true
			options.given[flag] = true
			continue
		}
		target := map[string]*string{"--file": &options.file, "--id": &options.id, "--version": &options.version,
			"--source-root": &options.sourceRoot, "--variant": &options.variant, "--os": &options.os, "--arch": &options.arch,
			"--abi": &options.abi, "--c-runtime": &options.cRuntime, "--c-runtime-versions": &options.cRuntimeVersions,
			"--environment": &options.environment, "--if-match": &options.ifMatch}[flag]
		if target == nil {
			return options, fmt.Errorf("unknown argument %q", flag)
		}
		if options.given[flag] {
			return options, fmt.Errorf("%s given more than once", flag)
		}
		options.given[flag] = true
		if index+1 >= len(args) {
			return options, fmt.Errorf("%s needs a value", flag)
		}
		index++
		*target = args[index]
	}
	return options, nil
}

// fileSHA256 hashes raw file bytes; nil when the file is absent.
// Rules: MD-021 correction § 5(1).
func fileSHA256(data []byte, present bool) *string {
	if !present {
		return nil
	}
	sum := sha256.Sum256(data)
	text := hex.EncodeToString(sum[:])
	return &text
}

// readCBindingsFile reads and parses the metadata file.
// Rules: MD-021 correction §§ 2(8), 5(7).
func readCBindingsFile(file string) ([]byte, *cbindings.Package, []cbindings.Diagnostic, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, nil, err
	}
	pkg, diagnostics := cbindings.Parse(string(data))
	return data, pkg, diagnostics, nil
}

// emitCBindings writes one JSON document or a short human report.
// Rules: rules/tooling/cli.md — § 8(3).
func emitCBindings(result cBindingsResult, asJSON bool, stdout io.Writer, human string) {
	if result.Diagnostics == nil {
		result.Diagnostics = []cbindings.Diagnostic{}
	}
	if asJSON {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(result)
		return
	}
	fmt.Fprintln(stdout, human)
	for _, d := range result.Diagnostics {
		fmt.Fprintf(stdout, "%s: %s\n", d.ID, d.Message)
	}
}

// runCBindingsInit creates the metadata file, or is a no-op when an
// identical package already exists; an incompatible file is never replaced.
// Rules: MD-021 correction §§ 4(4), 5(3)-(4).
func runCBindingsInit(options cBindingsOptions, stdout, stderr io.Writer) int {
	result := cBindingsResult{Format: "sec.c-bindings-cli", Version: 1, Operation: "init", File: options.file}
	pkg := &cbindings.Package{SchemaVersion: cbindings.SchemaVersion, ID: options.id, Version: options.version, SourceRoot: options.sourceRoot}
	if diagnostics := pkg.Validate(); len(diagnostics) != 0 {
		result.Status, result.Diagnostics = "rejected", diagnostics
		emitCBindings(result, options.json, stdout, "init rejected")
		return 2
	}
	if data, existing, diagnostics, err := readCBindingsFile(options.file); err == nil {
		sha := fileSHA256(data, true)
		result.BeforeSHA, result.AfterSHA = sha, sha
		if len(diagnostics) == 0 && existing.ID == pkg.ID && existing.Version == pkg.Version && existing.SourceRoot == pkg.SourceRoot {
			result.Status = "complete"
			emitCBindings(result, options.json, stdout, "c-bindings already initialized; unchanged")
			return 0
		}
		result.Status = "rejected"
		result.Diagnostics = append(diagnostics, cbindings.Diagnostic{ID: "c-bindings.file-exists", Message: fmt.Sprintf("%s already exists with a different package; use package set to change it", options.file)})
		emitCBindings(result, options.json, stdout, "init rejected")
		return 2
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(stderr, "sec platform c-bindings init: %v\n", err)
		return 1
	}
	text := fmt.Sprintf("schema_version = 1\n\n[bindings]\nid = %q\nversion = %q\nsource_root = %q\n", pkg.ID, pkg.Version, pkg.SourceRoot)
	if err := writeFileAtomically(options.file, []byte(text)); err != nil {
		fmt.Fprintf(stderr, "sec platform c-bindings init: %v\n", err)
		return 1
	}
	result.Status, result.Changed, result.AfterSHA = "complete", true, fileSHA256([]byte(text), true)
	emitCBindings(result, options.json, stdout, "created "+options.file)
	return 0
}

// writeFileAtomically writes through a temporary file in the same directory
// and renames it into place, so a failure never leaves a partial file.
// Rules: MD-021 correction § 5(3).
func writeFileAtomically(file string, data []byte) error {
	return replaceFileAtomically(file, data, nil)
}

// errStaleFile reports that the file changed after it was read.
var errStaleFile = errors.New("the file changed after it was read")

// replaceFileAtomically writes through a temporary file in the same
// directory, re-hashes the current file immediately before the rename when
// expected is set (nil requires the file to be absent), and keeps the
// existing permission bits; new files get 0644.
// Rules: MD-021 correction §§ 5(2)-(3).
func replaceFileAtomically(file string, data []byte, expected *string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(file); err == nil {
		mode = info.Mode().Perm()
	}
	temporary, err := os.CreateTemp(filepath.Dir(file), ".c-bindings-*.toml")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		return err
	}
	if expected != nil {
		current, err := os.ReadFile(file)
		if err != nil || *fileSHA256(current, true) != *expected {
			return errStaleFile
		}
	}
	return os.Rename(name, file)
}

// runCBindingsShow prints the complete model and the raw-file SHA-256.
// Rules: MD-021 correction § 5(1).
func runCBindingsShow(options cBindingsOptions, stdout, stderr io.Writer) int {
	data, pkg, diagnostics, err := readCBindingsFile(options.file)
	if err != nil {
		fmt.Fprintf(stderr, "sec platform c-bindings show: %v\n", err)
		return 1
	}
	result := cBindingsResult{Format: "sec.c-bindings-cli", Version: 1, Operation: "show", Status: "complete", File: options.file,
		FileSHA: fileSHA256(data, true), Diagnostics: diagnostics}
	if pkg != nil {
		result.Package = packageJSON(pkg)
	}
	if len(diagnostics) != 0 {
		result.Status = "rejected"
	}
	human := "c-bindings package"
	if pkg != nil {
		human = fmt.Sprintf("%s %s (source root %s), %d environments", pkg.ID, pkg.Version, pkg.SourceRoot, len(pkg.Environments))
	}
	emitCBindings(result, options.json, stdout, human)
	if len(diagnostics) != 0 {
		return 2
	}
	return 0
}

func packageJSON(pkg *cbindings.Package) *cBindingsPackageJSON {
	out := &cBindingsPackageJSON{SchemaVersion: pkg.SchemaVersion, ID: pkg.ID, Version: pkg.Version, SourceRoot: pkg.SourceRoot,
		CommonModules: append([]string{}, pkg.CommonModules...), Environments: map[string]cBindingsEnvironmentJSON{}}
	for name, environment := range pkg.Environments {
		out.Environments[name] = cBindingsEnvironmentJSON{environment.OS, environment.Arch, environment.ABI, environment.CRuntime,
			environment.CRuntimeVersions, append([]string{}, environment.Modules...)}
	}
	return out
}

// runCBindingsValidate checks the file structurally, or with --variant runs
// the full resolution for that Variant.
// Rules: MD-021 correction §§ 5(7), 8.9(1)-(2).
func runCBindingsValidate(options cBindingsOptions, stdout, stderr io.Writer) int {
	data, pkg, diagnostics, err := readCBindingsFile(options.file)
	if err != nil {
		fmt.Fprintf(stderr, "sec platform c-bindings validate: %v\n", err)
		return 1
	}
	result := cBindingsResult{Format: "sec.c-bindings-cli", Version: 1, Operation: "validate", Status: "complete", File: options.file,
		FileSHA: fileSHA256(data, true), Scope: "structural", Diagnostics: diagnostics}
	resolvable := pkg != nil && len(diagnostics) == 0 && pkg.Resolvable()
	result.Resolvable = &resolvable
	if options.variant != "" && len(diagnostics) == 0 {
		result.Scope = "variant"
		resolution, resolveDiagnostics := resolveCBindingsVariant(options, pkg)
		result.Binding, result.Diagnostics = resolution, resolveDiagnostics
	}
	if len(result.Diagnostics) != 0 {
		result.Status = "rejected"
	}
	human := fmt.Sprintf("%s validation: %s (resolvable: %v)", result.Scope, result.Status, resolvable)
	emitCBindings(result, options.json, stdout, human)
	if result.Status != "complete" {
		return 2
	}
	return 0
}

// resolveCBindingsVariant reads the Variant from the project manifest next
// to the package root and resolves it against the project package.
// No compiler-distributed packages exist yet, so the project package (or
// the explicit --file) is the only candidate.
// Rules: MD-021 correction §§ 8.1-8.5.
func resolveCBindingsVariant(options cBindingsOptions, pkg *cbindings.Package) (*cbindings.Resolution, []cbindings.Diagnostic) {
	root := filepath.Dir(filepath.Dir(filepath.Dir(options.file)))
	manifest, err := os.ReadFile(filepath.Join(root, ".sec", "sec.toml"))
	if err != nil {
		return nil, []cbindings.Diagnostic{{ID: "c-bindings.no-project", Message: fmt.Sprintf("cannot read the project manifest: %v", err)}}
	}
	variant, errs := project.ReadVariantCBindings(string(manifest), options.variant)
	if len(errs) != 0 {
		diagnostics := make([]cbindings.Diagnostic, len(errs))
		for index, err := range errs {
			diagnostics[index] = cbindings.Diagnostic{ID: "c-bindings.invalid-variant", Message: err.Error()}
		}
		return nil, diagnostics
	}
	request := cbindings.Request{Variant: variant.Variant, OS: variant.OS, Arch: variant.Arch, ABI: variant.ABI,
		CRuntime: variant.CRuntime, CRuntimeVersion: variant.CRuntimeVersion}
	if variant.Selected {
		request.Selector = &cbindings.Selector{ID: variant.BindingsID, Version: variant.BindingsVersion, Environment: variant.BindingsEnvironment}
	}
	origin := "project"
	if options.file != cbindings.DefaultFile {
		origin = "explicit"
	}
	return cbindings.Resolve(request, []cbindings.Candidate{{Package: pkg, Origin: origin}})
}

// runCBindingsResolve prints the sec.c-bindings-resolution report.
// Rules: MD-021 correction § 8.9(3)-(4).
func runCBindingsResolve(options cBindingsOptions, stdout, stderr io.Writer) int {
	if options.variant == "" {
		fmt.Fprintln(stderr, "sec platform c-bindings resolve: --variant is required")
		return 1
	}
	report := cBindingsResolutionReport{Format: "sec.c-bindings-resolution", Version: 1, Status: "complete", Variant: options.variant, Target: map[string]string{}}
	_, pkg, diagnostics, err := readCBindingsFile(options.file)
	if err != nil {
		fmt.Fprintf(stderr, "sec platform c-bindings resolve: %v\n", err)
		return 1
	}
	if len(diagnostics) == 0 {
		report.Binding, diagnostics = resolveCBindingsVariant(options, pkg)
	}
	report.Diagnostics = diagnostics
	if report.Diagnostics == nil {
		report.Diagnostics = []cbindings.Diagnostic{}
	}
	if binding := report.Binding; binding != nil {
		report.Target = map[string]string{"os": binding.OS, "arch": binding.Arch, "abi": binding.ABI, "c_runtime": binding.CRuntime}
		if binding.CRuntimeVersion != "" {
			report.Target["c_runtime_version"] = binding.CRuntimeVersion
		}
	}
	if len(report.Diagnostics) != 0 {
		report.Status, report.Binding = "rejected", nil
	}
	if options.json {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(report)
	} else if report.Binding != nil {
		fmt.Fprintf(stdout, "%s: %s@%s environment %s, modules %s\n", options.variant, report.Binding.PackageID, report.Binding.PackageVersion,
			report.Binding.Environment, strings.Join(report.Binding.Modules, ", "))
	} else {
		for _, d := range report.Diagnostics {
			fmt.Fprintf(stdout, "%s: %s\n", d.ID, d.Message)
		}
	}
	if report.Status != "complete" {
		return 2
	}
	return 0
}

// cBindingsOperation builds the typed request of one mutating command and
// rejects flags that do not belong to it.
// Rules: MD-021 correction §§ 4(3)-(5); rules/tooling/cli.md — § 8.
func cBindingsOperation(operation, positional string, options cBindingsOptions) (cbindings.Operation, error) {
	allowed := map[string][]string{
		"package.set":        {"--id", "--version", "--source-root"},
		"environment.add":    {"--os", "--arch", "--abi", "--c-runtime", "--c-runtime-versions"},
		"environment.set":    {"--os", "--arch", "--abi", "--c-runtime", "--c-runtime-versions", "--clear-c-runtime-versions"},
		"environment.remove": {},
		"module.add":         {"--common", "--environment"},
		"module.remove":      {"--common", "--environment"},
	}[operation]
	for flag := range options.given {
		if flag == "--file" || flag == "--json" || flag == "--if-match" || indexString(allowed, flag) >= 0 {
			continue
		}
		return cbindings.Operation{}, fmt.Errorf("%s does not apply to this command", flag)
	}
	value := func(flag string, text string) *string {
		if !options.given[flag] {
			return nil
		}
		return &text
	}
	op := cbindings.Operation{Kind: cbindings.OperationKind(operation), Environment: positional,
		ID: value("--id", options.id), Version: value("--version", options.version), SourceRoot: value("--source-root", options.sourceRoot),
		OS: value("--os", options.os), Arch: value("--arch", options.arch), ABI: value("--abi", options.abi),
		CRuntime: value("--c-runtime", options.cRuntime), CRuntimeVersions: value("--c-runtime-versions", options.cRuntimeVersions),
		ClearCRuntimeVersions: options.clearCRuntimeVersions}
	switch operation {
	case "package.set":
		if op.ID == nil && op.Version == nil && op.SourceRoot == nil {
			return op, fmt.Errorf("give at least one of --id, --version, --source-root")
		}
	case "environment.add":
		for _, flag := range []string{"--os", "--arch", "--abi", "--c-runtime"} {
			if !options.given[flag] {
				return op, fmt.Errorf("%s is required", flag)
			}
		}
	case "environment.set":
		if op.CRuntimeVersions != nil && op.ClearCRuntimeVersions {
			return op, fmt.Errorf("--c-runtime-versions and --clear-c-runtime-versions exclude each other")
		}
		if len(options.given) == countGiven(options, "--file", "--json", "--if-match") {
			return op, fmt.Errorf("give at least one field to change")
		}
	case "module.add", "module.remove":
		if options.common == options.given["--environment"] {
			return op, fmt.Errorf("give exactly one of --common or --environment NAME")
		}
		op.Module, op.Common, op.Environment = positional, options.common, options.environment
	}
	return op, nil
}

func indexString(values []string, value string) int {
	for index, candidate := range values {
		if candidate == value {
			return index
		}
	}
	return -1
}

func countGiven(options cBindingsOptions, flags ...string) int {
	count := 0
	for _, flag := range flags {
		if options.given[flag] {
			count++
		}
	}
	return count
}

// runCBindingsMutation reads the file, checks --if-match, applies the typed
// edit and replaces the file atomically only after the edited text
// validated; any failure leaves the previous bytes in place.
// Rules: MD-021 correction §§ 5(2)-(6); rules/tooling/cli.md — § 8(3).
func runCBindingsMutation(op cbindings.Operation, options cBindingsOptions, stdout, stderr io.Writer) int {
	result := cBindingsResult{Format: "sec.c-bindings-cli", Version: 1, Operation: string(op.Kind), File: options.file}
	command := strings.ReplaceAll(string(op.Kind), ".", " ")
	reject := func(diagnostics ...cbindings.Diagnostic) int {
		result.Status, result.Changed, result.Diagnostics = "rejected", false, diagnostics
		result.AfterSHA = result.BeforeSHA
		emitCBindings(result, options.json, stdout, command+" rejected")
		return 2
	}
	data, err := os.ReadFile(options.file)
	if errors.Is(err, os.ErrNotExist) {
		return reject(cbindings.Diagnostic{ID: "c-bindings.no-file", Message: fmt.Sprintf("%s does not exist; create it with c-bindings init", options.file)})
	} else if err != nil {
		fmt.Fprintf(stderr, "sec platform c-bindings %s: %v\n", command, err)
		return 1
	}
	result.BeforeSHA = fileSHA256(data, true)
	if options.ifMatch != "" && !strings.EqualFold(options.ifMatch, *result.BeforeSHA) {
		return reject(cbindings.Diagnostic{ID: "c-bindings.stale-file", Message: fmt.Sprintf("%s has SHA-256 %s, not the expected %s; re-read it with show --json", options.file, *result.BeforeSHA, options.ifMatch)})
	}
	edit := cbindings.Apply(string(data), op)
	if edit.Rejected {
		return reject(edit.Diagnostics...)
	}
	if !edit.Changed {
		result.Status, result.Reason, result.AfterSHA = "complete", edit.Reason, result.BeforeSHA
		emitCBindings(result, options.json, stdout, fmt.Sprintf("%s: unchanged (%s)", command, edit.Reason))
		return 0
	}
	if err := replaceFileAtomically(options.file, []byte(edit.Text), result.BeforeSHA); errors.Is(err, errStaleFile) {
		return reject(cbindings.Diagnostic{ID: "c-bindings.stale-file", Message: fmt.Sprintf("%s changed while the edit was prepared; nothing was written", options.file)})
	} else if err != nil {
		fmt.Fprintf(stderr, "sec platform c-bindings %s: %v\n", command, err)
		return 1
	}
	result.Status, result.Changed, result.AfterSHA = "complete", true, fileSHA256([]byte(edit.Text), true)
	emitCBindings(result, options.json, stdout, fmt.Sprintf("%s: updated %s", command, options.file))
	return 0
}
