import { MAX_CHUNK_CHARS, extractDiffForFiles, isReviewableFile } from '@openreview/core';

export interface ScopeEntry {
  file: string;
  state: 'selected' | 'partial' | 'skipped';
  reason?: 'unsupported_file' | 'missing_patch' | 'file_limit' | 'budget_truncated';
}

export function selectActionDiff(
  diff: string,
  files: string[],
  maxFiles: number,
): { diff: string; files: string[]; coverage: ScopeEntry[] } {
  if (!Number.isSafeInteger(maxFiles) || maxFiles < 1) {
    throw new Error('max-files must be a positive integer');
  }
  const selected: string[] = [];
  const patches: string[] = [];
  const coverage: ScopeEntry[] = [];
  for (const file of files) {
    if (!isReviewableFile(file)) {
      coverage.push({ file, state: 'skipped', reason: 'unsupported_file' });
      continue;
    }
    const patch = extractDiffForFiles(diff, [file]);
    if (!patch) {
      coverage.push({ file, state: 'skipped', reason: 'missing_patch' });
    } else if (selected.length >= maxFiles) {
      coverage.push({ file, state: 'skipped', reason: 'file_limit' });
    } else {
      selected.push(file);
      patches.push(patch);
      coverage.push(
        patch.length > MAX_CHUNK_CHARS
          ? { file, state: 'partial', reason: 'budget_truncated' }
          : { file, state: 'selected' },
      );
    }
  }
  return { diff: patches.join(''), files: selected, coverage };
}
