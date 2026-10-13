package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sec/internal/project"
)

type buildVariant struct {
	options buildCommandOptions
	name    string
}
type buildSelection struct {
	jobs             []buildVariant
	manifest, target string
	snapshot         string
}

// selectBuildJobs preserves standalone file builds and selects declared Target
// Variants for `sec build <TargetName>`. A platform override filters the same
// logical build, rather than reserving a separate number per Variant.
// Rules: rules/projects/projects.md — §§17,19,21,26,29(3–5),30(1–2),36.
func selectBuildJobs(options buildCommandOptions) (buildSelection, error) {
	if info, err := os.Stat(options.InputFile); err == nil && !info.IsDir() {
		if err := validateBuildCounterManifest(options.InputFile); err != nil {
			return buildSelection{}, err
		}
		selection := buildSelection{jobs: []buildVariant{{options, options.Target.String()}}}
		path, found := findNearestProjectManifest(filepath.Dir(options.InputFile))
		if !found {
			if options.BuildNumber != nil {
				return buildSelection{}, fmt.Errorf("--build-number requires a declared Project Target")
			}
			return selection, nil
		}
		manifest, err := project.LoadManifest(path)
		if err != nil {
			if options.BuildNumber != nil {
				return buildSelection{}, err
			}
			return selection, nil // legacy manifest diagnostics remain separate work
		}
		absolute, _ := filepath.Abs(options.InputFile)
		if target, found := project.TargetForSource(manifest.Targets(), manifest.Facts().ProjectRoot, absolute); found {
			selection.manifest, selection.target = manifest.Path(), target.Name
			selection.snapshot = manifest.Text()
		} else if options.BuildNumber != nil {
			return buildSelection{}, fmt.Errorf("--build-number requires a declared Target owning %s", options.InputFile)
		}
		return selection, nil
	}
	if !project.ValidProjectKey(options.InputFile) {
		return buildSelection{}, fmt.Errorf("input must be a source file or a declared Target name")
	}
	path, found := findNearestProjectManifest(".")
	if !found {
		return buildSelection{}, fmt.Errorf("no Project manifest for Target %q", options.InputFile)
	}
	manifest, err := project.LoadManifest(path)
	if err != nil {
		return buildSelection{}, err
	}
	selection := buildSelection{manifest: manifest.Path(), target: options.InputFile, snapshot: manifest.Text()}
	var target project.Target
	for _, candidate := range manifest.Targets() {
		if candidate.Name == selection.target {
			target = candidate
			break
		}
	}
	if target.Source == "" {
		return buildSelection{}, fmt.Errorf("unknown Target or missing source: %s", selection.target)
	}
	root := manifest.Facts().ProjectRoot
	if filepath.IsAbs(target.Source) {
		return buildSelection{}, fmt.Errorf("Target source must be relative to the Project")
	}
	directory := filepath.Join(root, filepath.FromSlash(target.Source))
	relative, err := filepath.Rel(root, directory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return buildSelection{}, fmt.Errorf("Target source must remain within the Project")
	}
	input := filepath.Join(directory, "main.sec")
	if info, err := os.Stat(input); err != nil || info.IsDir() {
		files, _ := filepath.Glob(filepath.Join(directory, "*.sec"))
		input = ""
		for _, file := range files {
			if info, err := os.Stat(file); err == nil && !info.IsDir() && !strings.HasSuffix(file, "_test.sec") {
				input = file
				break
			}
		}
		if input == "" {
			return buildSelection{}, fmt.Errorf("Target %s has no production source", selection.target)
		}
	}
	doc, _ := project.ParseManifestDocument(manifest.Text())
	table, err := doc.Table("target", selection.target)
	if err != nil {
		return buildSelection{}, err
	}
	variants, ok := table["variants"].([]any)
	if !ok || len(variants) == 0 {
		return buildSelection{}, fmt.Errorf("Target %s requires configured Variants", selection.target)
	}
	artifact := selection.target
	if value, found := table["artifact"]; found {
		var ok bool
		artifact, ok = value.(string)
		if !ok || artifact == "" || filepath.Base(artifact) != artifact || strings.ContainsAny(artifact, `/\`) || artifact == "." || artifact == ".." {
			return buildSelection{}, fmt.Errorf("Target artifact must be a filename")
		}
	}
	build, _ := doc.Table("build")
	profile, _ := build["profile"].(string)
	seen := map[string]bool{}
	for _, value := range variants {
		name, ok := value.(string)
		if !ok || !project.ValidProjectKey(name) || seen[name] {
			return buildSelection{}, fmt.Errorf("invalid or duplicate Variant %v", value)
		}
		seen[name] = true
		variant, err := doc.Table("variant", name)
		if err != nil || variant == nil {
			return buildSelection{}, fmt.Errorf("undefined Variant %s", name)
		}
		osName, _ := variant["os"].(string)
		arch, _ := variant["arch"].(string)
		platform, ok := parseCompilerTarget(osName + "-" + arch)
		if !ok {
			return buildSelection{}, fmt.Errorf("invalid platform for Variant %s", name)
		}
		if options.TargetSet && platform != options.Target {
			continue
		}
		job := options
		job.InputFile, job.Target = input, platform
		if !options.OutputSet {
			if !project.ValidProjectKey(profile) {
				return buildSelection{}, fmt.Errorf("named Target output requires a configured [build].profile or explicit -o")
			}
			filename := artifact
			if platform.OS == "windows" && !strings.HasSuffix(filename, ".exe") {
				filename += ".exe"
			}
			job.OutputFile = filepath.Join(root, "bin", selection.target, name, profile, filename)
			if job.LLVMOutputFile != "" {
				job.LLVMOutputFile = job.OutputFile + ".ll"
			}
			if job.MLIROutputFile != "" {
				job.MLIROutputFile = job.OutputFile + ".mlir"
			}
		}
		selection.jobs = append(selection.jobs, buildVariant{job, name})
	}
	if len(selection.jobs) == 0 {
		return buildSelection{}, fmt.Errorf("no configured Variant matches the requested platform")
	}
	if options.OutputSet && len(selection.jobs) > 1 {
		return buildSelection{}, fmt.Errorf("-o requires selection of one Variant to avoid overwriting outputs")
	}
	return selection, nil
}
