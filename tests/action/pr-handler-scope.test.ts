import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Context } from '@actions/github/lib/context.js';

import { handlePullRequest } from '../../action/src/pr-handler.js';

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
    loadRepoConfigFromGitHub: vi
      .fn()
      .mockResolvedValue({ instructions: '', reviewBuddyConfig: {} }),
    runGoReview: mocks.runGoReview,
  };
});

describe('pull request Action review scope', () => {
  afterEach(() => vi.unstubAllEnvs());
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubEnv('INPUT_GITHUB-TOKEN', 'test-token');
    vi.stubEnv('INPUT_MAX-FILES', '1');
    mocks.getPR.mockResolvedValue({
      title: 'Change',
      body: '',
      head: { sha: 'head' },
      base: { sha: 'base' },
      user: { login: 'author' },
    });
    mocks.getPRFiles.mockResolvedValue([{ filename: 'first.ts' }, { filename: 'second.ts' }]);
    mocks.getPRDiff.mockResolvedValue(
      'diff --git a/first.ts b/first.ts\n@@ -0,0 +1 @@\n+first\n' +
        'diff --git a/second.ts b/second.ts\n@@ -0,0 +1 @@\n+second\n',
    );
    mocks.runGoReview.mockResolvedValue({
      status: 'complete',
      findings: [],
      summary: { duration: '0ms', totalFindings: 0 },
      coverage: [
        { path: 'first.ts', state: 'reviewed' },
        { path: 'second.ts', state: 'skipped', reason: 'file_limit' },
      ],
    });
    mocks.postReviewResults.mockResolvedValue({
      duration: '0ms',
      totalFindings: 0,
      resolvedCount: 0,
    });
  });

  it('passes the complete PR and file limit to the Go engine', async () => {
    const context = {
      repo: { owner: 'test', repo: 'repo' },
      payload: { pull_request: { number: 1, title: 'Change', draft: false, body: '' } },
    } as unknown as Context;
    await handlePullRequest(context);

    expect(mocks.runGoReview).toHaveBeenCalledOnce();
    const input = mocks.runGoReview.mock.calls[0][0] as { files: string[]; diff: string };
    expect(input.files).toEqual(['first.ts', 'second.ts']);
    expect(input.diff).toContain('+first');
    expect(input.diff).toContain('+second');
    expect(mocks.runGoReview.mock.calls[0][2]).toBe(1);
  });

  it('does not publish an approved summary when no file was reviewed', async () => {
    mocks.runGoReview.mockResolvedValue({
      status: 'skipped',
      findings: [],
      coverage: [{ path: 'image.png', state: 'skipped', reason: 'unsupported_file' }],
    });
    const context = {
      repo: { owner: 'test', repo: 'repo' },
      payload: { pull_request: { number: 1, title: 'Change', draft: false, body: '' } },
    } as unknown as Context;
    await handlePullRequest(context);
    expect(mocks.postReviewResults).not.toHaveBeenCalled();
    expect(mocks.postAcknowledgement).toHaveBeenCalledTimes(1);
  });
});
