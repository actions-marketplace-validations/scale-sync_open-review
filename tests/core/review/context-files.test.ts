import { describe, expect, it } from 'vitest';

import { isContextCandidate } from '../../../core/src/review/reviewable-files.js';

describe('conversation file candidates', () => {
  it('allows source and workflow files while excluding hidden secrets and build files', () => {
    expect(isContextCandidate('src/login.ts')).toBe(true);
    expect(isContextCandidate('.github/workflows/ci.yml')).toBe(true);
    expect(isContextCandidate('src/.env.local')).toBe(false);
    expect(isContextCandidate('node_modules/pkg/index.ts')).toBe(false);
    expect(isContextCandidate('keys/private.pem')).toBe(false);
    expect(isContextCandidate('../outside.ts')).toBe(false);
  });
});
