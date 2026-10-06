import { describe, expect, it } from 'vitest';

import { selectActionDiff } from '../../action/src/review-scope.js';

const patches = [
  'diff --git a/src/first.ts b/src/first.ts\n@@ -0,0 +1 @@\n+first\n',
  'diff --git a/src/second.ts b/src/second.ts\n@@ -0,0 +1 @@\n+second\n',
  'diff --git a/src/third.ts b/src/third.ts\n@@ -0,0 +1 @@\n+third\n',
  'diff --git a/pnpm-lock.yaml b/pnpm-lock.yaml\n@@ -0,0 +1 @@\n+lock\n',
].join('');

describe('Action review scope', () => {
  it('applies max-files to the actual model diff', () => {
    const result = selectActionDiff(
      patches,
      ['src/first.ts', 'src/second.ts', 'src/third.ts', 'pnpm-lock.yaml'],
      2,
    );
    expect(result.files).toEqual(['src/first.ts', 'src/second.ts']);
    expect(result.diff).toContain('+first');
    expect(result.diff).toContain('+second');
    expect(result.diff).not.toContain('+third');
    expect(result.diff).not.toContain('+lock');
    expect(result.coverage.map((entry) => [entry.state, entry.reason])).toEqual([
      ['selected', undefined],
      ['selected', undefined],
      ['skipped', 'file_limit'],
      ['skipped', 'unsupported_file'],
    ]);
  });

  it('skips files missing from the patch and rejects invalid limits', () => {
    const result = selectActionDiff(patches, ['missing.ts', 'src/first.ts'], 1);
    expect(result.files).toEqual(['src/first.ts']);
    expect(result.coverage[0].reason).toBe('missing_patch');
    expect(() => selectActionDiff(patches, ['src/first.ts'], -1)).toThrow('positive integer');
  });

  it('marks a large patch partial before the model receives it', () => {
    const large = 'diff --git a/big.ts b/big.ts\n@@ -0,0 +1 @@\n+' + 'x'.repeat(41_000) + '\n';
    const result = selectActionDiff(large, ['big.ts'], 1);
    expect(result.coverage[0]).toMatchObject({ state: 'partial', reason: 'budget_truncated' });
    expect(result.files).toEqual(['big.ts']);
  });
});
