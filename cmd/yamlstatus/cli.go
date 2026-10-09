package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

func runCLI(args []string, out, errOut io.Writer) error {
	if len(args) > 0 {
		if command, ok := subcommands[args[0]]; ok {
			return command(args[1:], out, errOut)
		}
	}
	flags := flag.NewFlagSet("yamlstatus", flag.ContinueOnError)
	flags.SetOutput(errOut)
	dir := flags.String("dir", "governance", "Governance directory")
	file := flags.String("file", "", "Select a fragment by relative path or full path")
	id := flags.String("id", "", "Select an exact integration ID")
	random := flags.Bool("random", false, "Choose a random matching remaining point")
	point := flags.Int("point", 0, "Choose a remaining point by its 1-based index (requires -id)")
	exclude := flags.String("exclude", "", "Exclude points containing this text (case insensitive)")
	jsonOutput := flags.Bool("json", false, "Print selected point and integration context as JSON")
	flags.Usage = func() {
		fmt.Fprintln(errOut, "Usage: yamlstatus [options] [directory]\nWithout selection options, prints implementation statistics.\nWith -file or -id, selects the first matching remaining point; -random randomizes selection.")
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
	if *point < 0 || (*point > 0 && *id == "") {
		return fmt.Errorf("-point must be positive and requires -id")
	}
	if *point > 0 && *random {
		return fmt.Errorf("-point and -random cannot be combined")
	}
	selection := *random || *file != "" || *id != "" || *point > 0
	if !selection && (*jsonOutput || *exclude != "") {
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
	if !selection {
		fmt.Fprintf(out, "Governance directory: %s\n", *dir)
		printResults(out, stats)
		return nil
	}
	selected, err := selectPoint(stats.Entries, *id, *point, *exclude, *random)
	if err != nil {
		return err
	}
	return printPoint(out, selected, *jsonOutput)
}
