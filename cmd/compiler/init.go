package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sec/internal/project"
)

type initCommandOptions struct {
	ProjectDir     string
	ProjectName    string
	ProjectNameSet bool
	Target         CompilerTarget
	Profile        string
	Minimal        bool
	ModulePath     string
	DryRun         bool
	TargetKind     string
	TargetName     string
	TargetSet      bool
}

// runInitCommand selects project creation or additive module creation.
// Rules: rules/projects/projects.md — §§8–11,35–38,45,48(1–3).
func runInitCommand(args []string) {
	options, ok := parseInitCommandOptions(args, hostCompilerTarget())
	if !ok {
		printUsage()
		os.Exit(1)
	}
	if versions := validateDiscoveredProjectVersions([]string{options.ProjectDir}); versions.Errors > 0 {
		printDiagnosticSummary(versions)
		exitCLI(3)
	}

	if options.TargetKind != "" {
		if err := initTarget(options); err != nil {
			fmt.Fprintf(os.Stderr, "init error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if options.ModulePath != "" {
		if err := initModule(options); err != nil {
			fmt.Fprintf(os.Stderr, "init error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	_, existing, err := discoverInitManifest(options.ProjectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init error: %v\n", err)
		os.Exit(1)
	}
	if !existing && !options.DryRun && !options.ProjectNameSet && stdinIsTerminal() {
		name, err := promptString("Project name", filepath.Base(absPathOrFallback(options.ProjectDir)))
		if err != nil {
			fmt.Fprintf(os.Stderr, "init error: %v\n", err)
			os.Exit(1)
		}
		options.ProjectName = name
	}

	if options.ProjectName == "" {
		fmt.Fprintln(os.Stderr, "init error: project name is required")
		os.Exit(1)
	}

	if err := initProject(options); err != nil {
		fmt.Fprintf(os.Stderr, "init error: %v\n", err)
		os.Exit(1)
	}
}

// parseInitCommandOptions separates scaffold selection from the build profile.
// Rules: rules/projects/projects.md — §37(6–8),§38,§22 build profiles.
func parseInitCommandOptions(args []string, defaultTarget CompilerTarget) (initCommandOptions, bool) {
	options := initCommandOptions{
		ProjectDir: ".",
		Target:     defaultTarget,
		Profile:    "server",
	}

	filtered := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--dry-run" {
			if options.DryRun {
				return initCommandOptions{}, false
			}
			options.DryRun = true
			continue
		}
		filtered = append(filtered, arg)
		if (arg == "--name" || arg == "--target" || arg == "--profile") && i+1 < len(args) {
			i++
			filtered = append(filtered, args[i])
		}
	}

	args = filtered

	if len(args) > 0 && (args[0] == "command" || args[0] == "library" || args[0] == "firmware" || args[0] == "test") {
		if len(args) < 2 || !project.ValidProjectKey(args[1]) {
			return initCommandOptions{}, false
		}
		options.TargetKind, options.TargetName = args[0], args[1]
		for i := 2; i < len(args); i++ {
			if args[i] != "--target" || options.TargetSet || i+1 >= len(args) {
				return initCommandOptions{}, false
			}
			target, ok := parseCompilerTarget(args[i+1])
			if !ok {
				return initCommandOptions{}, false
			}
			options.Target = target
			options.TargetSet = true
			i++
		}
		return options, true
	}

	if len(args) > 0 && args[0] == "module" {
		if len(args) != 2 || args[1] == "" {
			return initCommandOptions{}, false
		}
		options.ModulePath = args[1]
		return options, true
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--minimal":
			if options.Minimal {
				return initCommandOptions{}, false
			}
			options.Minimal = true
		case "--name":
			if i+1 >= len(args) || options.ProjectName != "" {
				return initCommandOptions{}, false
			}
			options.ProjectName = args[i+1]
			options.ProjectNameSet = true
			i++
		case "--target":
			if i+1 >= len(args) {
				return initCommandOptions{}, false
			}
			target, ok := parseCompilerTarget(args[i+1])
			if !ok {
				return initCommandOptions{}, false
			}
			options.Target = target
			i++
		case "--profile":
			if i+1 >= len(args) || options.Profile != "server" {
				return initCommandOptions{}, false
			}
			options.Profile = strings.TrimSpace(args[i+1])
			i++
		default:
			if strings.HasPrefix(args[i], "-") || options.ProjectDir != "." {
				return initCommandOptions{}, false
			}
			options.ProjectDir = args[i]
		}
	}

	if options.ProjectName == "" {
		options.ProjectName = inferProjectName(options.ProjectDir)
	}
	if options.ProjectName == "" || options.Profile == "" || options.Target.OS == "" || options.Target.Arch == "" {
		return initCommandOptions{}, false
	}

	return options, true
}

// initProject discovers the governing project or plans a complete new scaffold
// before applying any writes. Existing projects are validated and preserved.
// Rules: rules/projects/projects.md — §§4,14(8),37,40,42–45.
func initProject(options initCommandOptions) error {
	projectDir := filepath.Clean(options.ProjectDir)
	existing, ok, err := discoverInitManifest(projectDir)
	if err != nil {
		return err
	}
	if ok {
		identity, err := readInitIdentity(existing)
		if err != nil {
			return err
		}
		if options.DryRun {
			fmt.Println("no changes")
		} else {
			fmt.Printf("Sec project %q already initialized in %s\n", identity.Name, filepath.Dir(filepath.Dir(existing)))
		}
		return nil
	}
	if err := rejectNestedInitManifest(projectDir); err != nil {
		return err
	}
	targetName := sanitizeOutputName(options.ProjectName)
	if targetName == "" {
		return fmt.Errorf("project name %q cannot be used as target name", options.ProjectName)
	}
	layout := projectInitLayout(projectDir, targetName, options.Minimal)
	uuid, err := newProjectUUID()
	if err != nil {
		return err
	}
	config, err := defaultSecConfig(options, uuid, targetName)
	if err != nil {
		return err
	}

	if _, manifestErrors := project.ParseManifest(filepath.Join(projectDir, ".sec", "sec.toml"), config); len(manifestErrors) > 0 {
		return fmt.Errorf("generated manifest is invalid: %v", manifestErrors[0])
	}

	sources, err := inspectInitModuleSources(filepath.Dir(layout.MainPath), "main")
	if err != nil {
		return err
	}
	expectedMain := 0
	if sources.hasMainSource {
		expectedMain = 1
	}
	if sources.mainFunctions != expectedMain {
		return fmt.Errorf("init source conflict: command module in %s would not have exactly one main function", filepath.Dir(layout.MainPath))
	}
	plan := &initPlan{}
	for _, dir := range layout.Directories {
		if err := plan.addDirectory(dir); err != nil {
			return err
		}
	}
	if err := plan.addFile(layout.MainPath, []byte(defaultMainSource()), true); err != nil {
		return err
	}
	// Publish the manifest last, after every source and directory write succeeds.
	if err := plan.addFile(filepath.Join(projectDir, ".sec", "sec.toml"), []byte(config), false); err != nil {
		return err
	}
	if err := executeInitPlan(plan, options.DryRun); err != nil {
		return err
	}
	if !options.DryRun {
		fmt.Printf("initialized Sec project %q in %s\n", options.ProjectName, projectDir)
	}
	return nil
}

func tomlBareKey(key string) string {
	if key == "" {
		return `""`
	}
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return strconv.Quote(key)
	}
	return key
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func findNearestProjectManifest(start string) (string, bool) {
	absStart, err := filepath.Abs(start)
	if err != nil {
		absStart = start
	}
	for current := filepath.Clean(absStart); ; current = filepath.Dir(current) {
		manifest := filepath.Join(current, ".sec", "sec.toml")
		if fileExists(manifest) {
			return manifest, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
	}
}

func samePath(left string, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr == nil {
		left = leftAbs
	}
	if rightErr == nil {
		right = rightAbs
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func inferProjectName(projectDir string) string {
	base := filepath.Base(absPathOrFallback(projectDir))
	if base == "." || base == string(filepath.Separator) {
		return ""
	}
	return base
}

func absPathOrFallback(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func sanitizeOutputName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// newProjectUUID generates a random RFC 9562 UUIDv4 for a new manifest only.
// Rules: rules/projects/projects.md — §3.3(4–6),§37(1).
func newProjectUUID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80

	encoded := make([]byte, 32)
	hex.Encode(encoded, bytes[:])
	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		encoded[0:8],
		encoded[8:12],
		encoded[12:16],
		encoded[16:20],
		encoded[20:32],
	), nil
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func promptString(label string, fallback string) (string, error) {
	if fallback != "" {
		fmt.Printf("%s [%s]: ", label, fallback)
	} else {
		fmt.Printf("%s: ", label)
	}

	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	return value, nil
}
