import * as path from "path";
import * as vscode from "vscode";
import { LanguageClient, LanguageClientOptions, ServerOptions } from "vscode-languageclient/node";

let client: LanguageClient | undefined;
let output: vscode.OutputChannel | undefined;

export function activate(context: vscode.ExtensionContext) {
  output = vscode.window.createOutputChannel("SEC Language Server");
  context.subscriptions.push(output);

  const exe = process.platform === "win32" ? "lsp-sec.exe" : "lsp-sec";
  const serverPath = resolveLanguageServerPath(context, exe);
  output.appendLine(`Starting SEC language server: ${serverPath}`);

  const serverOptions: ServerOptions = {
    command: serverPath,
    args: [],
    options: {}
  };

  const clientOptions: LanguageClientOptions = {
    documentSelector: [
      { scheme: "file", language: "sec" }
    ],
    synchronize: {
      fileEvents: vscode.workspace.createFileSystemWatcher("**/*.{sec,se}")
    }
  };

  client = new LanguageClient(
    "secLanguageServer",
    "SEC Language Server",
    serverOptions,
    clientOptions
  );

  context.subscriptions.push(client);
  client.start().catch((error) => {
    const message = error instanceof Error ? error.message : String(error);
    output?.appendLine(`Failed to start SEC language server: ${message}`);
    vscode.window.showErrorMessage(`Failed to start SEC language server: ${message}`);
  });
}

export function deactivate(): Thenable<void> | undefined {
  return client?.stop();
}

function resolveLanguageServerPath(context: vscode.ExtensionContext, exe: string): string {
  const configuredPath = vscode.workspace.getConfiguration("sec.languageServer").get<string>("path");
  if (configuredPath && configuredPath.trim() !== "") {
    return configuredPath;
  }

  const candidates = [
    context.asAbsolutePath(path.join("bin", exe)),
    context.asAbsolutePath(path.join("..", "bin", exe))
  ];

  for (const candidate of candidates) {
    if (fileExists(candidate)) {
      return candidate;
    }
  }

  return exe;
}

function fileExists(file: string): boolean {
  try {
    return require("fs").existsSync(file);
  } catch {
    return false;
  }
}
