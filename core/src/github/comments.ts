import { createHash } from 'node:crypto';

import { formatInlineComment, formatSummaryComment } from '../review/formatter.js';
import {
  buildReviewState,
  enrichReviewSummary,
  parseReviewState,
  type ReviewState,
} from '../review/review-state.js';
import type { PRContext, ReviewFinding, ReviewSummary } from '../review/types.js';

import type { GitHubClient } from './client.js';

// Re-export types and formatters so existing consumers don't break
export type { ReviewFinding, ReviewSummary };
export type { FindingCategory, FindingSeverity, FindingSource, Citation } from '../review/types.js';
export { formatInlineComment, formatSummaryComment };
export { parseReviewState, enrichReviewSummary, buildReviewState };
export type { ReviewState };

export const REVIEW_SUMMARY_MARKER = '<!-- openreview-summary -->';

/* ------------------------------------------------------------------ */
/*  Comment Poster                                                     */
/* ------------------------------------------------------------------ */

export class CommentPoster {
  constructor(private client: GitHubClient) {}

  /**
   * Post all findings as a single batch review.
   * Review state is always COMMENT — never auto-approve or request changes.
   */
  async postReview(prNumber: number, findings: ReviewFinding[], headSha = ''): Promise<void> {
    if (findings.length === 0) return;

    const existing = await this.fetchInlineMarkers(prNumber);
    const comments = findings
      .filter((f) => !existing.has(this.inlineMarker(headSha, f)))
      .map((f) => ({
        path: f.file,
        line: f.startLine,
        side: 'RIGHT' as const,
        body: `${formatInlineComment(f)}\n\n${this.inlineMarker(headSha, f)}`,
      }));
    if (comments.length === 0) return;

    await this.client['api'].post(
      `/repos/${this.client.owner}/${this.client.repo}/pulls/${prNumber}/reviews`,
      {
        event: 'COMMENT',
        comments,
      },
    );
  }

  /**
   * Incremental review post: only new inline comments, summary updated with
   * resolved/open tracking across commits (gitar-bot style).
   */
  async postReviewResults(
    prNumber: number,
    pr: PRContext,
    findings: ReviewFinding[],
    baseSummary: ReviewSummary,
  ): Promise<ReviewSummary> {
    const previousState = await this.fetchReviewState(prNumber);
    const summary = enrichReviewSummary(pr, findings, baseSummary, previousState);
    const state = buildReviewState(pr, findings);

    const newFindings = summary.newFindings ?? findings;
    if (newFindings.length > 0) {
      await this.postReview(prNumber, newFindings, pr.metadata.headSha);
    }

    await this.postSummaryComment(prNumber, summary, state);
    return summary;
  }

  /**
   * Post or update the summary comment.
   * Uses replace-not-duplicate strategy via the HTML marker tag.
   */
  async postSummaryComment(
    prNumber: number,
    summary: ReviewSummary,
    state?: ReviewState,
  ): Promise<void> {
    const body = formatSummaryComment(summary, state);
    await this.postOrUpdateMarkedComment(prNumber, REVIEW_SUMMARY_MARKER, body);
  }

  /** Load persisted review state from the existing summary comment, if any. */
  async fetchReviewState(prNumber: number): Promise<ReviewState | null> {
    const body = await this.fetchMarkedCommentBody(prNumber, REVIEW_SUMMARY_MARKER);
    return body ? parseReviewState(body) : null;
  }

  /** Reply in a PR thread by posting a new issue comment. */
  async postChatReply(prNumber: number, reply: string): Promise<void> {
    await this.client['api'].post(
      `/repos/${this.client.owner}/${this.client.repo}/issues/${prNumber}/comments`,
      { body: reply },
    );
  }

  /** Post a standalone acknowledgement comment on the PR. */
  async postAcknowledgement(prNumber: number, message: string): Promise<void> {
    await this.client['api'].post(
      `/repos/${this.client.owner}/${this.client.repo}/issues/${prNumber}/comments`,
      { body: message },
    );
  }

  /* ---- Internal helpers ---- */

  private inlineMarker(headSha: string, finding: ReviewFinding): string {
    const key = [headSha, finding.file, finding.startLine, finding.title.toLowerCase().trim()].join(
      '\0',
    );
    return `<!-- openreview-finding:${createHash('sha256').update(key).digest('hex').slice(0, 24)} -->`;
  }

  private async fetchInlineMarkers(prNumber: number): Promise<Set<string>> {
    const markers = new Set<string>();
    // shortcut: scan the latest 1000 inline comments; increase if large PRs exceed this.
    for (let page = 1; page <= 10; page++) {
      const { data } = await this.client['api'].get(
        `/repos/${this.client.owner}/${this.client.repo}/pulls/${prNumber}/comments`,
        { params: { per_page: 100, page, sort: 'created', direction: 'desc' } },
      );
      for (const comment of data) {
        if (
          process.env.GITHUB_ACTIONS === 'true' &&
          comment.user?.login &&
          comment.user.login !== 'github-actions[bot]'
        )
          continue;
        const match =
          typeof comment.body === 'string'
            ? comment.body.match(/<!-- openreview-finding:[a-f0-9]{24} -->/)
            : null;
        if (match) markers.add(match[0]);
      }
      if (data.length < 100) break;
    }
    return markers;
  }

  private async postOrUpdateMarkedComment(
    prNumber: number,
    marker: string,
    body: string,
  ): Promise<void> {
    const marked = await this.findMarkedCommentRecord(prNumber, marker);

    if (marked) {
      await this.client['api'].patch(
        `/repos/${this.client.owner}/${this.client.repo}/issues/comments/${marked.id}`,
        { body },
      );
      return;
    }

    await this.client['api'].post(
      `/repos/${this.client.owner}/${this.client.repo}/issues/${prNumber}/comments`,
      { body },
    );
  }

  private async fetchMarkedCommentBody(prNumber: number, marker: string): Promise<string | null> {
    const comment = await this.findMarkedCommentRecord(prNumber, marker);
    return typeof comment?.body === 'string' ? comment.body : null;
  }

  /** Return the existing summary without disturbing later conversation. */
  private async findMarkedCommentRecord(
    prNumber: number,
    marker: string,
  ): Promise<{ id: number; body: string } | null> {
    let page = 1;
    const perPage = 100;

    // shortcut: scan the latest 1000 timeline comments; increase for longer PR threads.
    while (page <= 10) {
      const { data } = await this.client['api'].get(
        `/repos/${this.client.owner}/${this.client.repo}/issues/${prNumber}/comments`,
        { params: { per_page: perPage, page, sort: 'created', direction: 'desc' } },
      );

      for (const comment of data) {
        if (
          process.env.GITHUB_ACTIONS === 'true' &&
          comment.user?.login &&
          comment.user.login !== 'github-actions[bot]'
        )
          continue;
        if (typeof comment.body === 'string' && comment.body.includes(marker)) {
          return { id: comment.id, body: comment.body };
        }
      }

      if (data.length < perPage) break;
      page++;
    }

    return null;
  }
}
