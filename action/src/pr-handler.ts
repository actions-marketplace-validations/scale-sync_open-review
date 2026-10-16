import * as core from '@actions/core';
import type { Context } from '@actions/github/lib/context.js';
import { createHash } from 'node:crypto';
import {
  CommentPoster,
  GitHubClient,
  VERSION,
  buildFastReviewSummary,
  loadRepoConfigFromGitHub,
  runGoReview,
  toGoRequest,
} from '@openreview/core';
import type { PRContext } from '@openreview/core';
import { injectInputsToEnv } from './inputs.js';

/**
 * Handle pull_request events: opened, synchronize, reopened, ready_for_review.
 * Runs Fast mode review and posts findings as a batch GitHub review + summary comment.
 */
export async function handlePullRequest(context: Context): Promise<void> {
  const payload = context.payload;
  const pr = payload.pull_request;

  if (!pr) {
    core.warning('No pull_request in payload');
    return;
  }

  const owner = context.repo.owner;
  const repo = context.repo.repo;
  const prNumber = pr.number as number;
  const token = core.getInput('github-token', { required: true });

  core.info(`Processing PR #${prNumber}: ${pr.title}`);

  // Skip draft PRs unless configured
  const reviewDrafts = core.getInput('review-drafts') === 'true';
  if (pr.draft && !reviewDrafts) {
    core.info('Skipping draft PR (set review-drafts: true to enable)');
    return;
  }

  // Skip if PR description contains openreview: skip
  if (typeof pr.body === 'string' && pr.body.includes('openreview: skip')) {
    core.info('Skipping review: PR description contains "openreview: skip"');
    return;
  }

  // Set up env vars from action inputs
  injectInputsToEnv();

  const client = new GitHubClient({ token, owner, repo });
  const poster = new CommentPoster(client);

  try {
    const prMeta = await client.getPR(prNumber);
    const files = await client.getPRFiles(prNumber);
    const diff = await client.getPRDiff(prNumber);

    const rawMaxFiles = core.getInput('max-files') || '100';
    if (!/^\d+$/.test(rawMaxFiles)) throw new Error('max-files must be a positive integer');

    const { instructions, reviewBuddyConfig, ruleSources } = await loadRepoConfigFromGitHub(
      client,
      prMeta.head.sha,
      files.map((file) => file.filename),
    );

    const prContext: PRContext = {
      owner,
      repo,
      prNumber,
      metadata: {
        title: prMeta.title,
        body: prMeta.body,
        headSha: prMeta.head.sha,
        baseSha: prMeta.base.sha,
        author: prMeta.user.login,
      },
      diff,
      files: files.map((file) => file.filename),
      instructions,
      ruleSources,
      learnings: [],
    };

      prContext.reviewFingerprint = createHash('sha256').update(JSON.stringify({
        engineVersion: VERSION,
        request: toGoRequest(prContext, files),
        maxFiles: Number(rawMaxFiles),
        trustedLinters: process.env.OPEN_REVIEW_TRUSTED_LINTERS === 'true',
      })).digest('hex');
      const previous = await poster.fetchReviewState(prNumber);
      if (previous?.headSha === prMeta.head.sha && previous.inputFingerprint === prContext.reviewFingerprint) {
        core.info('Review already completed for this exact input');
        return;
      }

    const result = await runGoReview(prContext, files, Number(rawMaxFiles));
    const { findings } = result;
    const summary = result.summary ?? buildFastReviewSummary([], 0, 0);
    for (const entry of result.coverage) {
      core.info(
        `Review coverage: ${entry.state} ${entry.path}${entry.reason ? ` (${entry.reason})` : ''}`,
      );
    }
    for (const event of result.audit ?? []) {
      core.info(
        `Review audit: ${event.stage} ${event.status}${event.message ? ` (${event.message})` : ''}`,
      );
    }
    if (result.status === 'failed') throw new Error(result.error || 'Go review failed');
    if (result.status === 'partial') {
      await poster.postAcknowledgement(
        prNumber,
        '[WARN] **open-review** — Review stopped at its token or time limit. Partial findings were not posted.',
      );
      core.setFailed('Review was partial');
      return;
    }
    if (result.status === 'empty' || result.status === 'skipped') {
      await poster.postAcknowledgement(
        prNumber,
        '[INFO] **open-review** — No reviewable files in this PR. See the Action log for coverage.',
      );
      return;
    }
    if (reviewBuddyConfig.finishing_touches) {
      summary.finishingTouches = reviewBuddyConfig.finishing_touches;
    }

    const finalSummary = await poster.postReviewResults(prNumber, prContext, findings, summary);

    core.info(
      `Review complete: ${finalSummary.totalFindings} open, ${finalSummary.resolvedCount ?? 0} resolved (${finalSummary.duration})`,
    );
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    core.error(`Review failed: ${msg}`);

    // Post error comment so the user knows
    await poster.postAcknowledgement(prNumber, `[ERROR] **open-review** — Review failed: ${msg}`);

    throw err;
  }
}
