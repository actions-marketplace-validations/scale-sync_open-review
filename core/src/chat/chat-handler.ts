import type { GitHubClient } from '../github/client.js';
import { CommentPoster } from '../github/comments.js';
import { createMainLLM, streamChat } from '../llm/router.js';
import { extractDiffForFiles } from '../review/diff-fingerprint.js';
import { isContextCandidate } from '../review/reviewable-files.js';
import type { SnapshotBuilder } from '../review/snapshot.js';
import type { PRContext } from '../review/types.js';

import { generateSuggestions } from './suggestions.js';

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

export interface CommentEvent {
  commentId: number;
  body: string;
  user: string;
  prNumber: number;
}

export interface ChatContext {
  pr: PRContext;
  client: GitHubClient;
  snapshot: SnapshotBuilder;
  botUsername?: string;
}

/* ------------------------------------------------------------------ */
/*  Chat handler                                                       */
/* ------------------------------------------------------------------ */

const MENTION_RE = /@(?:open-review|openreview)\s*/i;
const BOT_DEFAULT = 'open-review[bot]';

/**
 * Handle an @openreview mention in a PR comment.
 * Extracts the question, builds context, calls LLM, validates citations,
 * and posts the answer as a reply comment with follow-up suggestions.
 */
export async function handleChatMention(event: CommentEvent, ctx: ChatContext): Promise<void> {
  // Detect bot's own comments to prevent reply loops
  const botName = ctx.botUsername ?? BOT_DEFAULT;
  if (
    event.user.toLowerCase() === botName.toLowerCase() ||
    event.user.toLowerCase().endsWith('[bot]')
  ) {
    return;
  }

  // Extract question from comment text
  const question = extractQuestion(event.body);
  if (!question) return;

  // Load conversation thread history
  const threadHistory = await loadThreadHistory(ctx.client, event.prNumber);

  // Build chat context with snapshot tools
  const messages = await buildChatMessages(question, threadHistory, ctx.pr, ctx.snapshot);

  // Call LLM and collect response
  const llm = createMainLLM();
  let answer = '';
  for await (const chunk of streamChat(llm, messages)) {
    answer += chunk;
  }

  // Don't post empty replies
  if (!answer.trim()) return;

  // Validate citations in the response
  answer = await validateAnswerCitations(answer, ctx.snapshot);

  // Generate follow-up suggestions
  const suggestions = await generateSuggestions(answer, ctx.pr);

  // Append suggestions as blockquote list
  if (suggestions.length > 0) {
    answer += '\n\n---\n**Suggested follow-ups:**\n';
    answer += suggestions.map((s) => `> - ${s}`).join('\n');
  }

  // Post reply as a new comment on the PR
  const poster = new CommentPoster(ctx.client);
  await poster.postChatReply(event.prNumber, answer);
}

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function extractQuestion(body: string): string | null {
  const stripped = body.replace(MENTION_RE, '').trim();
  return stripped.length > 0 ? stripped : null;
}

async function loadThreadHistory(
  client: GitHubClient,
  prNumber: number,
): Promise<Array<{ user: string; body: string }>> {
  try {
    const comments = await client.getIssueComments(prNumber);
    return comments.map((c) => ({
      user: c.user.login,
      body: c.body,
    }));
  } catch {
    return [];
  }
}

async function buildChatMessages(
  question: string,
  threadHistory: Array<{ user: string; body: string }>,
  pr: PRContext,
  snapshot: SnapshotBuilder,
): Promise<Array<{ role: string; content: string }>> {
  const safeFiles = pr.files.filter(isContextCandidate);
  const snippets: string[] = [];
  let remaining = 20_000;
  for (const file of safeFiles.slice(0, 8)) {
    if (remaining <= 0) break;
    const content = (await snapshot.getFile(file)).slice(0, Math.min(remaining, 4_000));
    if (!content) continue;
    const numbered = content
      .split('\n')
      .map((line, index) => String(index + 1) + ': ' + line)
      .join('\n');
    snippets.push(file + '\n' + numbered);
    remaining -= content.length;
  }
  const systemParts = [
    'You are open-review, a codebase-aware AI assistant for code review.',
    'You answer questions about the PR and the codebase.',
    '',
    'Use only the bounded diff and file excerpts below. Cite code as path:line.',
    '',
    `Repository: ${pr.owner}/${pr.repo}`,
    `PR #${pr.prNumber}: ${pr.metadata.title}`,
    `Author: ${pr.metadata.author}`,
    '',
    'Changed files: ' + safeFiles.slice(0, 100).join(', '),
  ];

  if (pr.instructions) {
    systemParts.push('', '## Project Instructions', pr.instructions);
  }

  const messages: Array<{ role: string; content: string }> = [
    { role: 'system', content: systemParts.join('\n') },
  ];

  // Include recent thread history for context
  const recentHistory = threadHistory.slice(-10);
  for (const entry of recentHistory) {
    const role = /open-?review/i.test(entry.user) ? 'ai' : 'human';
    messages.push({ role, content: entry.body.slice(0, 2_000) });
  }

  // Add the current question with diff context
  messages.push({
    role: 'human',
    content:
      'Question: ' +
      question.slice(0, 2_000) +
      '\n\nDiff:\n' +
      extractDiffForFiles(pr.diff, safeFiles).slice(0, 20_000) +
      '\n\nFile excerpts:\n' +
      snippets.join('\n\n'),
  });

  return messages;
}

async function validateAnswerCitations(answer: string, snapshot: SnapshotBuilder): Promise<string> {
  const references = [
    ...answer.matchAll(/\x60([a-zA-Z0-9_/.@-]+\.[a-zA-Z0-9]+)(?::(\d+))?\x60/g),
  ].slice(0, 20);
  const fileSet = new Set(await snapshot.listFiles());
  for (const match of references) {
    const file = match[1];
    const line = match[2] ? Number(match[2]) : 0;
    const content = fileSet.has(file) ? await snapshot.getFile(file) : '';
    const validLine = line === 0 || (line > 0 && line <= content.split('\n').length);
    if (!content || !validLine) {
      answer = answer.replaceAll(match[0], match[0] + ' *(citation not verified)*');
    }
  }
  return answer;
}
