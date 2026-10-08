"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
exports.activate = activate;
exports.deactivate = deactivate;
const path = __importStar(require("path"));
const vscode = __importStar(require("vscode"));
const node_1 = require("vscode-languageclient/node");
const targets_1 = require("./targets");
let client;
let output;
function activate(context) {
    output = vscode.window.createOutputChannel("SEC Language Server");
    context.subscriptions.push(output);
    const exe = process.platform === "win32" ? "lsp-sec.exe" : "lsp-sec";
    const serverPath = resolveLanguageServerPath(context, exe);
    output.appendLine(`Starting SEC language server: ${serverPath}`);
    const serverOptions = {
        command: serverPath,
        args: [],
        options: {}
    };
    const clientOptions = {
        documentSelector: [
            { scheme: "file", language: "sec" }
        ],
        synchronize: {
            fileEvents: [vscode.workspace.createFileSystemWatcher("**/*.{sec,se}"), vscode.workspace.createFileSystemWatcher("**/.sec/sec.toml")],
            configurationSection: "sec"
        },
        initializationOptions: {
            analysis: {
                parameters: {
                    hover: vscode.workspace.getConfiguration("sec.analysis.parameters").get("hover", "off"),
                    advisories: vscode.workspace.getConfiguration("sec.analysis.parameters").get("advisories", "off")
                }
            },
            inlayHints: {
                types: vscode.workspace.getConfiguration("sec.inlayHints").get("types", true),
                parameters: vscode.workspace.getConfiguration("sec.inlayHints").get("parameters", true),
                ownership: vscode.workspace.getConfiguration("sec.inlayHints").get("ownership", true)
            }
        }
    };
    client = new node_1.LanguageClient("secLanguageServer", "SEC Language Server", serverOptions, clientOptions);
    context.subscriptions.push(client);
    registerCompilerKnownDefinitions(context);
    registerShowLocations(context);
    (0, targets_1.registerTargetSelection)(context, () => client);
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
function registerCompilerKnownDefinitions(context) {
    const provider = {
        async provideTextDocumentContent(uri) {
            if (!client) {
                return "// The SEC language server is not running.\n";
            }
            const result = await client.sendRequest("sec/compilerKnownDefinition", { uri: uri.toString() });
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
function registerShowLocations(context) {
    const toPosition = (value) => new vscode.Position(value.line, value.character);
    context.subscriptions.push(vscode.commands.registerCommand("sec.showLocations", (uri, position, locations) => {
        const converted = (locations ?? []).map((item) => new vscode.Location(vscode.Uri.parse(item.uri), new vscode.Range(toPosition(item.range.start), toPosition(item.range.end))));
        return vscode.commands.executeCommand("editor.action.showReferences", vscode.Uri.parse(uri), toPosition(position), converted);
    }));
}
function deactivate() {
    return client?.stop();
}
function resolveLanguageServerPath(context, exe) {
    const configuredPath = vscode.workspace.getConfiguration("sec.languageServer").get("path");
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
function fileExists(file) {
    try {
        return require("fs").existsSync(file);
    }
    catch {
        return false;
    }
}
//# sourceMappingURL=extension.js.map