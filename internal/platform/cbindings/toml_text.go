package cbindings

import "sec/internal/configedit"

// scanDocument uses the shared comment-preserving TOML editor; callers validate
// the model with the owning schema before and after every edit.
// Rules: rules/projects/projects.md §5(2),(4)-(6); MD-021 correction §5(3).
func scanDocument(text string) (*configedit.Document, error) { return configedit.Scan(text) }

// tomlString shares TOML escaping across the owning configuration services.
// Rules: rules/projects/projects.md §5(2).
func tomlString(value string) string { return configedit.String(value) }
