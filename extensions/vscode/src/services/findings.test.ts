import assert from 'node:assert/strict';
import { test } from 'node:test';
import { findingPath } from './findings';

test('finding paths stay inside the reviewed root', () => {
  assert.equal(findingPath('/repo', 'src/main.ts'), '/repo/src/main.ts');
  assert.equal(findingPath('/repo', '../secret.txt'), undefined);
  assert.equal(findingPath('/repo', '/tmp/secret.txt'), undefined);
});
