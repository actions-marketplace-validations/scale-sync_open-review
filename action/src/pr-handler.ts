import * as core from '@actions/core';
import type { Context } from '@actions/github/lib/context.js';
import {
  CommentPoster,
  GitHubClient,
  buildFastReviewSummary,
  loadRepoConfigFromGitHub,
  runGoReview,
} from '@openreview/core';
import type { PRContext } from '@openreview/core';

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

  // Post acknowledgement
  await poster.postAcknowledgement(
    prNumber,
    '[INFO] **open-review** — Review started... results will appear shortly.',
  );

  try {
    const prMeta = await client.getPR(prNumber);
    const files = await client.getPRFiles(prNumber);
    const diff = await client.getPRDiff(prNumber);

    const rawMaxFiles = core.getInput('max-files') || '100';
    if (!/^\d+$/.test(rawMaxFiles)) throw new Error('max-files must be a positive integer');

    const { instructions, reviewBuddyConfig } = await loadRepoConfigFromGitHub(
      client,
      prMeta.head.sha,
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
      learnings: [],
    };

    const result = await runGoReview(prContext, files, Number(rawMaxFiles));
    const { findings } = result;
    const summary = result.summary ?? buildFastReviewSummary([], 0, 0);
    for (const entry of result.coverage) {
      core.info(
        `Review coverage: ${entry.state} ${entry.path}${entry.reason ? ` (${entry.reason})` : ''}`,
      );
    }
    if (result.status === 'failed') throw new Error(result.error || 'Go review failed');
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

/**
 * Inject action inputs into process.env so @openreview/core config picks them up.
 */
function injectInputsToEnv(): void {
  const mappings: Array<[string, string]> = [
    ['openai-api-key', 'OPENAI_API_KEY'],
    ['anthropic-api-key', 'ANTHROPIC_API_KEY'],
    ['gemini-api-key', 'GEMINI_API_KEY'],
    ['main-model', 'MAIN_MODEL'],
    ['sub-model', 'SUB_MODEL'],
    ['max-files', 'MAX_FILES'],
    ['review-drafts', 'REVIEW_DRAFTS'],
  ];

  for (const [input, envVar] of mappings) {
    const value = core.getInput(input);
    if (value) {
      process.env[envVar] = value;
    }
  }
}
