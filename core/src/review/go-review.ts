import { spawn } from 'node:child_process';
import { existsSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import type { PRFile } from '../github/client.js';

import { extractDiffForFiles } from './diff-fingerprint.js';
import type { PRContext, ReviewFinding, ReviewSummary, ReviewTrace } from './types.js';

export interface GoReviewResult {
  status: 'empty' | 'skipped' | 'complete' | 'partial' | 'failed';
  findings: ReviewFinding[];
  summary?: ReviewSummary;
  trace?: ReviewTrace;
  coverage: Array<{ path: string; state: string; reason?: string }>;
  audit?: Array<{ stage: string; status: string; message?: string }>;
  error?: string;
  usage?: {
    promptTokens: number;
    outputTokens: number;
    totalTokens: number;
    modelCalls: number;
    estimated: boolean;
  };
}

export interface GoReviewOptions {
  effort?: 'low' | 'medium' | 'high';
  provider?: string;
  model?: string;
  baseUrl?: string;
  promptTokens?: number;
  outputTokens?: number;
  totalTokens?: number;
  requestSeconds?: number;
  groupSeconds?: number;
}

function limit(value: number | undefined, key: string): number {
  const selected = value ?? (process.env[key] ? Number(process.env[key]) : 0);
  if (!Number.isSafeInteger(selected) || selected < 0)
    throw new Error(`${key} must be a nonnegative integer`);
  return selected;
}

export function toGoRequest(
  pr: PRContext,
  files: PRFile[],
  reviewKind: 'fast' | 'deep' = 'fast',
  options: GoReviewOptions = {},
) {
  return {
    contractVersion: 2,
    mode: 'branch',
    repoRoot: pr.owner + '/' + pr.repo,
    baseSha: pr.metadata.baseSha,
    headSha: pr.metadata.headSha,
    title: pr.metadata.title,
    instructions: pr.instructions,
    reviewKind,
    reviewEffort: options.effort || process.env.OPEN_REVIEW_EFFORT || 'medium',
    model: {
      provider: options.provider || process.env.OPEN_REVIEW_PROVIDER || '',
      name: options.model || process.env.MAIN_MODEL || '',
      baseUrl: options.baseUrl || process.env.OPEN_REVIEW_BASE_URL || '',
    },
    limits: {
      promptTokens: limit(options.promptTokens, 'OPEN_REVIEW_PROMPT_TOKENS'),
      outputTokens: limit(options.outputTokens, 'OPEN_REVIEW_OUTPUT_TOKENS'),
      totalTokens: limit(options.totalTokens, 'OPEN_REVIEW_TOTAL_TOKENS'),
      requestSeconds: limit(options.requestSeconds, 'OPEN_REVIEW_REQUEST_SECONDS'),
      groupSeconds: limit(options.groupSeconds, 'OPEN_REVIEW_GROUP_SECONDS'),
    },
    diff: '',
    files: [],
    changes: files.map((file) => ({
      path: file.filename,
      ...(file.previous_filename ? { oldPath: file.previous_filename } : {}),
      status:
        file.status === 'removed'
          ? 'deleted'
          : file.status === 'changed'
            ? 'type_changed'
            : file.status === 'unchanged'
              ? 'modified'
              : file.status,
      binary: false,
      patch: extractDiffForFiles(pr.diff, [file.filename]),
      hunks: [],
    })),
  };
}

function sourceRoot(): string {
  let directory = dirname(fileURLToPath(import.meta.url));
  while (directory !== dirname(directory)) {
    if (existsSync(join(directory, 'go.mod'))) return directory;
    directory = dirname(directory);
  }
  throw new Error(
    'Go review source unavailable; set OPEN_REVIEW_BIN to a built open-review executable',
  );
}

export async function runGoReview(
  pr: PRContext,
  files: PRFile[],
  maxFiles = 100,
  reviewKind: 'fast' | 'deep' = 'fast',
  options: GoReviewOptions = {},
): Promise<GoReviewResult> {
  if (!Number.isSafeInteger(maxFiles) || maxFiles < 1) {
    throw new Error('max-files must be a positive integer');
  }
  const bundledAction = dirname(fileURLToPath(import.meta.url)).endsWith(join('action', 'dist'));
  if (bundledAction && (process.platform !== 'linux' || process.arch !== 'x64')) {
    throw new Error('This Action requires a Linux x64 runner');
  }
  const root = bundledAction || !process.env.OPEN_REVIEW_BIN ? sourceRoot() : process.cwd();
  const binary = bundledAction
    ? join(root, 'action/bin/open-review-linux-amd64')
    : process.env.OPEN_REVIEW_BIN || '';
  if (binary && !existsSync(binary)) throw new Error('Go review executable is missing: ' + binary);
  const command = binary || 'go';
  const args = binary
    ? ['review-json', '--max-files', String(maxFiles)]
    : ['run', './cmd/open-review', 'review-json', '--max-files', String(maxFiles)];
  const cwd = binary ? process.cwd() : root;
  const request = JSON.stringify(toGoRequest(pr, files, reviewKind, options));
  if (Buffer.byteLength(request) > 10_000_000) throw new Error('Review request exceeds 10 MB');

  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { cwd, env: process.env, stdio: ['pipe', 'pipe', 'pipe'] });
    let stdout = '';
    let stderr = '';
    child.stdout.setEncoding('utf8');
    child.stderr.setEncoding('utf8');
    child.stdout.on('data', (chunk: string) => {
      stdout += chunk;
      if (stdout.length > 10_000_000) child.kill();
    });
    child.stderr.on('data', (chunk: string) => {
      stderr += chunk;
      if (stderr.length > 100_000) child.kill();
    });
    child.on('error', reject);
    child.stdin.on('error', reject);
    child.on('close', (code) => {
      try {
        const result = JSON.parse(stdout) as GoReviewResult;
        if (!Array.isArray(result.findings) || !Array.isArray(result.coverage)) {
          throw new Error('Invalid Go review result');
        }
        resolve(result);
      } catch (error) {
        reject(code === 0 ? error : new Error(stderr.trim() || 'Go review failed'));
      }
    });
    child.stdin.end(request);
  });
}
