import * as core from '@actions/core';
import type { Context } from '@actions/github/lib/context.js';
import {
  CommentPoster,
  GitHubClient,
  LearningsStore,
  SnapshotBuilder,
  buildFastReviewSummary,
  containsTrigger,
  handleChatMention,
  loadRepoConfigFromGitHub,
  parseDiff,
  runGoReview,
} from '@openreview/core';
import type { PRContext } from '@openreview/core';
import { injectInputsToEnv } from './inputs.js';

/**
 * Handle pull_request_review_comment and issue_comment events.
 * Routes to: RLM deep review, chat handler, or learnings management.
 */
export async function handleComment(context: Context): Promise<void> {
  const payload = context.payload;
  const comment = payload.comment;

  if (!comment || !comment.body) {
    core.warning('No comment body in payload');
    return;
  }

  const owner = context.repo.owner;
  const repo = context.repo.repo;
  const prNumber = (payload.pull_request?.number ?? payload.issue?.number) as number;
  const commentBody = comment.body as string;
  const commentAuthor = (comment.user?.login ?? '') as string;
  const commentId = comment.id as number;
  const token = core.getInput('github-token', { required: true });
  const repoSlug = `${owner}/${repo}`;

  // Loop prevention: skip if the comment is from the bot itself
  if (commentAuthor === 'github-actions[bot]' || commentAuthor.includes('[bot]')) {
    core.debug('Skipping bot comment (loop prevention)');
    return;
  }

  // Only process product mentions
  if (!/@(?:open-review|openreview)\b/i.test(commentBody)) {
    // Check for learnings trigger phrases
    if (containsTrigger(commentBody)) {
      core.info('Detected learning trigger phrase');
      const store = new LearningsStore(repoSlug);
      await store.add(commentBody, commentBody);

      const client = new GitHubClient({ token, owner, repo });
      const poster = new CommentPoster(client);
      await poster.postChatReply(prNumber, "Got it! I'll remember this for future reviews.");
      return;
    }
    return;
  }

  // Set up env vars from action inputs
  injectInputsToEnv();

  const client = new GitHubClient({ token, owner, repo });
  const poster = new CommentPoster(client);

  // Extract the command/question after @openreview
  const mentionText = commentBody.replace(/@(?:open-review|openreview)\s*/i, '').trim();

  // Route: @openreview rlm → deep review
  if (mentionText.toLowerCase() === 'rlm') {
    core.info('Triggering RLM deep review');
    await poster.postAcknowledgement(
      prNumber,
      '[INFO] **open-review** — Deep review started (RLM mode)... this may take 2-5 minutes.',
    );

    try {
      const prMeta = await client.getPR(prNumber);
      const files = await client.getPRFiles(prNumber);
      const diff = await client.getPRDiff(prNumber);
      const { instructions, ruleSources } = await loadRepoConfigFromGitHub(
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
        files: files.map((f) => f.filename),
        instructions,
        ruleSources,
        learnings: [],
      };

      const result = await runGoReview(prContext, files, 100, 'deep');
      for (const event of result.audit ?? []) {
        core.info(`[Deep] ${event.stage}: ${event.status}`);
      }
      if (result.status === 'failed') throw new Error(result.error || 'Deep review failed');
      if (result.status === 'partial') {
        await poster.postAcknowledgement(
          prNumber,
          '[WARN] **open-review** — Deep review stopped at its token or time limit. Partial findings were not posted.',
        );
        core.setFailed('Deep review was partial');
        return;
      }
      if (result.status === 'empty' || result.status === 'skipped') {
        await poster.postAcknowledgement(prNumber, '[INFO] **open-review** — No reviewable files.');
        return;
      }
      const findings = result.findings;
      await poster.postReviewResults(
        prNumber,
        prContext,
        findings,
        result.summary ?? buildFastReviewSummary([], 0, 0),
      );

      await poster.postAcknowledgement(
        prNumber,
        `[SUCCESS] **open-review** — RLM deep review complete. Found ${findings.length} issue(s).`,
      );
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      core.error(`RLM review failed: ${msg}`);
      await poster.postAcknowledgement(
        prNumber,
        `[ERROR] **open-review** — RLM review failed: ${msg}`,
      );
    }
    return;
  }

  // Route: @openreview review → fresh fast review
  if (mentionText.toLowerCase() === 'review') {
    core.info('Triggering fresh fast review via comment');
    await poster.postAcknowledgement(prNumber, '[INFO] **open-review** — Fresh review started...');
    try {
      const prMeta = await client.getPR(prNumber);
      const files = await client.getPRFiles(prNumber);
      const diff = await client.getPRDiff(prNumber);

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
        files: files.map((f) => f.filename),
        instructions,
        ruleSources,
        learnings: [],
      };

      const result = await runGoReview(prContext, files);
      for (const event of result.audit ?? []) {
        core.info(`Review audit: ${event.stage} ${event.status}`);
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
          '[INFO] **open-review** — No reviewable files in this PR.',
        );
        return;
      }
      const { findings } = result;
      const summary = result.summary ?? buildFastReviewSummary([], 0, 0);
      if (reviewBuddyConfig.finishing_touches) {
        summary.finishingTouches = reviewBuddyConfig.finishing_touches;
      }

      await poster.postReviewResults(prNumber, prContext, findings, summary);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      await poster.postAcknowledgement(prNumber, `[ERROR] **open-review** — Review failed: ${msg}`);
    }
    return;
  }

  // Route: @openreview list learnings
  if (mentionText.toLowerCase() === 'list learnings') {
    core.info('Listing learnings');
    const store = new LearningsStore(repoSlug);
    const learnings = await store.list();

    if (learnings.length === 0) {
      await poster.postChatReply(prNumber, 'No learnings stored for this repository.');
    } else {
      const list = learnings
        .map((l, i) => `${i + 1}. ${l.finding} (used ${l.usedCount}x)`)
        .join('\n');
      await poster.postChatReply(prNumber, `**Team Learnings (${learnings.length}):**\n\n${list}`);
    }
    return;
  }

  // Route: @openreview forget: <description>
  if (mentionText.toLowerCase().startsWith('forget:')) {
    const desc = mentionText.slice(7).trim();
    core.info(`Deleting learning: ${desc}`);
    const store = new LearningsStore(repoSlug);
    const learnings = await store.list();
    const match = learnings.find((l) => l.finding.toLowerCase().includes(desc.toLowerCase()));

    if (match) {
      await store.delete(match.id);
      await poster.postChatReply(prNumber, `[SUCCESS] Forgot: "${match.finding}"`);
    } else {
      await poster.postChatReply(prNumber, `No matching learning found for: "${desc}"`);
    }
    return;
  }

  // Route: @openreview <question> → chat handler
  core.info(`Handling chat question: ${mentionText.slice(0, 80)}`);
  try {
    const prMeta = await client.getPR(prNumber);
    const files = await client.getPRFiles(prNumber);
    const diff = await client.getPRDiff(prNumber);
    const parsedDiffs = parseDiff(diff);

    const snapshot = new SnapshotBuilder({
      client,
      headRef: prMeta.head.sha,
      diffs: parsedDiffs,
    });

    const { instructions } = await loadRepoConfigFromGitHub(client, prMeta.head.sha);

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
      files: files.map((f) => f.filename),
      instructions,
      learnings: [],
    };

    const chatEvent = {
      commentId,
      body: mentionText,
      user: commentAuthor,
      prNumber,
    };

    const chatContext = {
      pr: prContext,
      client,
      snapshot,
      botUsername: 'github-actions[bot]',
    };

    await handleChatMention(chatEvent, chatContext);
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    core.error(`Chat handler failed: ${msg}`);
    await poster.postChatReply(prNumber, `[ERROR] **open-review** — Failed to answer: ${msg}`);
  }
}
