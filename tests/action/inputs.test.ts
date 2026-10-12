import { describe, expect, it } from 'vitest';

import { injectInputsToEnv } from '../../action/src/inputs.js';

describe('Action review inputs', () => {
  it('passes read-only context credentials and effort to the Go process', () => {
    const names = [
      'INPUT_GITHUB-TOKEN',
      'INPUT_EFFORT',
      'INPUT_INSTRUCTION',
      'GITHUB_TOKEN',
      'OPEN_REVIEW_EFFORT',
      'OPEN_REVIEW_INSTRUCTION',
    ];
    const old = names.map((name) => process.env[name]);
    try {
      process.env['INPUT_GITHUB-TOKEN'] = 'test-token';
      process.env.INPUT_EFFORT = 'high';
      process.env.INPUT_INSTRUCTION = 'Check authorization';
      injectInputsToEnv();
      expect(process.env.GITHUB_TOKEN).toBe('test-token');
      expect(process.env.OPEN_REVIEW_EFFORT).toBe('high');
      expect(process.env.OPEN_REVIEW_INSTRUCTION).toBe('Check authorization');
    } finally {
      names.forEach((name, index) => {
        if (old[index] === undefined) delete process.env[name];
        else process.env[name] = old[index];
      });
    }
  });
});
