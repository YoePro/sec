package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"sec/internal/diagnostics"
)

func runCLI(args []string, out, errOut io.Writer) error {
	if len(args) > 0 {
		if command, ok := subcommands[args[0]]; ok {
			return command(args[1:], out, errOut)
		}
	}

	// Get the different arguments
	flags := flag.NewFlagSet("yamlstatus", flag.ContinueOnError)
	flags.SetOutput(errOut)
	dir := flags.String("dir", "governance", "Governance directory")
	file := flags.String("file", "", "Select a fragment by relative path or full path")
	id := flags.String("id", "", "Select an exact integration ID")
	random := flags.Bool("random", false, "Choose a random matching remaining point")
	point := flags.Int("point", 0, "Choose a remaining point by its 1-based index (requires -id)")
	exclude := flags.String("exclude", "", "Exclude points containing this text (case insensitive)")
	jsonOutput := flags.Bool("json", false, "Print selection, search, path or count-diff reports as JSON")
	grep := flags.String("grep", "", "Find all remaining points containing literal text (case insensitive; supports -file, -id and -json)")
	md := flags.String("md", "", "List exact MD-NNN references, or possible stale blockers with -md closed (supports -json)")
	corrections := flags.Bool("corrections", false, "Audit applied correction links, disconnected archive files and pending corrections (supports -json)")
	diagnosticAudit := flags.Bool("diagnostics", false, "Compare registered diagnostic IDs with governance/rules and suggest unused IDs by namespace (supports -json)")
	uses := flags.String("uses", "", "List code/tests/rules references to a repository path (supports -file, -id and -json)")
	paths := flags.Bool("paths", false, "Report missing code/tests/rules paths (supports -file, -id and -json)")
	goPaths := flags.Bool("go", false, "With -paths, check only .go paths under internal/ and cmd/")
	snapshotFile := flags.String("snapshot", "", "Save portable checklist counts to a new JSON baseline file")
	diffBaseline := flags.String("diff", "", "Compare checklist counts with a snapshot file or HEAD (supports -json)")
	verbose := flags.Bool("verbose", false, "Show all data otherwise just the most relevant.")
	html := flags.Bool("html", false, "Return the output as HTML.")
	itemOnly := flags.Bool("itemonly", false, "Only show the status for items")
	listFiles := flags.Bool("listfiles", false, "List YAML files with Implemented, Remaining and Partial counts and per-file percentages")
	flags.Usage = func() {
		fmt.Fprintln(errOut, "Usage: yamlstatus [options] [directory]\nWithout selection options, prints implementation statistics.\nWith -file or -id, selects the first matching remaining point; -random randomizes selection.\nWith -uses PATH, lists integrations referencing a file in code/tests/rules.\nWith -paths, checks listed paths for existence; -go restricts to internal/ and cmd/ Go files.")
		fmt.Fprintln(errOut, "With -snapshot FILE, saves portable counts; -diff FILE|HEAD compares counts by fragment and ID.")
		fmt.Fprintln(errOut, "With --listfiles, lists per-file checklist counts and percentages (supports -dir or a directory argument).")
		flags.PrintDefaults()
		fmt.Fprintln(errOut, "\n"+editUsage)
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if flags.NArg() > 1 {
		return fmt.Errorf("expected at most one directory argument")
	}
	if flags.NArg() == 1 {
		*dir = flags.Arg(0)
	}
	if *diagnosticAudit {
		root, err := auditQueryRoot(flags, "diagnostics", *dir)
		if err != nil {
			return err
		}
		if err := diagnostics.Validate(); err != nil {
			return err
		}
		report, err := auditDiagnosticRegistry(root, *dir, diagnostics.All())
		if err != nil {
			return err
		}
		return printDiagnosticAudit(out, report, *jsonOutput)
	}
	if *corrections {
		root, err := auditQueryRoot(flags, "corrections", *dir)
		if err != nil {
			return err
		}
		report, err := auditCorrections(root, *dir)
		if err != nil {
			return err
		}
		return printCorrectionAudit(out, report, *jsonOutput)
	}
	mdMode := false
	flags.Visit(func(option *flag.Flag) {
		if option.Name == "md" {
			mdMode = true
		}
	})
	if mdMode {
		root, err := auditQueryRoot(flags, "md", *dir)
		if err != nil {
			return err
		}
		report, err := findDecisionReferences(root, *dir, *md)
		if err != nil {
			return err
		}
		return printDecisionReferences(out, report, *jsonOutput)
	}
	if *listFiles {
		var incompatible string
		flags.Visit(func(option *flag.Flag) {
			if option.Name != "dir" && option.Name != "listfiles" && incompatible == "" {
				incompatible = option.Name
			}
		})
		if incompatible != "" {
			return fmt.Errorf("--listfiles cannot be combined with -%s; use only -dir or a directory argument", incompatible)
		}
		stats, err := collectStats(*dir)
		if err != nil {
			return err
		}
		return printFileChecklists(out, stats.FileChecklists)
	}
	snapshotMode, diffMode, invalidCountOption := false, false, false
	flags.Visit(func(option *flag.Flag) {
		switch option.Name {
		case "snapshot":
			snapshotMode = true
		case "diff":
			diffMode = true
		case "dir", "json":
		default:
			invalidCountOption = true
		}
	})
	if snapshotMode || diffMode {
		if snapshotMode == diffMode || snapshotMode && strings.TrimSpace(*snapshotFile) == "" || diffMode && strings.TrimSpace(*diffBaseline) == "" {
			return fmt.Errorf("specify exactly one non-empty -snapshot FILE or -diff BASELINE")
		}
		if invalidCountOption || snapshotMode && *jsonOutput {
			return fmt.Errorf("-snapshot/-diff cannot be combined with selection, search, paths or statistics presentation options; -json is supported only with -diff")
		}
		stats, err := collectStats(*dir)
		if err != nil {
			return err
		}
		current, err := snapshotCounts(*dir, stats)
		if err != nil {
			return err
		}
		if *snapshotFile != "" {
			if err := writeCountSnapshot(*snapshotFile, current); err != nil {
				return err
			}
			fmt.Fprintf(out, "Saved checklist counts for %d integrations\n", len(current.Entries))
			return nil
		}
		var baseline countSnapshot
		if *diffBaseline == "HEAD" {
			baseline, err = headCountSnapshot(*dir)
		} else {
			baseline, err = readCountSnapshot(*diffBaseline)
		}
		if err != nil {
			return err
		}
		return printCountDiff(out, compareCounts(baseline, current), *jsonOutput)
	}
	usesMode, grepMode := false, false
	flags.Visit(func(option *flag.Flag) {
		if option.Name == "uses" {
			usesMode = true
		}
		if option.Name == "grep" {
			grepMode = true
		}
	})
	if grepMode {
		if strings.TrimSpace(*grep) == "" {
			return fmt.Errorf("-grep requires non-empty text")
		}
		if usesMode || *paths || *goPaths || *random || *point != 0 || *exclude != "" || *html || *itemOnly || *verbose {
			return fmt.Errorf("-grep cannot be combined with -uses, -paths, -go, -random, -point, -exclude, -html, -itemonly or -verbose")
		}
	}
	if *goPaths && !*paths {
		return fmt.Errorf("-go requires -paths")
	}
	if *paths && usesMode {
		return fmt.Errorf("-paths and -uses cannot be combined")
	}
	if *paths && (*random || *point != 0 || *exclude != "" || *html || *itemOnly || *verbose) {
		return fmt.Errorf("-paths cannot be combined with -random, -point, -exclude, -html, -itemonly or -verbose")
	}
	if usesMode {
		if strings.TrimSpace(*uses) == "" {
			return fmt.Errorf("-uses requires a non-empty path")
		}
		if *random || *point != 0 || *exclude != "" || *html || *itemOnly || *verbose {
			return fmt.Errorf("-uses cannot be combined with -random, -point, -exclude, -html, -itemonly or -verbose")
		}
	}
	if *point < 0 || (*point > 0 && *id == "") {
		return fmt.Errorf("-point must be positive and requires -id")
	}
	if *point > 0 && *random {
		return fmt.Errorf("-point and -random cannot be combined")
	}
	selection := *random || *file != "" || *id != "" || *point > 0
	if !selection && !usesMode && !*paths && !grepMode && (*jsonOutput || *exclude != "") {
		return fmt.Errorf("-json and -exclude require -random, -file or -id")
	}
	input := *dir
	if *file != "" {
		input = *file
		if !filepath.IsAbs(input) {
			// Accept both core.yaml and governance/core.yaml from the repository root.
			rel, err := filepath.Rel(*dir, input)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				input = filepath.Join(*dir, input)
			}
		}
		ext := strings.ToLower(filepath.Ext(input))
		if ext != ".yaml" && ext != ".yml" {
			return fmt.Errorf("-file must name a YAML fragment")
		}
	}
	stats, err := collectStats(input)
	if err != nil {
		return err
	}
	if grepMode {
		report, err := searchRemaining(stats.Entries, *grep, *id)
		if err != nil {
			return err
		}
		return printRemainingSearch(out, report, *jsonOutput)
	}
	if *paths {
		root, err := referenceRoot(*dir)
		if err != nil {
			return err
		}
		report, err := checkReferencedPaths(stats.Entries, root, *id, *goPaths)
		if err != nil {
			return err
		}
		if err := printMissingPaths(out, report, *jsonOutput); err != nil {
			return err
		}
		if len(report.Missing) != 0 {
			return fmt.Errorf("%d missing path references", len(report.Missing))
		}
		return nil
	}
	if usesMode {
		root, err := referenceRoot(*dir)
		if err != nil {
			return err
		}
		report, err := findPathUses(stats.Entries, *uses, root, *id)
		if err != nil {
			return err
		}
		return printPathUses(out, report, *jsonOutput)
	}
	if !selection {
		if !*html {
			fmt.Fprintf(out, "Governance directory: %s\n", *dir)
		}
		printResults(out, stats, *verbose, *html, *itemOnly)
		return nil
	}
	selected, err := selectPoint(stats.Entries, *id, *point, *exclude, *random)
	if err != nil {
		return err
	}
	return printPoint(out, selected, *jsonOutput)
}
