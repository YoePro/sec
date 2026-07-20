package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type initCommandOptions struct {
	ProjectDir     string
	ProjectName    string
	ProjectNameSet bool
	Target         CompilerTarget
	Profile        string
}

func runInitCommand(args []string) {
	options, ok := parseInitCommandOptions(args, hostCompilerTarget())
	if !ok {
		printUsage()
		os.Exit(1)
	}

	if !options.ProjectNameSet && stdinIsTerminal() {
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

func parseInitCommandOptions(args []string, defaultTarget CompilerTarget) (initCommandOptions, bool) {
	options := initCommandOptions{
		ProjectDir: ".",
		Target:     defaultTarget,
		Profile:    "server",
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 >= len(args) || options.ProjectName != "" {
				return initCommandOptions{}, false
			}
			options.ProjectName = strings.TrimSpace(args[i+1])
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

func initProject(options initCommandOptions) error {
	projectDir := filepath.Clean(options.ProjectDir)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return err
	}

	for _, dir := range []string{
		filepath.Join(projectDir, "cmd", "main"),
		filepath.Join(projectDir, "bin"),
		filepath.Join(projectDir, ".sec"),
		filepath.Join(projectDir, "internal"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	mainPath := filepath.Join(projectDir, "cmd", "main", "main.sec")
	if err := writeFileIfMissing(mainPath, []byte(defaultMainSource())); err != nil {
		return err
	}

	configPath := filepath.Join(projectDir, ".sec", "sec.toml")
	if fileExists(configPath) {
		return fmt.Errorf("%s already exists", configPath)
	}

	uuid, err := newProjectUUID()
	if err != nil {
		return err
	}

	config, err := defaultSecConfig(options, uuid)
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return err
	}

	fmt.Printf("initialized Sec project %q in %s\n", options.ProjectName, projectDir)
	return nil
}

func defaultMainSource() string {
	return `module main

fn main() int {
	return 0
}
`
}

func defaultSecConfig(options initCommandOptions, uuid string) (string, error) {
	definition, ok := findTargetDefinition(options.Target)
	if !ok || definition.LLVMTriple == "" {
		return "", fmt.Errorf("target %s has no LLVM triple", options.Target.String())
	}

	outputName := sanitizeOutputName(options.ProjectName)
	if outputName == "" {
		return "", fmt.Errorf("project name %q cannot be used as output name", options.ProjectName)
	}

	var b strings.Builder
	fmt.Fprintln(&b, "[project]")
	fmt.Fprintf(&b, "name = %q\n", options.ProjectName)
	fmt.Fprintf(&b, "uuid = %q\n", uuid)
	fmt.Fprintln(&b, "imports = []")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "[platform]")
	fmt.Fprintf(&b, "os = %q\n", options.Target.OS)
	fmt.Fprintf(&b, "arch = %q\n", options.Target.Arch)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "[build]")
	fmt.Fprintf(&b, "target = %q\n", definition.LLVMTriple)
	fmt.Fprintf(&b, "profile = %q\n", options.Profile)
	fmt.Fprintf(&b, "output = %q\n", filepath.ToSlash(filepath.Join("bin", outputName)))
	return b.String(), nil
}

func writeFileIfMissing(path string, data []byte) error {
	if fileExists(path) {
		return nil
	}
	return os.WriteFile(path, data, 0644)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
