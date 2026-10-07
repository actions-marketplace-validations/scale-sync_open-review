import { execFileSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { afterEach, describe, expect, it } from 'vitest';

import {
  loadLocalAskContext,
  markUnverifiedCitations,
} from '../../cli/src/commands/ask-context.js';

const folders: string[] = [];
afterEach(() => {
  for (const folder of folders.splice(0)) rmSync(folder, { recursive: true, force: true });
});

describe('local ask context', () => {
  it('reads bounded tracked source and checks line citations', async () => {
    const root = mkdtempSync(join(tmpdir(), 'open-review-ask-'));
    folders.push(root);
    execFileSync('git', ['init', '-q'], { cwd: root });
    mkdirSync(join(root, 'src'));
    writeFileSync(join(root, 'src/login.ts'), 'export const login = true;\n');
    writeFileSync(join(root, '.env'), 'TOKEN=secret');
    execFileSync('git', ['add', '.'], { cwd: root });

    const context = await loadLocalAskContext(root, 'login');
    expect(context.prompt).toContain('src/login.ts');
    expect(context.prompt).toContain('1: export const login');
    expect(context.prompt).not.toContain('TOKEN=secret');
    expect(
      markUnverifiedCitations(
        'See ' + String.fromCharCode(96) + 'src/login.ts:1' + String.fromCharCode(96),
        context.lineCounts,
      ),
    ).not.toContain('not verified');
    expect(
      markUnverifiedCitations(
        'See ' + String.fromCharCode(96) + 'src/login.ts:99' + String.fromCharCode(96),
        context.lineCounts,
      ),
    ).toContain('citation not verified');
  });
});
