import { chmodSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { afterEach, describe, expect, it, vi } from 'vitest';

import { runGoReview } from '../../../core/src/review/go-review.js';
import type { PRFile } from '../../../core/src/github/client.js';
import type { PRContext } from '../../../core/src/review/types.js';

const folders: string[] = [];
afterEach(() => {
  vi.unstubAllEnvs();
  for (const folder of folders.splice(0)) rmSync(folder, { recursive: true, force: true });
});

const pr: PRContext = {
  owner: 'org',
  repo: 'repo',
  prNumber: 4,
  metadata: { title: 'Fix bug', body: '', baseSha: 'base', headSha: 'head', author: 'dev' },
  diff: 'diff --git a/x.go b/x.go\n@@ -0,0 +1 @@\n+x\n',
  files: ['x.go'],
  instructions: 'Review errors',
  learnings: [],
};
const files: PRFile[] = [
  { filename: 'x.go', status: 'modified', additions: 1, deletions: 0, changes: 1 },
];

describe('Go process adapter', () => {
  it('sends selected PR metadata through stdin and parses coverage', async () => {
    const folder = mkdtempSync(join(tmpdir(), 'open-review-adapter-'));
    folders.push(folder);
    const binary = join(folder, 'review-stub');
    writeFileSync(
      binary,
      '#!/usr/bin/env node\n' +
        "let input='';process.stdin.on('data',x=>input+=x);process.stdin.on('end',()=>{" +
        "const request=JSON.parse(input);if(request.changes[0].patch.includes('+x')&&process.argv[2]==='review-json'&&process.argv[3]==='--max-files'&&process.argv[4]==='1')" +
        "process.stdout.write(JSON.stringify({status:'complete',findings:[],coverage:[{path:'x.go',state:'reviewed'}]}));else process.exitCode=2;});\n",
    );
    chmodSync(binary, 0o755);
    vi.stubEnv('OPEN_REVIEW_BIN', binary);
    const result = await runGoReview(pr, files, 1);
    expect(result.coverage).toEqual([{ path: 'x.go', state: 'reviewed' }]);
  });

  it('retains a failed coverage result when the process exits nonzero', async () => {
    const folder = mkdtempSync(join(tmpdir(), 'open-review-adapter-'));
    folders.push(folder);
    const binary = join(folder, 'review-stub');
    writeFileSync(
      binary,
      '#!/usr/bin/env node\n' +
        "process.stdout.write(JSON.stringify({status:'failed',error:'model unavailable',findings:[],coverage:[{path:'x.go',state:'failed'}]}));process.exitCode=1;\n",
    );
    chmodSync(binary, 0o755);
    vi.stubEnv('OPEN_REVIEW_BIN', binary);
    const result = await runGoReview(pr, files);
    expect(result.status).toBe('failed');
    expect(result.coverage[0].state).toBe('failed');
  });
});
