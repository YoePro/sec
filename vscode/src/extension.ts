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
      fileEvents: vscode.workspace.createFileSystemWatcher("**/*.{sec,se}"),
      configurationSection: "sec"
    },
    initializationOptions: {
      analysis: {
        parameters: {
          hover: vscode.workspace.getConfiguration("sec.analysis.parameters").get<string>("hover", "off"),
          advisories: vscode.workspace.getConfiguration("sec.analysis.parameters").get<string>("advisories", "off")
        }
      },
      inlayHints: {
        types: vscode.workspace.getConfiguration("sec.inlayHints").get<boolean>("types", true),
        parameters: vscode.workspace.getConfiguration("sec.inlayHints").get<boolean>("parameters", true),
        ownership: vscode.workspace.getConfiguration("sec.inlayHints").get<boolean>("ownership", true)
      }
    }
  };

  client = new LanguageClient(
    "secLanguageServer",
    "SEC Language Server",
    serverOptions,
    clientOptions
  );

  context.subscriptions.push(client);
  registerCompilerKnownDefinitions(context);
  registerShowLocations(context);
  client.start().catch((error) => {
    const message = error instanceof Error ? error.message : String(error);
    output?.appendLine(`Failed to start SEC language server: ${message}`);
    vscode.window.showErrorMessage(`Failed to start SEC language server: ${message}`);
  });
}

// Synthetic read-only definitions of compiler-known members
// (rules/compiler/compiler_known_members.md "Synthetic definitions"). The
// language server renders them from its registry; a content provider keeps
// them read-only and outside the workspace.
const compilerKnownScheme = "sec-compiler-known";

function registerCompilerKnownDefinitions(context: vscode.ExtensionContext) {
  const provider: vscode.TextDocumentContentProvider = {
    async provideTextDocumentContent(uri: vscode.Uri): Promise<string> {
      if (!client) {
        return "// The SEC language server is not running.\n";
      }
      const result = await client.sendRequest<{ text: string }>("sec/compilerKnownDefinition", { uri: uri.toString() });
      return result.text;
    }
  };
  context.subscriptions.push(vscode.workspace.registerTextDocumentContentProvider(compilerKnownScheme, provider));
  context.subscriptions.push(vscode.workspace.onDidOpenTextDocument((document) => {
    if (document.uri.scheme === compilerKnownScheme && document.languageId !== "sec") {
      void vscode.languages.setTextDocumentLanguage(document, "sec");
    }
  }));
}

// sec.showLocations lists locations sent by the language server, such as the
// implementations or failing members behind an interface conformance lens.
function registerShowLocations(context: vscode.ExtensionContext) {
  type LspPosition = { line: number; character: number };
  type LspLocation = { uri: string; range: { start: LspPosition; end: LspPosition } };
  const toPosition = (value: LspPosition) => new vscode.Position(value.line, value.character);
  context.subscriptions.push(vscode.commands.registerCommand("sec.showLocations", (uri: string, position: LspPosition, locations: LspLocation[]) => {
    const converted = (locations ?? []).map((item) => new vscode.Location(vscode.Uri.parse(item.uri), new vscode.Range(toPosition(item.range.start), toPosition(item.range.end))));
    return vscode.commands.executeCommand("editor.action.showReferences", vscode.Uri.parse(uri), toPosition(position), converted);
  }));
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
