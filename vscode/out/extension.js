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
            fileEvents: vscode.workspace.createFileSystemWatcher("**/*.{sec,se}")
        }
    };
    client = new node_1.LanguageClient("secLanguageServer", "SEC Language Server", serverOptions, clientOptions);
    context.subscriptions.push(client);
    client.start().catch((error) => {
        const message = error instanceof Error ? error.message : String(error);
        output?.appendLine(`Failed to start SEC language server: ${message}`);
        vscode.window.showErrorMessage(`Failed to start SEC language server: ${message}`);
    });
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