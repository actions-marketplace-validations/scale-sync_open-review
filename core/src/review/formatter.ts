import { embedReviewState, formatFindingListItem, type ReviewState } from './review-state.js';
import type { FindingSeverity, ReviewFinding, ReviewSummary } from './types.js';

/* ------------------------------------------------------------------ */
/*  Severity badges                                                    */
/* ------------------------------------------------------------------ */

const SEVERITY_BADGES: Record<FindingSeverity, string> = {
  severe: '[CRITICAL] **Bug — Severe**',
  'non-severe': '[MODERATE] **Bug — Non-severe**',
  investigate: '[FLAG] **Investigate**',
  informational: '[INFO] **Informational**',
};

export const SUMMARY_MARKER = '<!-- openreview-summary -->';

/* ------------------------------------------------------------------ */
/*  Summary Comment                                                    */
/* ------------------------------------------------------------------ */

export function formatSummaryComment(summary: ReviewSummary, state?: ReviewState): string {
  const lines: string[] = [SUMMARY_MARKER];

  if (state) {
    lines.push(embedReviewState(state));
  }

  lines.push('## Code Review', '');

  const resolvedCount = summary.resolvedCount ?? summary.resolvedFindings?.length ?? 0;
  const openCount = summary.totalFindings;
  const statusBadge = summary.approved ? '✅ **Approved**' : '⚠️ **Findings**';
  const tally =
    resolvedCount > 0
      ? `**${resolvedCount} resolved** / **${openCount} open**`
      : `**${openCount} finding${openCount === 1 ? '' : 's'}**`;

  lines.push(`${statusBadge} · ${tally}`, '');

  if (summary.narrative) {
    lines.push(summary.narrative, '');
  }

  if (resolvedCount > 0 && summary.resolvedFindings?.length) {
    lines.push(
      '<details>',
      `<summary>✅ ${resolvedCount} resolved</summary>`,
      '',
      ...summary.resolvedFindings.map(formatFindingListItem),
      '',
      '</details>',
      '',
    );
  }

  const openFindings = summary.openFindings ?? [];
  if (openCount > 0 && openFindings.length > 0) {
    lines.push(
      '<details>',
      `<summary>${summary.approved ? 'ℹ️' : '🔎'} ${openCount} open</summary>`,
      '',
      ...openFindings.map(
        (f) =>
          `${formatFindingListItem(f)}${f.verification === 'uncertain' ? ` (needs checking: ${f.verificationReason ?? 'unanchored'})` : ''}`,
      ),
      '',
      '</details>',
      '',
    );
  } else if (openCount === 0) {
    lines.push('✅ No open findings.', '');
  }

  lines.push(`<sub>Files reviewed: ${summary.filesReviewed} · Duration: ${summary.duration}</sub>`);

  if (summary.finishingTouches) {
    lines.push('', summary.finishingTouches);
  }

  return lines.join('\n');
}

/* ------------------------------------------------------------------ */
/*  Inline Comment                                                     */
/* ------------------------------------------------------------------ */

export function formatInlineComment(finding: ReviewFinding): string {
  const badge = SEVERITY_BADGES[finding.severity];
  let body = `${badge}: ${finding.title}\n\n${finding.explanation}\n\n${finding.file}:${finding.startLine}${finding.endLine > finding.startLine ? `-${finding.endLine}` : ''}`;

  if (finding.suggestedFix) {
    const fence = finding.suggestedFix.includes('```') ? '````' : '```';
    body += `\n\n**Suggested fix:**\n${fence}${fence === '```' ? 'suggestion' : 'text'}\n${finding.suggestedFix}\n${fence}`;
  }

  if (finding.source === 'linter' || finding.source === 'both') {
    const linter = finding.linterName ?? 'linter';
    const label = finding.source === 'both' ? `AI + ${linter}` : linter;
    body += `\n\n> Source: ${label}`;
  }

  return body;
}
