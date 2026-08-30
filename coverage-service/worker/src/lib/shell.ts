import { spawn } from 'child_process';
import { existsSync, readdirSync, statSync } from 'fs';
import { join } from 'path';

export interface ShellResult {
  stdout: string;
  stderr: string;
  exitCode: number;
}

/**
 * Env vars needed to run npm/node/python/git for an untrusted PR checkout —
 * nothing that could carry a secret (API keys, GitHub tokens, DB/Redis URLs).
 * `command` here runs attacker-controlled code (package.json scripts, test
 * files) from a cloned PR branch, so it must never see the parent process's
 * full environment.
 */
const SANDBOX_ENV_KEYS = [
  'PATH',
  'HOME',
  'LANG',
  'LC_ALL',
  'LC_CTYPE',
  'TMPDIR',
  'TMP',
  'TEMP',
  'SHELL',
];

function buildSandboxEnv(): NodeJS.ProcessEnv {
  const env: NodeJS.ProcessEnv = { CI: 'true', FORCE_COLOR: '0' };
  for (const key of SANDBOX_ENV_KEYS) {
    const value = process.env[key];
    if (value !== undefined) env[key] = value;
  }
  return env;
}

export async function runCommand(
  command: string,
  cwd: string,
  timeoutMs?: number,
): Promise<ShellResult> {
  const timeout = timeoutMs ?? parseInt(process.env.EXECUTION_TIMEOUT_MS ?? '600000', 10);

  return new Promise((resolve, reject) => {
    const proc = spawn('sh', ['-c', command], {
      cwd,
      stdio: 'pipe',
      env: buildSandboxEnv(),
    });

    let stdout = '';
    let stderr = '';
    proc.stdout.on('data', (d) => (stdout += d.toString()));
    proc.stderr.on('data', (d) => (stderr += d.toString()));

    const timer = setTimeout(() => {
      proc.kill('SIGKILL');
      reject(new Error(`Command timed out after ${timeout}ms: ${command}`));
    }, timeout);

    proc.on('close', (code) => {
      clearTimeout(timer);
      resolve({ stdout, stderr, exitCode: code ?? 1 });
    });

    proc.on('error', (err) => {
      clearTimeout(timer);
      reject(err);
    });
  });
}

const COVERAGE_XML_NAMES = new Set([
  'coverage.xml',
  'cobertura-coverage.xml',
  'cobertura.xml',
]);

const SKIP_DIRS = new Set([
  '.git',
  'node_modules',
  '.venv',
  'venv',
  '__pycache__',
  'dist',
  'build',
]);

export function findCoverageXml(repoDir: string): string | null {
  const candidates = [
    'coverage.xml',
    'coverage/coverage.xml',
    'coverage/cobertura-coverage.xml',
    'cobertura.xml',
    'reports/coverage.xml',
  ];

  for (const candidate of candidates) {
    const full = join(repoDir, candidate);
    if (existsSync(full)) return full;
  }

  return findCoverageXmlRecursive(repoDir, 0, 6);
}

function findCoverageXmlRecursive(
  dir: string,
  depth: number,
  maxDepth: number,
): string | null {
  if (depth > maxDepth) return null;

  let entries: string[];
  try {
    entries = readdirSync(dir);
  } catch {
    return null;
  }

  for (const entry of entries) {
    if (COVERAGE_XML_NAMES.has(entry)) {
      return join(dir, entry);
    }
  }

  for (const entry of entries) {
    if (SKIP_DIRS.has(entry)) continue;

    const full = join(dir, entry);
    let stat;
    try {
      stat = statSync(full);
    } catch {
      continue;
    }

    if (stat.isDirectory()) {
      const found = findCoverageXmlRecursive(full, depth + 1, maxDepth);
      if (found) return found;
    }
  }

  return null;
}

export async function cleanupDir(dir: string) {
  const { rm } = await import('fs/promises');
  try {
    await rm(dir, { recursive: true, force: true });
  } catch {
    // best effort cleanup
  }
}
