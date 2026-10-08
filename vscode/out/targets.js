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
exports.registerTargetSelection = registerTargetSelection;
const vscode = __importStar(require("vscode"));
// Keep project target selection visible and switch using the server's canonical
// variant list. Rules: rules/tooling/lsp.md — Target-aware analysis, Target status.
function registerTargetSelection(context, getClient) {
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
        if (!uri || !client?.isRunning()) {
            status.hide();
            return;
        }
        try {
            const options = await client.sendRequest("sec/targets", { uri });
            if (generation !== refreshGeneration || activeURI() !== uri) {
                return;
            }
            const selected = options.find(option => option.active);
            status.text = selected ? `SEC: ${selected.logicalTarget}/${selected.variant}` : "SEC: automatic target";
            status.tooltip = selected ? `${selected.os}/${selected.arch}, ${selected.pointerWidthBits}-bit` : "Select the active SEC project target";
            status.show();
        }
        catch {
            if (generation === refreshGeneration) {
                status.hide();
            }
        }
    };
    context.subscriptions.push(status, vscode.commands.registerCommand("sec.selectTarget", async () => {
        const uri = activeURI();
        const client = getClient();
        if (!uri || !client?.isRunning()) {
            return;
        }
        try {
            const options = await client.sendRequest("sec/targets", { uri });
            const choices = [{ label: "Automatic target", description: "Use source directives and the project default", option: undefined },
                ...options.map(option => ({ label: `${option.logicalTarget}/${option.variant}`, description: `${option.os}/${option.arch}, ${option.pointerWidthBits}-bit${option.active ? " (active)" : ""}`, option }))];
            const choice = await vscode.window.showQuickPick(choices, { title: "SEC: Select project target", placeHolder: "Select a declared output variant" });
            if (!choice) {
                return;
            }
            await client.sendRequest("sec/selectTarget", { uri, logicalTarget: choice.option?.logicalTarget ?? "", variant: choice.option?.variant ?? "" });
            await refresh();
        }
        catch (error) {
            vscode.window.showErrorMessage(`SEC target selection failed: ${error instanceof Error ? error.message : String(error)}`);
        }
    }), vscode.window.onDidChangeActiveTextEditor(() => void refresh()), vscode.workspace.onDidSaveTextDocument(() => void refresh()));
    const initialClient = getClient();
    if (initialClient) {
        context.subscriptions.push(initialClient.onDidChangeState(() => void refresh()));
    }
    const manifestWatcher = vscode.workspace.createFileSystemWatcher("**/.sec/sec.toml");
    context.subscriptions.push(manifestWatcher, manifestWatcher.onDidChange(() => void refresh()), manifestWatcher.onDidCreate(() => void refresh()), manifestWatcher.onDidDelete(() => void refresh()));
    void refresh();
}
//# sourceMappingURL=targets.js.map