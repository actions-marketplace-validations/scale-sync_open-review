import { describe, expect, it } from 'vitest';

import { toGoRequest } from '../../core/src/review/go-review.js';
import type { PRFile } from '../../core/src/github/client.js';
import type { PRContext } from '../../core/src/review/types.js';

describe('Go review request', () => {
  it('sends full patches and metadata, never credentials', () => {
    const pr: PRContext = {
      owner: 'org',
      repo: 'repo',
      prNumber: 4,
      metadata: { title: 'Fix bug', body: '', baseSha: 'base', headSha: 'head', author: 'dev' },
      files: ['new.go', 'image.png'],
      diff:
        'diff --git a/old.go b/new.go\n--- a/old.go\n+++ b/new.go\n@@ -1 +1 @@\n-old\n+new\n' +
        'diff --git a/image.png b/image.png\nBinary files differ\n',
      instructions: 'Check errors',
      learnings: [],
    };
    const files: PRFile[] = [
      {
        filename: 'new.go',
        previous_filename: 'old.go',
        status: 'renamed',
        additions: 1,
        deletions: 1,
        changes: 2,
      },
      { filename: 'image.png', status: 'added', additions: 0, deletions: 0, changes: 0 },
    ];
    const request = toGoRequest(pr, files);
    expect(request.changes[0]).toMatchObject({
      path: 'new.go',
      oldPath: 'old.go',
      status: 'renamed',
      patch: expect.stringContaining('+new'),
    });
    expect(request.changes[1].patch).toContain('Binary files differ');
    expect(request.title).toBe('Fix bug');
    expect(request.instructions).toBe('Check errors');
    expect(request.reviewKind).toBe('fast');
    expect(request.reviewEffort).toBe('medium');
    expect(toGoRequest(pr, files, 'deep').reviewKind).toBe('deep');
    expect(JSON.stringify(request)).not.toContain('api-key');
    const override = toGoRequest(pr, files, 'fast', {
      provider: 'anthropic',
      model: 'claude-test',
      totalTokens: 5000,
      effort: 'high',
    });
    expect(override.model).toEqual({ provider: 'anthropic', name: 'claude-test', baseUrl: '' });
    expect(override.limits.totalTokens).toBe(5000);
    expect(override.reviewEffort).toBe('high');
    expect(JSON.stringify(override)).not.toContain('secret');
  });
});
