#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEST="${1:-$HOME/.vscode-server/extensions/sec-lang.sec-syntax}"

echo "Building SEC language server..."
go build -o "$ROOT/bin/lsp-sec" "$ROOT/cmd/lsp"
mkdir -p "$ROOT/vscode/bin"
cp "$ROOT/bin/lsp-sec" "$ROOT/vscode/bin/lsp-sec"
chmod +x "$ROOT/vscode/bin/lsp-sec"

echo "Building VS Code extension..."
(cd "$ROOT/vscode" && npm install && npm run compile)

echo "Copying extension to $DEST"
rsync -a --delete \
  --exclude ".git" \
  "$ROOT/vscode/" "$DEST/"

echo "Done."
echo "Restart VS Code Server or reload the VS Code window if the extension was already loaded."
