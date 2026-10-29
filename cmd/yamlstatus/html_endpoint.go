package main

import (
	_ "embed"
	"fmt"
	"html/template"
	"strings"
	"time"
)

//go:embed html_endpoint.html
var htmlPageSource string

var htmlPage = template.Must(template.New("implementation-status").Parse(htmlPageSource))

// htmlNow supplies the report's update time; tests replace it.
var htmlNow = time.Now

type htmlCount struct {
	Label      string
	Count      int
	Percentage string
}

type htmlReport struct {
	Stats          Stats
	Statuses       []htmlCount
	Checklist      []htmlCount
	Fields         []htmlCount
	ChecklistTotal int
	Completion     string
	Updated        string
	UpdatedISO     string
}

// createHTMLOutput renders a complete document using the Sec manual's shared
// layout and stylesheet. html/template escapes all governance-derived text.
func createHTMLOutput(stats Stats, verbose bool) string {
	updated := htmlNow().UTC()
	report := htmlReport{
		Stats: stats, ChecklistTotal: stats.TotalImplemented + stats.TotalRemaining + stats.TotalPartial,
		Updated: updated.Format("2006-01-02 15:04 UTC"), UpdatedISO: updated.Format(time.RFC3339),
	}
	for _, status := range []string{"implemented", "partial", "planned"} {
		report.Statuses = append(report.Statuses, htmlCount{Label: status, Count: stats.StatusCounts[status]})
	}
	for _, status := range sortedKeys(stats.StatusCounts) {
		if status != "implemented" && status != "partial" && status != "planned" {
			report.Statuses = append(report.Statuses, htmlCount{Label: status, Count: stats.StatusCounts[status]})
		}
	}
	for _, item := range []htmlCount{
		{Label: "Implemented", Count: stats.TotalImplemented},
		{Label: "Remaining", Count: stats.TotalRemaining},
		{Label: "Partial", Count: stats.TotalPartial},
	} {
		if report.ChecklistTotal > 0 {
			item.Percentage = fmt.Sprintf("%.1f%%", float64(item.Count)/float64(report.ChecklistTotal)*100)
		}
		report.Checklist = append(report.Checklist, item)
	}
	report.Completion = report.Checklist[0].Percentage

	if verbose {
		for _, field := range sortedKeys(stats.FieldCounts) {
			report.Fields = append(report.Fields, htmlCount{Label: field, Count: stats.FieldCounts[field]})
		}
	}

	var output strings.Builder
	if err := htmlPage.Execute(&output, report); err != nil {
		// The embedded template and its statically typed model are a code
		// invariant; strings.Builder cannot fail on a write.
		panic(fmt.Errorf("render implementation status: %w", err))
	}
	return output.String()
}
