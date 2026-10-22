import { spawn } from 'node:child_process';

export const CONTRACT_VERSION = 2;

export interface CoverageEntry {
  path: string;
  oldPath?: string;
  status: string;
  state: string;
  reason?: string;
  patchBytes: number;
  sentBytes: number;
}

export interface Finding {
  id: string;
  category: 'bug' | 'flag';
  severity: 'severe' | 'non-severe' | 'investigate' | 'informational';
  file: string;
  startLine: number;
  endLine: number;
  title: string;
  explanation: string;
  suggestedFix?: string;
  source: string;
}

export interface Usage {
  promptTokens: number;
  outputTokens: number;
  totalTokens: number;
  modelCalls: number;
  estimated: boolean;
}

export interface Result {
  contractVersion: number;
  status: 'complete' | 'partial' | 'empty' | 'skipped' | 'failed';
  coverage: CoverageEntry[];
  findings: Finding[];
  usage: Usage;
  error?: string;
  request: Record<string, unknown>;
  summary?: Record<string, unknown>;
  trace?: Record<string, unknown>;
}

export interface ReviewOptions {
  repo: string;
  mode: 'workspace' | 'commit' | 'branch' | 'scan';
  commit?: string;
  base?: string;
  head?: string;
  scanPath?: string;
  model?: string;
  provider?: string;
  highRecall?: boolean;
  binary?: string;
  signal?: AbortSignal;
}

export interface ProviderConfigOptions {
  provider: string;
  model: string;
  apiKey: string;
  baseUrl?: string;
  binary?: string;
}

export class ReviewError extends Error {
  constructor(message: string, public readonly exitCode?: number) {
    super(message);
    this.name = 'ReviewError';
  }
}

/** Replace the saved BYOK provider and key without putting the secret in argv. */
export async function configureProvider(options: ProviderConfigOptions): Promise<void> {
  const binary = options.binary ?? process.env.OPEN_REVIEW_BIN ?? 'open-review';
  const args = ['config', 'set', '--provider', options.provider, '--model', options.model, '--api-key-stdin'];
  if (options.baseUrl) args.push('--base-url', options.baseUrl);
  await new Promise<void>((resolve, reject) => {
    const child = spawn(binary, args, { stdio: ['pipe', 'pipe', 'pipe'] });
    let stderr = '';
    child.stderr.setEncoding('utf8');
    child.stderr.on('data', (chunk: string) => { stderr += chunk; });
    child.on('error', (error) => reject(new ReviewError(error.message)));
    child.on('close', (code) => code === 0 ? resolve() : reject(new ReviewError(stderr.trim() || 'open-review config failed', code ?? undefined)));
    child.stdin.end(`${options.apiKey}\n`);
  });
}

export async function review(options: ReviewOptions): Promise<Result> {
  const args = ['review', '--repo', options.repo, '--output', 'json', '--quiet'];
  switch (options.mode) {
    case 'workspace': args.push('--workspace'); break;
    case 'commit':
      if (!options.commit) throw new ReviewError('commit mode requires commit');
      args.push('--commit', options.commit);
      break;
    case 'branch':
      if (!options.base || !options.head) throw new ReviewError('branch mode requires base and head');
      args.push('--base', options.base, '--head', options.head);
      break;
    case 'scan':
      if (!options.scanPath) throw new ReviewError('scan mode requires scanPath');
      args.push('--scan', options.scanPath);
      break;
  }
  if (options.model) args.push('--model', options.model);
  if (options.provider) args.push('--provider', options.provider);
  if (options.highRecall) args.push('--high-recall');

  const binary = options.binary ?? process.env.OPEN_REVIEW_BIN ?? 'open-review';
  return new Promise<Result>((resolve, reject) => {
    const child = spawn(binary, args, { signal: options.signal, stdio: ['ignore', 'pipe', 'pipe'] });
    let stdout = '';
    let stderr = '';
    child.stdout.setEncoding('utf8');
    child.stderr.setEncoding('utf8');
    child.stdout.on('data', (chunk: string) => { stdout += chunk; });
    child.stderr.on('data', (chunk: string) => { stderr += chunk; });
    child.on('error', (error) => reject(new ReviewError(error.message)));
    child.on('close', (code) => {
      let parsed: Result;
      try { parsed = JSON.parse(stdout) as Result; }
      catch { reject(new ReviewError(stderr.trim() || 'open-review returned invalid JSON', code ?? undefined)); return; }
      if (parsed.contractVersion !== CONTRACT_VERSION) {
        reject(new ReviewError(`Unsupported contract version ${parsed.contractVersion}`, code ?? undefined));
      } else if (code !== 0 && code !== 2) {
        reject(new ReviewError(parsed.error || stderr.trim() || 'open-review failed', code ?? undefined));
      } else {
        resolve(parsed);
      }
    });
  });
}

export function scan(repo: string, path: string, options: Omit<ReviewOptions, 'repo' | 'mode' | 'scanPath'> = {}): Promise<Result> {
  return review({ ...options, repo, mode: 'scan', scanPath: path });
}
