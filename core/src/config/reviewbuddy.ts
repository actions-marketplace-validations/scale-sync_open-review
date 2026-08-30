import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';

import { parse as parseYaml } from 'yaml';
import { z } from 'zod';

/** Filename for the per-repo config, always read from the repo root. */
export const REVIEWBUDDY_CONFIG_FILE = '.reviewbuddy.yaml';

/* ------------------------------------------------------------------ */
/*  Schema                                                              */
/* ------------------------------------------------------------------ */

const PathInstructionSchema = z.object({
  path: z.string().describe('Glob-style path prefix these instructions apply to, e.g. "apps/web/**"'),
  instructions: z.string(),
});

export const ReviewBuddyConfigSchema = z.object({
  /** chill = only severe/non-severe bugs; assertive (default) = current thorough behavior. */
  profile: z.enum(['chill', 'assertive']).default('assertive'),
  path_instructions: z.array(PathInstructionSchema).default([]),
  labeling_instructions: z.string().default(''),
  /** Custom note appended to every review summary comment. */
  finishing_touches: z.string().default(''),
});

export type ReviewBuddyConfig = z.infer<typeof ReviewBuddyConfigSchema>;
export type PathInstruction = z.infer<typeof PathInstructionSchema>;

export const DEFAULT_REVIEWBUDDY_CONFIG: ReviewBuddyConfig = ReviewBuddyConfigSchema.parse({});

/* ------------------------------------------------------------------ */
/*  Parsing                                                             */
/* ------------------------------------------------------------------ */

/**
 * Parse and validate `.reviewbuddy.yaml` content.
 * Throws with a readable message on malformed YAML or a schema violation —
 * callers should decide whether to surface, log, or fall back to defaults.
 */
export function parseReviewBuddyConfig(yamlContent: string): ReviewBuddyConfig {
  let raw: unknown;
  try {
    raw = parseYaml(yamlContent);
  } catch (err) {
    throw new Error(
      `Invalid ${REVIEWBUDDY_CONFIG_FILE}: ${err instanceof Error ? err.message : String(err)}`,
      { cause: err },
    );
  }

  const result = ReviewBuddyConfigSchema.safeParse(raw ?? {});
  if (!result.success) {
    const issues = result.error.issues
      .map((i) => `${i.path.join('.') || '(root)'}: ${i.message}`)
      .join('; ');
    throw new Error(`Invalid ${REVIEWBUDDY_CONFIG_FILE}: ${issues}`);
  }

  return result.data;
}

/** Read and parse `.reviewbuddy.yaml` from a local checkout. Returns defaults if absent. */
export async function loadReviewBuddyConfigFromDisk(repoPath: string): Promise<ReviewBuddyConfig> {
  const filePath = resolve(repoPath, REVIEWBUDDY_CONFIG_FILE);

  let content: string;
  try {
    content = await readFile(filePath, 'utf-8');
  } catch {
    return DEFAULT_REVIEWBUDDY_CONFIG;
  }

  return parseReviewBuddyConfig(content);
}

/* ------------------------------------------------------------------ */
/*  Prompt integration                                                  */
/* ------------------------------------------------------------------ */

/**
 * Render a ReviewBuddy config as a block for the "## Project Review
 * Instructions" section of the review prompt (see fast-review.ts buildPrompt).
 *
 * ponytail: path_instructions are listed for the model rather than enforced
 * per diff-chunk — the prompt pipeline reviews chunks of the whole diff, not
 * one file at a time, so true per-path scoping would need a chunking rewrite.
 * Upgrade if a repo's path_instructions start conflicting across directories.
 */
export function buildReviewBuddyInstructions(cfg: ReviewBuddyConfig): string {
  const parts: string[] = [];

  if (cfg.profile === 'chill') {
    parts.push(
      '## Review Profile: Chill',
      'Only report "severe" and "non-severe" bugs. Skip "informational" and purely ' +
        'stylistic "investigate" flags unless they carry real risk.',
    );
  }

  if (cfg.path_instructions.length > 0) {
    parts.push(
      '',
      '## Path-Specific Instructions',
      ...cfg.path_instructions.map((p) => `- Files matching \`${p.path}\`: ${p.instructions}`),
    );
  }

  if (cfg.labeling_instructions) {
    parts.push('', '## Labeling Guidance', cfg.labeling_instructions);
  }

  return parts.join('\n').trim();
}
