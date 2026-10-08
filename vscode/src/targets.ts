import * as vscode from "vscode";
import { LanguageClient } from "vscode-languageclient/node";

interface TargetOption {
  logicalTarget: string;
  variant: string;
  os: string;
  arch: string;
  pointerWidthBits: number;
  active: boolean;
}

// Keep project target selection visible and switch using the server's canonical
// variant list. Rules: rules/tooling/lsp.md — Target-aware analysis, Target status.
export function registerTargetSelection(context: vscode.ExtensionContext, getClient: () => LanguageClient | undefined) {
  const status = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 20);
  status.command = "sec.selectTarget";
  let refreshGeneration = 0;
  const activeURI = () => {
    const document = vscode.window.activeTextEditor?.document;
    return document?.languageId === "sec" && document.uri.scheme === "file" ? document.uri.toString() : undefined;
  };
  const refresh = async () => {
    const generation = ++refreshGeneration;
    const uri = activeURI();
    const client = getClient();
    if (!uri || !client?.isRunning()) { status.hide(); return; }
    try {
      const options = await client.sendRequest<TargetOption[]>("sec/targets", { uri });
      if (generation !== refreshGeneration || activeURI() !== uri) { return; }
      const selected = options.find(option => option.active);
      status.text = selected ? `SEC: ${selected.logicalTarget}/${selected.variant}` : "SEC: automatic target";
      status.tooltip = selected ? `${selected.os}/${selected.arch}, ${selected.pointerWidthBits}-bit` : "Select the active SEC project target";
      status.show();
    } catch { if (generation === refreshGeneration) { status.hide(); } }
  };
  context.subscriptions.push(status, vscode.commands.registerCommand("sec.selectTarget", async () => {
    const uri = activeURI();
    const client = getClient();
    if (!uri || !client?.isRunning()) { return; }
    try {
      const options = await client.sendRequest<TargetOption[]>("sec/targets", { uri });
      const choices = [{ label: "Automatic target", description: "Use source directives and the project default", option: undefined as TargetOption | undefined },
        ...options.map(option => ({ label: `${option.logicalTarget}/${option.variant}`, description: `${option.os}/${option.arch}, ${option.pointerWidthBits}-bit${option.active ? " (active)" : ""}`, option }))];
      const choice = await vscode.window.showQuickPick(choices, { title: "SEC: Select project target", placeHolder: "Select a declared output variant" });
      if (!choice) { return; }
      await client.sendRequest("sec/selectTarget", { uri, logicalTarget: choice.option?.logicalTarget ?? "", variant: choice.option?.variant ?? "" });
      await refresh();
    } catch (error) { vscode.window.showErrorMessage(`SEC target selection failed: ${error instanceof Error ? error.message : String(error)}`); }
  }), vscode.window.onDidChangeActiveTextEditor(() => void refresh()), vscode.workspace.onDidSaveTextDocument(() => void refresh()));
  const initialClient = getClient();
  if (initialClient) { context.subscriptions.push(initialClient.onDidChangeState(() => void refresh())); }
  const manifestWatcher = vscode.workspace.createFileSystemWatcher("**/.sec/sec.toml");
  context.subscriptions.push(manifestWatcher, manifestWatcher.onDidChange(() => void refresh()),
    manifestWatcher.onDidCreate(() => void refresh()), manifestWatcher.onDidDelete(() => void refresh()));
  void refresh();
}
