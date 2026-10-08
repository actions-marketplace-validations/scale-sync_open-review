import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Context } from '@actions/github/lib/context.js';

import { handleComment } from '../../action/src/comment-handler.js';

const mocks = vi.hoisted(() => ({
  getPR: vi.fn(),
  getPRFiles: vi.fn(),
  getPRDiff: vi.fn(),
  postAcknowledgement: vi.fn(),
  postReviewResults: vi.fn(),
  runGoReview: vi.fn(),
}));

vi.mock('../../core/dist/index.mjs', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@openreview/core')>();
  return {
    ...actual,
    GitHubClient: class {
      getPR = mocks.getPR;
      getPRFiles = mocks.getPRFiles;
      getPRDiff = mocks.getPRDiff;
    },
    CommentPoster: class {
      postAcknowledgement = mocks.postAcknowledgement;
      postReviewResults = mocks.postReviewResults;
    },
    loadRepoConfigFromGitHub: vi.fn().mockResolvedValue({ instructions: '' }),
    runGoReview: mocks.runGoReview,
  };
});

describe('deep PR command', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubEnv('INPUT_GITHUB-TOKEN', 'test-token');
    mocks.getPR.mockResolvedValue({
      title: 'Change',
      body: '',
      head: { sha: 'head' },
      base: { sha: 'base' },
      user: { login: 'dev' },
    });
    mocks.getPRFiles.mockResolvedValue([{ filename: 'main.go' }]);
    mocks.getPRDiff.mockResolvedValue('diff --git a/main.go b/main.go\n@@ -0,0 +1 @@\n+bad\n');
    mocks.runGoReview.mockResolvedValue({
      status: 'complete',
      findings: [],
      summary: { mode: 'rlm', totalFindings: 0 },
      coverage: [],
      audit: [{ stage: 'sandbox_1', status: 'complete' }],
    });
  });

  it('sends the deep request through Go and updates review state', async () => {
    const context = {
      repo: { owner: 'org', repo: 'repo' },
      payload: {
        issue: { number: 7 },
        comment: { id: 9, body: '@open-review rlm', user: { login: 'dev' } },
      },
    } as unknown as Context;
    await handleComment(context);

    expect(mocks.runGoReview).toHaveBeenCalledWith(
      expect.anything(),
      [{ filename: 'main.go' }],
      100,
      'deep',
    );
    expect(mocks.postReviewResults).toHaveBeenCalledOnce();
  });
});
