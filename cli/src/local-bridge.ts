import { basename } from 'node:path';

import { loadConfig, loadInstructions, runFastReview, validateConfig } from '@openreview/core';
import type { PRContext } from '@openreview/core';

interface LocalRequest {
  contractVersion: number;
  mode: 'workspace' | 'branch' | 'commit';
  repoRoot: string;
  baseRef?: string;
  headRef?: string;
  baseSha?: string;
  headSha?: string;
  commitSha?: string;
  diff: string;
  files: string[];
}

async function main(): Promise<void> {
  let input = '';
  for await (const chunk of process.stdin) {
    input += chunk.toString();
    if (input.length > 10_000_000) throw new Error('Review request exceeds 10 MB');
  }
  const request = JSON.parse(input) as LocalRequest;
  if (
    request.contractVersion !== 1 ||
    !['workspace', 'branch', 'commit'].includes(request.mode) ||
    typeof request.repoRoot !== 'string' ||
    request.repoRoot.length === 0 ||
    typeof request.diff !== 'string' ||
    !Array.isArray(request.files) ||
    !request.files.every((file) => typeof file === 'string' && file.length > 0)
  ) {
    throw new Error('Invalid local review request');
  }
  validateConfig(loadConfig());
  const instructionSet = await loadInstructions(request.repoRoot);
  const relevant = [...instructionSet.scopedInstructions]
    .filter(([dir]) => request.files.some((file) => file.startsWith(dir + '/')))
    .map(([dir, content]) => `## ${dir}\n${content}`);
  const label =
    request.mode === 'workspace'
      ? 'Workspace changes'
      : request.mode === 'branch'
        ? `Branch comparison: ${request.baseRef} → ${request.headRef}`
        : `Commit: ${request.commitSha}`;
  const context: PRContext = {
    owner: 'local',
    repo: basename(request.repoRoot),
    prNumber: 0,
    diff: request.diff,
    files: request.files,
    metadata: {
      title: label,
      body: '',
      headSha: request.headSha ?? request.commitSha ?? '',
      baseSha: request.baseSha ?? '',
      author: 'local',
    },
    instructions: [instructionSet.globalInstructions, ...relevant].filter(Boolean).join('\n\n'),
    learnings: [],
  };
  const result = await runFastReview(context);
  process.stdout.write(JSON.stringify(result));
}

main().catch((error: unknown) => {
  process.stderr.write(`Error: ${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
});
