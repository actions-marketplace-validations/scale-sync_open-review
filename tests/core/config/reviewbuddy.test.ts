import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import {
  DEFAULT_REVIEWBUDDY_CONFIG,
  buildReviewBuddyInstructions,
  loadReviewBuddyConfigFromDisk,
  parseReviewBuddyConfig,
} from '../../../core/src/config/reviewbuddy.js';

describe('parseReviewBuddyConfig', () => {
  it('returns defaults for empty YAML', () => {
    expect(parseReviewBuddyConfig('')).toEqual(DEFAULT_REVIEWBUDDY_CONFIG);
  });

  it('parses a fully specified config', () => {
    const cfg = parseReviewBuddyConfig(`
profile: chill
labeling_instructions: Use "flag" for anything not immediately actionable.
finishing_touches: "— reviewed by OpenReview"
path_instructions:
  - path: "apps/web/**"
    instructions: Favor accessibility findings.
`);

    expect(cfg.profile).toBe('chill');
    expect(cfg.labeling_instructions).toContain('flag');
    expect(cfg.finishing_touches).toBe('— reviewed by OpenReview');
    expect(cfg.path_instructions).toEqual([
      { path: 'apps/web/**', instructions: 'Favor accessibility findings.' },
    ]);
  });

  it('throws a readable error on invalid YAML', () => {
    expect(() => parseReviewBuddyConfig('profile: [unterminated')).toThrow(/Invalid \.reviewbuddy\.yaml/);
  });

  it('throws a readable error on schema violation', () => {
    expect(() => parseReviewBuddyConfig('profile: strict')).toThrow(/Invalid \.reviewbuddy\.yaml/);
  });
});

describe('buildReviewBuddyInstructions', () => {
  it('produces nothing for an all-default config', () => {
    expect(buildReviewBuddyInstructions(DEFAULT_REVIEWBUDDY_CONFIG)).toBe('');
  });

  it('injects a directive for the chill profile', () => {
    const block = buildReviewBuddyInstructions({ ...DEFAULT_REVIEWBUDDY_CONFIG, profile: 'chill' });
    expect(block).toContain('Review Profile: Chill');
  });

  it('lists path instructions', () => {
    const block = buildReviewBuddyInstructions({
      ...DEFAULT_REVIEWBUDDY_CONFIG,
      path_instructions: [{ path: 'packages/api/**', instructions: 'Check auth on every handler.' }],
    });
    expect(block).toContain('packages/api/**');
    expect(block).toContain('Check auth on every handler.');
  });

  it('includes labeling guidance', () => {
    const block = buildReviewBuddyInstructions({
      ...DEFAULT_REVIEWBUDDY_CONFIG,
      labeling_instructions: 'Treat lint-fixable style issues as informational.',
    });
    expect(block).toContain('Labeling Guidance');
    expect(block).toContain('lint-fixable');
  });
});

describe('loadReviewBuddyConfigFromDisk', () => {
  let tempDir: string;

  beforeEach(async () => {
    tempDir = await mkdtemp(join(tmpdir(), 'openreview-reviewbuddy-test-'));
  });

  afterEach(async () => {
    await rm(tempDir, { recursive: true, force: true });
  });

  it('returns defaults when no .reviewbuddy.yaml exists', async () => {
    expect(await loadReviewBuddyConfigFromDisk(tempDir)).toEqual(DEFAULT_REVIEWBUDDY_CONFIG);
  });

  it('reads and parses .reviewbuddy.yaml at the repo root', async () => {
    await writeFile(join(tempDir, '.reviewbuddy.yaml'), 'profile: chill\n');

    const cfg = await loadReviewBuddyConfigFromDisk(tempDir);

    expect(cfg.profile).toBe('chill');
  });
});
