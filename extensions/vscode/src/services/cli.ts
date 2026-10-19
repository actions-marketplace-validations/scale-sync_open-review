import { spawn } from 'node:child_process';

export interface CliResult { stdout: string; exitCode: number }

export function runCLI(binary: string, args: string[], cwd: string, signal?: AbortSignal): Promise<CliResult> {
  return new Promise((resolve, reject) => {
    const child = spawn(binary, args, { cwd, signal, shell: false, windowsHide: true });
    let stdout = '';
    let stderr = '';
    const limit = 16 << 20;
    child.stdout.on('data', (chunk: Buffer) => {
      stdout += chunk.toString('utf8');
      if (stdout.length > limit) child.kill();
    });
    child.stderr.on('data', (chunk: Buffer) => {
      stderr += chunk.toString('utf8');
      if (stderr.length > limit) child.kill();
    });
    child.on('error', reject);
    child.on('close', (code) => {
      if (stdout.length > limit || stderr.length > limit) reject(new Error('CLI output exceeds 16 MB'));
      else if (code === 0 || code === 2) resolve({ stdout, exitCode: code });
      else reject(new Error(stderr.trim() || `open-review exited ${code}`));
    });
  });
}
