import * as vscode from "vscode";
import { LanguageClient } from "vscode-languageclient/node";

interface TargetDiagnostic {
  range: { start: { line: number; character: number }; end: { line: number; character: number } };
  severity?: number;
  code?: string;
  message: string;
}

interface TargetRow {
  name: string;
  active: boolean;
  current: boolean;
  diagnostics: TargetDiagnostic[] | null;
  option?: { os: string; arch: string; pointerWidthBits: number };
}

interface TargetStatus {
  uri: string;
  version: number;
  state: "current" | "unavailable";
  active: string;
  options: { logicalTarget: string; variant: string; os: string; arch: string; pointerWidthBits: number }[];
  targets: TargetRow[];
}

class StatusItem extends vscode.TreeItem {
  constructor(label: string, public readonly children: StatusItem[] = []) {
    super(label, children.length ? vscode.TreeItemCollapsibleState.Collapsed : vscode.TreeItemCollapsibleState.None);
  }
}

// Present compiler-owned snapshots; no editor-side target resolver or analysis.
// Rules: rules/tooling/lsp.md — §§23.1–23.2, Document snapshots.
export function registerTargetStatus(context: vscode.ExtensionContext, getClient: () => LanguageClient | undefined) {
  const changed = new vscode.EventEmitter<void>();
  let roots: StatusItem[] = [];
  let generation = 0;
  const provider: vscode.TreeDataProvider<StatusItem> = {
    onDidChangeTreeData: changed.event,
    getTreeItem: item => item,
    getChildren: item => item ? item.children : roots
  };
  const view = vscode.window.createTreeView("sec.targetStatus", { treeDataProvider: provider, showCollapseAll: true });
  const document = () => {
    const candidate = vscode.window.activeTextEditor?.document;
    return candidate?.languageId === "sec" && candidate.uri.scheme === "file" ? candidate : undefined;
  };
  const refresh = async () => {
    const request = ++generation;
    const source = document();
    const client = getClient();
    roots = [];
    view.description = undefined;
    view.message = source ? "Target comparison unavailable for the current text. Results refresh on open/save." : "Open a SEC source file to compare targets.";
    changed.fire();
    if (!source || !client?.isRunning()) { return; }
    const uri = source.uri.toString();
    const version = source.version;
    try {
      const status = await client.sendRequest<TargetStatus>("sec/targetStatus", { textDocument: { uri } });
      if (request !== generation || document()?.uri.toString() !== uri || source.version !== version) { return; }
      const current = status.state === "current" && status.uri === uri && status.version === version;
      view.description = `v${version}`;
      view.message = current ? `Active analysis target: ${status.active}. Counts describe this document's current comparison snapshot.` : "No current comparison. Available comparisons refresh on open/save; unsaved changes are not compared automatically.";
      roots = status.targets.map(row => {
        const ready = current && row.current && row.diagnostics !== null;
        const diagnostics = ready ? row.diagnostics! : [];
        const children = diagnostics.map(diagnostic => {
          const item = new StatusItem(diagnostic.message.split(/\r?\n/, 1)[0]);
          item.description = diagnostic.code;
          item.tooltip = diagnostic.message;
          item.iconPath = new vscode.ThemeIcon(diagnostic.severity === 1 ? "error" : diagnostic.severity === 2 ? "warning" : "info");
          const range = new vscode.Range(diagnostic.range.start.line, diagnostic.range.start.character, diagnostic.range.end.line, diagnostic.range.end.character);
          item.command = { command: "vscode.open", title: "Open diagnostic", arguments: [source.uri, { selection: range }] };
          return item;
        });
        const item = new StatusItem(row.name, children);
        item.id = `target:${uri}:${row.name}`;
        const errors = diagnostics.filter(d => d.severity === 1).length;
        const warnings = diagnostics.filter(d => d.severity === 2).length;
        item.description = ready ? `${errors} ${errors === 1 ? "error" : "errors"}, ${warnings} ${warnings === 1 ? "warning" : "warnings"}${row.active ? " · active" : ""}` : "comparison unavailable";
        const option = row.option;
        item.tooltip = option ? `${option.os}/${option.arch}, ${option.pointerWidthBits}-bit` : row.name;
        item.iconPath = new vscode.ThemeIcon(row.active && current ? "target" : "list-tree");
        return item;
      });
      changed.fire();
    } catch (error) {
      if (request !== generation) { return; }
      view.message = `Target status unavailable: ${error instanceof Error ? error.message : String(error)}`;
    }
  };
  context.subscriptions.push(changed, view,
    vscode.commands.registerCommand("sec.refreshTargetStatus", refresh),
    vscode.window.onDidChangeActiveTextEditor(() => void refresh()),
    vscode.languages.onDidChangeDiagnostics(() => void refresh()),
    vscode.workspace.onDidChangeTextDocument(event => { if (event.document === document()) { void refresh(); } }),
    vscode.workspace.onDidCloseTextDocument(() => void refresh()),
    vscode.workspace.onDidSaveTextDocument(() => void refresh()));
  const client = getClient();
  if (client) { context.subscriptions.push(client.onDidChangeState(() => void refresh())); }
  void refresh();
}
