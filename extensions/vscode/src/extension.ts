import * as fs from 'node:fs/promises';
import * as path from 'node:path';
import { randomUUID } from 'node:crypto';
import * as vscode from 'vscode';
import { runCLI } from './services/cli';
import { findingPath } from './services/findings';

interface Finding {
  id: string;
  file: string;
  startLine: number;
  endLine: number;
  severity: string;
  title: string;
  explanation: string;
  citedSnippet?: string;
  suggestedFix?: string;
}
interface Result { status: string; findings: Finding[]; coverage: { path: string; state: string; reason?: string }[] }
interface Summary { id: string }
interface Session { triage?: Record<string, string> }

export function activate(context: vscode.ExtensionContext): void {
  const diagnostics = vscode.languages.createDiagnosticCollection('open-review');
  const log = vscode.window.createOutputChannel('open-review');
  context.subscriptions.push(diagnostics, log);
  let current: { root: string; dir: string; id: string; findings: Finding[]; triage: Record<string, string> } | undefined;

  const binary = () => vscode.workspace.getConfiguration('openReview').get<string>('cliPath', 'open-review');
  const workspace = async () => {
    const folders = vscode.workspace.workspaceFolders;
    if (!folders?.length) throw new Error('Open a repository folder first');
    return folders.length === 1 ? folders[0] : await vscode.window.showWorkspaceFolderPick();
  };
  const call = (root: string, args: string[], signal?: AbortSignal) => runCLI(binary(), args, root, signal);

  const redraw = () => {
    diagnostics.clear();
    if (!current) return;
    const byFile = new Map<string, vscode.Diagnostic[]>();
    for (const finding of current.findings) {
      if (current.triage[finding.id] && current.triage[finding.id] !== 'open') continue;
      const file = findingPath(current.root, finding.file);
      if (!file) continue;
      const range = new vscode.Range(Math.max(0, finding.startLine - 1), 0, Math.max(0, finding.endLine - 1), Number.MAX_SAFE_INTEGER);
      const severity = finding.severity === 'severe' ? vscode.DiagnosticSeverity.Error : finding.severity === 'non-severe' ? vscode.DiagnosticSeverity.Warning : vscode.DiagnosticSeverity.Information;
      const diagnostic = new vscode.Diagnostic(range, `${finding.title}\n${finding.explanation}`, severity);
      diagnostic.source = 'open-review';
      diagnostic.code = finding.id;
      byFile.set(file, [...(byFile.get(file) ?? []), diagnostic]);
    }
    for (const [file, items] of byFile) diagnostics.set(vscode.Uri.file(file), items);
  };

  const refresh = async () => {
    if (!current) return;
    const response = await call(current.root, ['sessions', 'show', '--dir', current.dir, '--repo', current.root, current.id]);
    current.triage = (JSON.parse(response.stdout) as Session).triage ?? {};
    redraw();
  };

  context.subscriptions.push(vscode.commands.registerCommand('openReview.preview', async () => {
    try {
      const folder = await workspace();
      if (!folder) return;
      const response = await call(folder.uri.fsPath, ['review', '--repo', folder.uri.fsPath, '--preview', '--output', 'json']);
      const preview = JSON.parse(response.stdout) as Result;
      const items = preview.coverage.map((item) => ({ label: `${item.state}: ${item.path}`, description: item.reason ?? '' }));
      await vscode.window.showQuickPick(items, { title: 'open-review scope', placeHolder: 'Selected and skipped files' });
    } catch (error) { void vscode.window.showErrorMessage(String(error)); }
  }));

  context.subscriptions.push(vscode.commands.registerCommand('openReview.review', async () => {
    try {
      const folder = await workspace();
      if (!folder) return;
      const root = folder.uri.fsPath;
      const dir = path.join(context.globalStorageUri.fsPath, randomUUID());
      await fs.mkdir(dir, { recursive: true, mode: 0o700 });
      const response = await vscode.window.withProgress({ location: vscode.ProgressLocation.Notification, title: 'open-review: Reviewing workspace', cancellable: true }, async (_, token) => {
        const controller = new AbortController();
        token.onCancellationRequested(() => controller.abort());
        return call(root, ['review', '--repo', root, '--output', 'json', '--quiet', '--session-dir', dir], controller.signal);
      });
      const result = JSON.parse(response.stdout) as Result;
      const sessions = JSON.parse((await call(root, ['sessions', 'list', '--dir', dir, '--repo', root])).stdout) as Summary[];
      if (sessions.length !== 1) throw new Error('Review session was not saved');
      current = { root, dir, id: sessions[0].id, findings: result.findings ?? [], triage: {} };
      await refresh();
      log.appendLine(`${result.status}: ${current.findings.length} findings`);
      for (const item of result.coverage ?? []) if (item.state !== 'reviewed') log.appendLine(`${item.state}: ${item.path} (${item.reason ?? 'no reason'})`);
      log.show(true);
    } catch (error) { void vscode.window.showErrorMessage(String(error)); }
  }));

  context.subscriptions.push(vscode.commands.registerCommand('openReview.refreshTriage', async () => {
    try { await refresh(); } catch (error) { void vscode.window.showErrorMessage(String(error)); }
  }));
  context.subscriptions.push(vscode.commands.registerCommand('openReview.providerSetup', () => {
    vscode.window.createTerminal({ name: 'open-review provider setup', shellPath: binary(), shellArgs: ['config', 'setup'] }).show();
  }));
  context.subscriptions.push(vscode.commands.registerCommand('openReview.markFinding', async (id: string, state: string) => {
    if (!current) return;
    try {
      await call(current.root, ['sessions', 'triage', '--dir', current.dir, '--repo', current.root, current.id, id, state]);
      await refresh();
    } catch (error) { void vscode.window.showErrorMessage(String(error)); }
  }));
  context.subscriptions.push(vscode.commands.registerCommand('openReview.applyFinding', async (finding: Finding, uri: vscode.Uri) => {
    if (!current || !finding.citedSnippet || !finding.suggestedFix) return;
    const document = await vscode.workspace.openTextDocument(uri);
    const range = new vscode.Range(Math.max(0, finding.startLine - 1), 0, Math.max(0, finding.endLine - 1), document.lineAt(Math.max(0, finding.endLine - 1)).text.length);
    if (document.getText(range).trim() !== finding.citedSnippet.trim()) { void vscode.window.showWarningMessage('Finding text changed; run review again before applying'); return; }
    const preview = await vscode.workspace.openTextDocument({ language: document.languageId, content: finding.suggestedFix });
    await vscode.window.showTextDocument(preview, { preview: true, viewColumn: vscode.ViewColumn.Beside });
    if (await vscode.window.showWarningMessage(`Replace ${finding.file}:${finding.startLine}-${finding.endLine} with the shown text?`, { modal: true }, 'Apply') !== 'Apply') return;
    const edit = new vscode.WorkspaceEdit();
    edit.replace(uri, range, finding.suggestedFix);
    await vscode.workspace.applyEdit(edit);
  }));

  context.subscriptions.push(vscode.languages.registerCodeActionsProvider({ scheme: 'file' }, {
    provideCodeActions(document, _range, actionContext) {
      if (!current) return [];
      const actions: vscode.CodeAction[] = [];
      for (const diagnostic of actionContext.diagnostics) {
        if (diagnostic.source !== 'open-review') continue;
        const finding = current.findings.find((item) => item.id === diagnostic.code);
        if (!finding) continue;
        for (const [title, state] of [['Dismiss finding', 'ignored'], ['Mark false positive', 'false_positive']] as const) {
          const action = new vscode.CodeAction(title, vscode.CodeActionKind.QuickFix);
          action.diagnostics = [diagnostic];
          action.command = { title, command: 'openReview.markFinding', arguments: [finding.id, state] };
          actions.push(action);
        }
        if (finding.suggestedFix && finding.citedSnippet && document.getText(diagnostic.range).trim() === finding.citedSnippet.trim()) {
          const action = new vscode.CodeAction('Preview and apply suggested text', vscode.CodeActionKind.QuickFix);
          action.diagnostics = [diagnostic];
          action.command = { title: action.title, command: 'openReview.applyFinding', arguments: [finding, document.uri] };
          actions.push(action);
        }
      }
      return actions;
    },
  }, { providedCodeActionKinds: [vscode.CodeActionKind.QuickFix] }));
}

export function deactivate(): void {}
