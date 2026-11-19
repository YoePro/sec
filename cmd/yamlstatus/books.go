package main

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
)

func runBooks(args []string, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("yamlstatus books", flag.ContinueOnError)
	flags.SetOutput(errOut)
	input := flags.String("indir", ".", "Source checkout root containing rules/README.md and language-rulebook-status.md")
	output := flags.String("outdir", "", "Static HTML destination outside the source checkout (required unless --check)")
	base := flags.String("base-url", "", "Optional deployment URL/path prefix; default uses portable relative links")
	check := flags.Bool("check", false, "Validate and render in memory without modifying the destination")
	flags.Usage = func() {
		fmt.Fprintln(errOut, "Usage: yamlstatus books --indir CHECKOUT --outdir DIRECTORY [--base-url /books/]\n       yamlstatus books --indir CHECKOUT --check\nGenerates a static index, category pages, linked rulebooks and permitted local images.\nUses a manifest to preserve unrelated files and safely replace generated files.\nFor cron, use absolute --indir and --outdir paths; no server or deployment is started.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("books accepts flags only; use --indir and --outdir")
	}
	if !*check && strings.TrimSpace(*output) == "" {
		return fmt.Errorf("books requires --outdir (or --check for read-only validation)")
	}
	if strings.TrimSpace(*input) == "" {
		return fmt.Errorf("books requires a nonempty --indir")
	}
	if *base != "" {
		u, err := url.Parse(*base)
		if err != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "" && u.Scheme != "http" && u.Scheme != "https") || u.Host != "" && u.Scheme == "" {
			return fmt.Errorf("invalid --base-url")
		}
		if u.Scheme == "" && !strings.HasPrefix(u.Path, "/") {
			return fmt.Errorf("--base-url must be an absolute HTTP(S) URL or root-relative path")
		}
		*base = strings.TrimRight(*base, "/") + "/"
	}
	if *output != "" {
		if _, err := bookDestination(*input, *output); err != nil {
			return err
		}
	}
	root, err := os.OpenRoot(*input)
	if err != nil {
		return err
	}
	defer root.Close()
	catalog, err := buildBookSite(&rulebookWorkspace{source: root}, *base)
	if err != nil {
		return err
	}
	validateBookArtifacts(catalog, *base)
	for _, diagnostic := range catalog.Diagnostics {
		location := diagnostic.File
		if diagnostic.Line > 0 {
			location = fmt.Sprintf("%s:%d", location, diagnostic.Line)
		}
		if _, err := fmt.Fprintf(errOut, "%s: %s\n", location, diagnostic.Message); err != nil {
			return err
		}
	}
	if len(catalog.Diagnostics) > 0 {
		return fmt.Errorf("rulebook validation failed: %d unresolved links/anchors; destination unchanged", len(catalog.Diagnostics))
	}
	if *check {
		_, err = fmt.Fprintf(out, "Validated %d rulebooks and %d static files; no output written.\n", len(catalog.Books), len(catalog.Artifacts))
		return err
	}
	if err := publishBookArtifacts(*input, *output, catalog.Artifacts); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Published %d rulebooks and %d static files. Open index.html in the destination.\n", len(catalog.Books), len(catalog.Artifacts))
	return err
}
