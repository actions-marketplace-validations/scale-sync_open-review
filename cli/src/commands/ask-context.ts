import { execFile } from 'node:child_process';
import { lstat, readFile } from 'node:fs/promises';
import { isAbsolute, join, relative, resolve, sep } from 'node:path';
import { promisify } from 'node:util';

import { isContextCandidate } from '@openreview/core';

const exec = promisify(execFile);

export interface AskContext {
  prompt: string;
  lineCounts: Map<string, number>;
  paths: string[];
}

export async function loadLocalAskContext(repo: string, question: string): Promise<AskContext> {
  const { stdout: rootOutput } = await exec('git', ['rev-parse', '--show-toplevel'], {
    cwd: repo,
    encoding: 'utf8',
  });
  const root = resolve(rootOutput.trim());
  const { stdout } = await exec('git', ['ls-files', '-z'], {
    cwd: root,
    encoding: 'utf8',
    maxBuffer: 2_000_000,
  });
  const terms = question.toLowerCase().match(/[a-z0-9_]{3,}/g) ?? [];
  const paths = stdout
    .split('\0')
    .filter((path) => path && isContextCandidate(path))
    .sort((a, b) => {
      const score = (path: string) =>
        terms.filter((term) => path.toLowerCase().includes(term)).length;
      return score(b) - score(a) || a.localeCompare(b);
    });
  const snippets: string[] = [];
  const lineCounts = new Map<string, number>();
  let remaining = 20_000;
  for (const path of paths) {
    if (lineCounts.size >= 8 || remaining <= 0) break;
    const full = join(root, path);
    if (isAbsolute(path) || relative(root, full).startsWith('..' + sep)) continue;
    try {
      const info = await lstat(full);
      if (!info.isFile() || info.size > 200_000) continue;
      const content = (await readFile(full, 'utf8')).slice(0, Math.min(remaining, 3_000));
      if (content.includes('\0')) continue;
      const lines = content.split('\n');
      lineCounts.set(path, lines.length);
      snippets.push(
        path + '\n' + lines.map((line, index) => String(index + 1) + ': ' + line).join('\n'),
      );
      remaining -= content.length;
    } catch {
      continue;
    }
  }
  return {
    prompt:
      'You are open-review. Answer from these bounded repository excerpts. Cite code as path:line. If the answer is not in the excerpts, say so.\n' +
      snippets.join('\n\n'),
    lineCounts,
    paths: [...lineCounts.keys()],
  };
}

export function markUnverifiedCitations(answer: string, lineCounts: Map<string, number>): string {
  return answer.replace(
    /\x60([a-zA-Z0-9_/.@-]+\.[a-zA-Z0-9]+):(\d+)\x60/g,
    (match, path: string, rawLine: string) => {
      const line = Number(rawLine);
      return line > 0 && line <= (lineCounts.get(path) ?? 0)
        ? match
        : match + ' *(citation not verified)*';
    },
  );
}
