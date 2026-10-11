import { randomUUID } from 'node:crypto';
import { rename, unlink, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';

import {
  CommentPoster,
  GitHubClient,
  buildFastReviewSummary,
  loadRepoConfigFromGitHub,
  runGoReview,
} from '@openreview/core';
import type { PRContext, ReviewFinding, ReviewSummary } from '@openreview/core';
import type { Command } from 'commander';

import { formatMarkdown, formatText } from '../formatter.js';

function positive(value: string | undefined, name: string): number | undefined {
  if (value === undefined) return undefined;
  const parsed = Number(value);
  if (!Number.isSafeInteger(parsed) || parsed < 1)
    throw new Error(`${name} must be a positive integer`);
  return parsed;
}

async function writeResult(path: string | undefined, output: string): Promise<void> {
  if (!path || path === '-') {
    process.stdout.write(output);
    return;
  }
  const temp = join(dirname(path), `.open-review-${randomUUID()}`);
  try {
    await writeFile(temp, output, { mode: 0o600, flag: 'wx' });
    await rename(temp, path);
  } catch (error) {
    await unlink(temp).catch(() => undefined);
    throw error;
  }
}

export function registerReviewCommand(program: Command): void {
  program
    .command('review')
    .description('Review a GitHub Pull Request')
    .requiredOption('--url <url>', 'GitHub PR URL')
    .option('--mode <mode>', 'Review mode: fast or rlm', 'fast')
    .option('--output <format>', 'Output format: text, markdown, or json', 'text')
    .option('--model <model>', 'Override LLM model')
    .option('--provider <provider>', 'Override model provider')
    .option('--base-url <url>', 'Override provider endpoint')
    .option('--prompt-tokens <n>', 'Maximum estimated prompt tokens per request')
    .option('--output-tokens <n>', 'Maximum output tokens per request')
    .option('--total-tokens <n>', 'Aggregate token budget')
    .option('--request-seconds <n>', 'Per-request timeout')
    .option('--group-seconds <n>', 'Review deadline')
    .option('--effort <level>', 'Review rounds: low, medium, or high', 'medium')
    .option('--output-file <path>', 'Write output atomically to a file')
    .option('--expert', 'Comprehensive SOLID/security/quality review')
    .option('--submit', 'Post findings as GitHub PR comment')
    .option('--quiet', 'Suppress progress output')
    .action(async (opts) => {
      try {
        if (opts.mode !== 'fast' && opts.mode !== 'rlm')
          throw new Error('mode must be fast or rlm');
        if (!['text', 'markdown', 'json'].includes(opts.output))
          throw new Error('output must be text, markdown, or json');
        if (!['low', 'medium', 'high'].includes(opts.effort))
          throw new Error('effort must be low, medium, or high');
        const modelOptions = {
          effort: opts.effort as 'low' | 'medium' | 'high',
          provider: opts.provider as string | undefined,
          model: opts.model as string | undefined,
          baseUrl: opts.baseUrl as string | undefined,
          promptTokens: positive(opts.promptTokens, 'prompt-tokens'),
          outputTokens: positive(opts.outputTokens, 'output-tokens'),
          totalTokens: positive(opts.totalTokens, 'total-tokens'),
          requestSeconds: positive(opts.requestSeconds, 'request-seconds'),
          groupSeconds: positive(opts.groupSeconds, 'group-seconds'),
        };

        const { client, prNumber } = GitHubClient.fromPRUrl(opts.url);

        if (!opts.quiet) {
          process.stderr.write(`Fetching PR #${prNumber}...\n`);
        }

        const prMeta = await client.getPR(prNumber);
        const files = await client.getPRFiles(prNumber);
        const diff = await client.getPRDiff(prNumber);

        const expertInstructions = opts.expert
          ? [
              'Perform a comprehensive expert review covering:',
              '- SOLID principles (single responsibility, open-closed, Liskov substitution, interface segregation, dependency inversion)',
              '- Security: OWASP Top 10, injection flaws, auth/authz issues, data exposure',
              '- Code quality: complexity, readability, maintainability, test coverage gaps',
              '- Performance: algorithmic complexity, resource leaks, unnecessary allocations',
            ].join('\n')
          : '';

        const { instructions: repoInstructions, reviewBuddyConfig } =
          await loadRepoConfigFromGitHub(client, prMeta.head.sha);

        const prContext: PRContext = {
          owner: client.owner,
          repo: client.repo,
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
          instructions: [repoInstructions, expertInstructions].filter(Boolean).join('\n\n'),
          learnings: [],
        };

        if (!opts.quiet) process.stderr.write(`Running ${opts.mode} review...\n`);
        const result = await runGoReview(
          prContext,
          files,
          100,
          opts.mode === 'rlm' ? 'deep' : 'fast',
          modelOptions,
        );
        if (result.status === 'failed') throw new Error(result.error || 'Go review failed');
        if (!opts.quiet)
          for (const event of result.audit ?? [])
            process.stderr.write(`  [${event.stage}] ${event.status}\n`);
        const noReview = result.status === 'empty' || result.status === 'skipped';
        const findings: ReviewFinding[] = result.findings;
        const summary: ReviewSummary = result.summary ?? buildFastReviewSummary([], 0, 0);

        if (reviewBuddyConfig.finishing_touches) {
          summary.finishingTouches = reviewBuddyConfig.finishing_touches;
        }

        // Format and print output
        const format = opts.output as string;
        let output: string;
        if (format === 'json') {
          output = JSON.stringify(
            {
              status: result.status,
              summary,
              findings,
              coverage: result.coverage,
              usage: result.usage ?? null,
            },
            null,
            2,
          );
        } else if (format === 'markdown') {
          output = formatMarkdown(findings, summary, result.status);
        } else {
          output = formatText(findings, summary, result.status);
        }

        await writeResult(opts.outputFile as string | undefined, output + '\n');

        // Post to GitHub if --submit flag is set
        if (opts.submit && noReview && !opts.quiet) {
          process.stderr.write('No reviewable files; nothing posted.\n');
        }
        if (opts.submit && !noReview && result.status === 'complete') {
          if (!opts.quiet) {
            process.stderr.write('Posting review to GitHub...\n');
          }

          const poster = new CommentPoster(client);
          const finalSummary = await poster.postReviewResults(
            prNumber,
            prContext,
            findings,
            summary,
          );

          if (!opts.quiet)
            process.stderr.write(
              `Review posted on PR #${prNumber} (${finalSummary.totalFindings} open, ${finalSummary.resolvedCount ?? 0} resolved)\n`,
            );
        }
        if (result.status === 'partial') {
          process.exitCode = 2;
          if (!opts.quiet) process.stderr.write('Review is partial; findings were not posted.\n');
        }
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        process.stderr.write(`Error: ${msg}\n`);
        process.exit(1);
      }
    });
}
