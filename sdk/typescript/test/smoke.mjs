import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { scan } from '../dist/index.js';

const repo = await mkdtemp(join(tmpdir(), 'open-review-node-'));
try {
  const result = await scan(repo, '.');
  assert.equal(result.contractVersion, 2);
  assert.equal(result.status, 'empty');
  assert.deepEqual(result.findings, []);
} finally {
  await rm(repo, { recursive: true });
}
