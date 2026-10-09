import { describe, expect, it } from 'vitest';
import { formatMarkdown, formatText } from '../../cli/src/formatter.js';
import type { ReviewSummary } from '../../core/src/review/types.js';

const summary: ReviewSummary = {
  filesReviewed: 1,
  duration: '1s',
  mode: 'fast',
  findingsBySeverity: { severe: 0, 'non-severe': 0, investigate: 0, informational: 0 },
  totalFindings: 0,
};

describe('partial CLI output', () => {
  it('does not claim a clean review when the budget stops it', () => {
    expect(formatText([], summary, 'partial')).toContain('No validated findings before the limit.');
    expect(formatMarkdown([], summary, 'partial')).toContain('**Status:** partial');
  });
});
