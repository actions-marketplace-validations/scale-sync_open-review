import { basename } from 'node:path';

import type { GitHubClient } from '../github/client.js';

import {
  INSTRUCTION_FILE_NAMES,
  buildInstructionResult,
  getDepth,
  sortInstructionFiles,
} from './instructions.js';
import {
  DEFAULT_REVIEWBUDDY_CONFIG,
  REVIEWBUDDY_CONFIG_FILE,
  buildReviewBuddyInstructions,
  parseReviewBuddyConfig,
  type ReviewBuddyConfig,
} from './reviewbuddy.js';

export interface RemoteRepoConfig {
  /** Combined instructions block, ready to drop into PRContext.instructions. */
  instructions: string;
  reviewBuddyConfig: ReviewBuddyConfig;
}

/**
 * Fetch REVIEW.md/AGENTS.md/CLAUDE.md/.cursorrules/.windsurfrules and
 * .reviewbuddy.yaml straight from the GitHub API at `ref` — no local
 * checkout required, so this works from the CLI (which never clones) and
 * from the Action/comment handlers alike.
 */
export async function loadRepoConfigFromGitHub(
  client: GitHubClient,
  ref: string,
): Promise<RemoteRepoConfig> {
  let tree: string[];
  try {
    tree = await client.getFileTree(ref);
  } catch {
    return { instructions: '', reviewBuddyConfig: DEFAULT_REVIEWBUDDY_CONFIG };
  }

  const instructionPaths = tree.filter((p) => INSTRUCTION_FILE_NAMES.has(basename(p)));
  const reviewBuddyPath = tree.find((p) => p === REVIEWBUDDY_CONFIG_FILE);

  const fetched = await Promise.all(
    instructionPaths.map(async (relativePath) => {
      try {
        const content = await client.getFileContent(relativePath, ref);
        return { relativePath, depth: getDepth(relativePath), content };
      } catch {
        return null;
      }
    }),
  );

  const instructionFiles = sortInstructionFiles(
    fetched.filter((f): f is { relativePath: string; depth: number; content: string } => f !== null),
  );
  const { globalInstructions, scopedInstructions } = buildInstructionResult(instructionFiles);

  let reviewBuddyConfig = DEFAULT_REVIEWBUDDY_CONFIG;
  if (reviewBuddyPath) {
    try {
      const content = await client.getFileContent(reviewBuddyPath, ref);
      reviewBuddyConfig = parseReviewBuddyConfig(content);
    } catch {
      // Invalid or unreadable — fall back to defaults rather than failing the review.
    }
  }

  const parts: string[] = [];
  if (globalInstructions) parts.push('## Project Review Instructions', globalInstructions);

  if (scopedInstructions.size > 0) {
    parts.push('', '## Directory-Specific Instructions');
    for (const [dir, content] of scopedInstructions) {
      parts.push(`### ${dir}`, content);
    }
  }

  const reviewBuddyBlock = buildReviewBuddyInstructions(reviewBuddyConfig);
  if (reviewBuddyBlock) parts.push('', reviewBuddyBlock);

  return {
    instructions: parts.join('\n').trim(),
    reviewBuddyConfig,
  };
}
