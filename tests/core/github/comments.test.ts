import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { GitHubClient } from '../../../core/src/github/client.js';
import { CommentPoster } from '../../../core/src/github/comments.js';
import type { PRContext, ReviewFinding, ReviewSummary } from '../../../core/src/review/types.js';

function makeMockClient() {
  const api = {
    get: vi.fn(),
    post: vi.fn(),
    patch: vi.fn(),
    delete: vi.fn(),
  };

  const client = {
    owner: 'test-owner',
    repo: 'test-repo',
    api,
    getPR: vi.fn().mockResolvedValue({ head: { sha: 'head-sha-2' } }),
  } as unknown as GitHubClient;

  return { client, api };
}

function makeFinding(
  id: string,
  file = 'src/index.ts',
  line = 10,
  title = `Finding ${id}`,
): ReviewFinding {
  return {
    id,
    category: 'bug',
    severity: 'severe',
    file,
    startLine: line,
    endLine: line,
    title,
    explanation: 'Test explanation',
    verification: 'verified',
    source: 'ai',
    citations: [],
  };
}

function makeSummary(totalFindings = 1): ReviewSummary {
  return {
    filesReviewed: 5,
    duration: '10s',
    mode: 'fast',
    findingsBySeverity: {
      severe: totalFindings,
      'non-severe': 0,
      investigate: 0,
      informational: 0,
    },
    totalFindings,
  };
}

function makePr(): PRContext {
  return {
    owner: 'test-owner',
    repo: 'test-repo',
    prNumber: 7,
    diff: 'diff --git a/src/b.ts b/src/b.ts\n--- a/src/b.ts\n+++ b/src/b.ts\n@@ -1,0 +12,2 @@\n+first\n+second\ndiff --git a/src/index.ts b/src/index.ts\n--- a/src/index.ts\n+++ b/src/index.ts\n@@ -1,0 +10 @@\n+line',
    files: ['src/index.ts'],
    metadata: {
      title: 'Test PR',
      body: 'Test body',
      headSha: 'head-sha-2',
      baseSha: 'base-sha',
      author: 'dev',
    },
    instructions: '',
    learnings: [],
  };
}

function encodedState(findings: ReviewFinding[]) {
  const state = {
    version: 1,
    headSha: 'head-sha-1',
    reviewedAt: '2026-01-01T00:00:00.000Z',
    findings: findings.map((f) => ({
      fingerprint: `${f.file}:${f.startLine}:${f.title.toLowerCase()}`,
      category: f.category,
      severity: f.severity,
      file: f.file,
      startLine: f.startLine,
      title: f.title,
    })),
  };
  const payload = Buffer.from(JSON.stringify(state), 'utf-8').toString('base64url');
  return `<!-- openreview-summary -->\n<!-- openreview-state:${payload} -->`;
}

describe('postReviewResults', () => {
  let poster: CommentPoster;
  let api: ReturnType<typeof makeMockClient>['api'];

  beforeEach(() => {
    vi.clearAllMocks();
    const m = makeMockClient();
    poster = new CommentPoster(m.client);
    api = m.api;
  });

  it('posts only new inline comments on a follow-up commit', async () => {
    const previousFinding = makeFinding('old', 'src/a.ts', 5, 'Old issue');
    const newFinding = makeFinding('new', 'src/b.ts', 12, 'New issue');

    api.get.mockResolvedValue({
      data: [{ id: 200, body: encodedState([previousFinding]) }],
    });
    api.post.mockResolvedValue({ data: {} });
    api.patch.mockResolvedValue({ data: {} });

    const summary = await poster.postReviewResults(7, makePr(), [newFinding], makeSummary(1));

    expect(api.post).toHaveBeenCalledOnce();
    const [reviewUrl, reviewBody] = api.post.mock.calls[0];
    expect(reviewUrl).toContain('/pulls/7/reviews');
    expect(reviewBody.comments).toHaveLength(1);
    expect(reviewBody.comments[0].path).toBe('src/b.ts');
    expect(reviewBody.commit_id).toBe('head-sha-2');

    expect(summary.resolvedCount).toBe(1);
    expect(summary.newCount).toBe(1);
    expect(api.patch).toHaveBeenCalledOnce();
  });

  it('skips inline review when all findings are unchanged', async () => {
    const finding = makeFinding('same', 'src/a.ts', 5, 'Same issue');

    api.get.mockResolvedValue({
      data: [{ id: 200, body: encodedState([finding]) }],
    });
    api.patch.mockResolvedValue({ data: {} });

    const summary = await poster.postReviewResults(7, makePr(), [finding], makeSummary(1));

    expect(api.post).not.toHaveBeenCalled();
    expect(summary.newCount).toBe(0);
    expect(summary.resolvedCount).toBe(0);
    expect(api.patch).toHaveBeenCalledOnce();
  });

  it('does not duplicate inline comments when a summary post fails and the event retries', async () => {
    const finding = makeFinding('retry');
    let inlineBody = '';
    let failSummary = true;
    api.get.mockImplementation(async (url: string) => ({
      data: url.includes('/pulls/7/comments') ? (inlineBody ? [{ body: inlineBody }] : []) : [],
    }));
    api.post.mockImplementation(
      async (url: string, payload: { body?: string; comments?: Array<{ body: string }> }) => {
        if (url.includes('/pulls/7/reviews')) inlineBody = payload.comments?.[0].body ?? '';
        if (url.includes('/issues/7/comments') && failSummary)
          throw new Error('summary write failed');
        return { data: {} };
      },
    );

    await expect(poster.postReviewResults(7, makePr(), [finding], makeSummary())).rejects.toThrow(
      'summary write failed',
    );
    failSummary = false;
    await poster.postReviewResults(7, makePr(), [finding], makeSummary());

    expect(
      api.post.mock.calls.filter(([url]) => String(url).includes('/pulls/7/reviews')),
    ).toHaveLength(1);
    expect(inlineBody).toContain('<!-- openreview-finding:');
  });

  it('does not replace the summary after the PR head moves', async () => {
    const { client, api } = makeMockClient();
    vi.mocked(client.getPR).mockResolvedValue({ head: { sha: 'new-head' } } as never);
    api.get.mockResolvedValue({ data: [] });
    await expect(new CommentPoster(client).postReviewResults(7, makePr(), [], makeSummary(0))).rejects.toThrow('head changed');
    expect(api.patch).not.toHaveBeenCalled();
    expect(api.post).not.toHaveBeenCalled();
  });
});

describe('safe inline positions', () => {
  it('posts a multi-line suggestion on the pinned head and keeps uncertain findings in the summary', async () => {
    const { client, api } = makeMockClient();
    const poster = new CommentPoster(client);
    api.get.mockResolvedValue({ data: [] });
    api.post.mockResolvedValue({ data: {} });
    const finding = {
      ...makeFinding('range', 'src/b.ts', 12),
      endLine: 13,
      suggestedFix: 'fixed();',
      verification: 'verified' as const,
    };
    const uncertain = {
      ...makeFinding('uncertain', 'src/b.ts', 99),
      verification: 'uncertain' as const,
      anchorStatus: 'unanchored' as const,
      verificationReason: 'outside added lines',
    };
    await poster.postReviewResults(7, makePr(), [finding, uncertain], makeSummary(2));
    const review = api.post.mock.calls.find(([url]) =>
      String(url).includes('/pulls/7/reviews'),
    )![1];
    expect(review.comments).toHaveLength(1);
    expect(review.comments[0]).toMatchObject({ line: 13, start_line: 12, start_side: 'RIGHT' });
    expect(review.comments[0].body).toContain('```suggestion');
    expect(
      api.post.mock.calls.find(([url]) => String(url).includes('/issues/7/comments'))![1].body,
    ).toContain('needs checking');
  });

  it('rejects a moved head before posting', async () => {
    const { client, api } = makeMockClient();
    vi.mocked(client.getPR).mockResolvedValue({ head: { sha: 'new-head' } } as never);
    await expect(
      new CommentPoster(client).postReview(7, [makeFinding('moved')], 'head-sha-2', makePr().diff),
    ).rejects.toThrow('head changed');
    expect(api.post).not.toHaveBeenCalled();
  });

  it('keeps unchanged and invalid ranges out of the inline review', async () => {
    const { client, api } = makeMockClient();
    api.get.mockResolvedValue({ data: [] });
    const poster = new CommentPoster(client);
    await poster.postReview(
      7,
      [makeFinding('outside', 'src/b.ts', 99)],
      'head-sha-2',
      makePr().diff,
    );
    expect(api.post).not.toHaveBeenCalled();
  });
});

describe('postSummaryComment', () => {
  it('creates new comment when no existing summary', async () => {
    const { client, api } = makeMockClient();
    const poster = new CommentPoster(client);
    api.get.mockResolvedValue({ data: [] });
    api.post.mockResolvedValue({ data: {} });

    await poster.postSummaryComment(7, makeSummary());

    expect(api.post).toHaveBeenCalledOnce();
    expect(api.post.mock.calls[0][1].body).toContain('<!-- openreview-summary -->');
  });

  it('updates the newest summary when it is already the latest timeline comment', async () => {
    const { client, api } = makeMockClient();
    const poster = new CommentPoster(client);
    api.get
      .mockResolvedValueOnce({
        data: [{ id: 300, body: '<!-- openreview-summary -->\nnewest' }],
      })
      .mockResolvedValueOnce({
        data: [{ id: 300, body: '<!-- openreview-summary -->\nnewest' }],
      });
    api.patch.mockResolvedValue({ data: {} });

    await poster.postSummaryComment(7, makeSummary(2));

    expect(api.patch).toHaveBeenCalledWith(
      '/repos/test-owner/test-repo/issues/comments/300',
      expect.objectContaining({ body: expect.stringContaining('<!-- openreview-summary -->') }),
    );
    expect(api.post).not.toHaveBeenCalled();
  });

  it('updates the existing summary without deleting it when newer comments exist', async () => {
    const { client, api } = makeMockClient();
    const poster = new CommentPoster(client);
    api.get.mockResolvedValue({
      data: [
        { id: 400, body: 'A newer conversation' },
        { id: 300, body: '<!-- openreview-summary -->\nolder summary' },
      ],
    });
    api.post.mockResolvedValue({ data: {} });
    api.delete.mockResolvedValue({ data: {} });

    await poster.postSummaryComment(7, makeSummary(2));

    expect(api.delete).not.toHaveBeenCalled();
    expect(api.post).not.toHaveBeenCalled();
    expect(api.patch).toHaveBeenCalledWith(
      '/repos/test-owner/test-repo/issues/comments/300',
      expect.objectContaining({ body: expect.stringContaining('<!-- openreview-summary -->') }),
    );
  });
});
