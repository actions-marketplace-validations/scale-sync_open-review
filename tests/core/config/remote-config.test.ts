import { describe, expect, it, vi } from 'vitest';

import { loadRepoConfigFromGitHub } from '../../../core/src/config/remote-config.js';
import type { GitHubClient } from '../../../core/src/github/client.js';

function makeMockClient(tree: string[], files: Record<string, string>) {
  const getFileTree = vi.fn().mockResolvedValue(tree);
  const getFileContent = vi.fn((path: string) => {
    if (path in files) return Promise.resolve(files[path]);
    return Promise.reject(new Error(`404: ${path}`));
  });

  const client = { getFileTree, getFileContent } as unknown as GitHubClient;
  return { client, getFileTree, getFileContent };
}

describe('loadRepoConfigFromGitHub', () => {
  it('returns empty defaults when the repo has no config files', async () => {
    const { client } = makeMockClient(['src/index.ts', 'package.json'], {});

    const result = await loadRepoConfigFromGitHub(client, 'main');

    expect(result.instructions).toBe('');
    expect(result.reviewBuddyConfig.profile).toBe('assertive');
  });

  it('falls back to empty defaults when getFileTree fails', async () => {
    const client = {
      getFileTree: vi.fn().mockRejectedValue(new Error('404')),
      getFileContent: vi.fn(),
    } as unknown as GitHubClient;

    const result = await loadRepoConfigFromGitHub(client, 'main');

    expect(result.instructions).toBe('');
    expect(result.reviewBuddyConfig.profile).toBe('assertive');
  });

  it('merges root instructions, scoped instructions, and reviewbuddy config', async () => {
    const { client } = makeMockClient(
      ['REVIEW.md', 'apps/web/AGENTS.md', '.reviewbuddy.yaml', 'src/index.ts'],
      {
        'REVIEW.md': 'Global review rules.',
        'apps/web/AGENTS.md': 'Web-specific rules.',
        '.reviewbuddy.yaml': 'profile: chill\nlabeling_instructions: Be terse.\n',
      },
    );

    const result = await loadRepoConfigFromGitHub(client, 'abc123');

    expect(result.instructions).toContain('Global review rules.');
    expect(result.instructions).toContain('apps/web');
    expect(result.instructions).toContain('Web-specific rules.');
    expect(result.instructions).toContain('Review Profile: Chill');
    expect(result.instructions).toContain('Be terse.');
    expect(result.reviewBuddyConfig.profile).toBe('chill');
  });

  it('falls back to default reviewbuddy config when the YAML is invalid', async () => {
    const { client } = makeMockClient(['.reviewbuddy.yaml'], {
      '.reviewbuddy.yaml': 'profile: not-a-real-profile',
    });

    const result = await loadRepoConfigFromGitHub(client, 'main');

    expect(result.reviewBuddyConfig.profile).toBe('assertive');
  });

  it('skips an instruction file that fails to fetch', async () => {
    const { client } = makeMockClient(['REVIEW.md', 'CLAUDE.md'], {
      'REVIEW.md': 'Only this one is fetchable.',
    });

    const result = await loadRepoConfigFromGitHub(client, 'main');

    expect(result.instructions).toContain('Only this one is fetchable.');
  });
});
