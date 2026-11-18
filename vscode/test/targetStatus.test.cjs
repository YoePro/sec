// Headless extension presentation regression: late/dirty/missing compiler
// snapshots cannot appear as successful clean target comparisons.
// Rules: rules/tooling/lsp.md — §§23.1–23.2, Document snapshots.
const assert = require("node:assert/strict");
const { test } = require("node:test");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const Module = require("node:module");
const { execFileSync } = require("node:child_process");

test("target view renders canonical results and rejects obsolete responses", async t => {
  const root = path.resolve(__dirname, "..");
  const output = fs.mkdtempSync(path.join(os.tmpdir(), "sec-target-view-"));
  t.after(() => fs.rmSync(output, { recursive: true, force: true }));
  execFileSync(path.join(root, "node_modules", ".bin", "tsc"), ["-p", root, "--outDir", output]);
  const disposable = () => ({ dispose() {} });
  const commands = new Map();
  const events = {};
  const source = { languageId: "sec", uri: { scheme: "file", toString: () => "file:///source.sec" }, version: 1 };
  let provider;
  const view = {};
  const vscode = {
    EventEmitter: class { event = disposable; fire() {} dispose() {} },
    TreeItem: class { constructor(label) { this.label = label; } },
    TreeItemCollapsibleState: { Collapsed: 1, None: 0 },
    ThemeIcon: class { constructor(id) { this.id = id; } },
    Range: class { constructor(...values) { this.values = values; } },
    commands: { registerCommand(name, callback) { commands.set(name, callback); return disposable(); } },
    window: {
      activeTextEditor: { document: source },
      createTreeView(id, options) { assert.equal(id, "sec.targetStatus"); provider = options.treeDataProvider; return view; },
      onDidChangeActiveTextEditor(callback) { events.editor = callback; return disposable(); }
    },
    languages: { onDidChangeDiagnostics: disposable },
    workspace: {
      onDidChangeTextDocument(callback) { events.change = callback; return disposable(); },
      onDidCloseTextDocument: disposable,
      onDidSaveTextDocument: disposable
    }
  };
  const originalLoad = Module._load;
  let registerTargetStatus;
  try {
    Module._load = function(name, ...args) { return name === "vscode" ? vscode : originalLoad.call(this, name, ...args); };
    ({ registerTargetStatus } = require(path.join(output, "targetStatus.js")));
  } finally { Module._load = originalLoad; }
  const pending = [];
  const client = {
    isRunning: () => true,
    onDidChangeState: disposable,
    sendRequest(method, params) {
      assert.equal(method, "sec/targetStatus");
      assert.equal(params.textDocument.uri, source.uri.toString());
      return new Promise(resolve => pending.push(resolve));
    }
  };
  registerTargetStatus({ subscriptions: [] }, () => client);
  const status = (version = source.version) => ({ uri: source.uri.toString(), version, state: "current", active: "app/small (linux-armv7)", options: [], targets: [
    { name: "app/small (linux-armv7)", active: true, current: true, option: { os: "linux", arch: "armv7", pointerWidthBits: 32 }, diagnostics: [{ severity: 1, code: "S1056", message: "overflow\nUse a wider type", range: { start: { line: 1, character: 2 }, end: { line: 1, character: 7 } } }] },
    { name: "app/large (linux-amd64)", active: false, current: true, diagnostics: [] }
  ] });
  const settle = () => new Promise(resolve => setImmediate(resolve));
  pending.shift()(status());
  await settle();
  let rows = provider.getChildren();
  assert.equal(rows[0].description, "1 error, 0 warnings · active");
  assert.equal(rows[0].tooltip, "linux/armv7, 32-bit");
  assert.equal(rows[1].description, "0 errors, 0 warnings");
  assert.equal(rows[0].children[0].label, "overflow");
  assert.equal(rows[0].children[0].tooltip, "overflow\nUse a wider type");
  assert.equal(rows[0].children[0].command.command, "vscode.open");
  assert.deepEqual(rows[0].children[0].command.arguments[1].selection.values, [1, 2, 1, 7]);

  source.version = 2;
  events.change({ document: source });
  assert.equal(provider.getChildren().length, 0, "dirty text must clear earlier results immediately");
  const oldRequest = pending.shift();
  const refresh = commands.get("sec.refreshTargetStatus")();
  const newRequest = pending.shift();
  newRequest({ ...status(), state: "unavailable" });
  await refresh;
  oldRequest(status(1));
  await settle();
  rows = provider.getChildren();
  assert.ok(rows.every(row => row.description === "comparison unavailable"));
  assert.ok(rows.every(row => row.children.length === 0));
  assert.match(view.message, /open\/save/);

  const inFlight = commands.get("sec.refreshTargetStatus")();
  const reply = pending.shift();
  vscode.window.activeTextEditor = undefined;
  events.editor();
  reply(status());
  await inFlight;
  assert.equal(provider.getChildren().length, 0, "response for a departed editor must stay hidden");
  assert.match(view.message, /Open a SEC source/);
});
