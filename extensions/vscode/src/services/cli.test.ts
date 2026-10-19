import assert from 'node:assert/strict';
import { test } from 'node:test';
import { runCLI } from './cli';

test('CLI runner separates arguments and accepts partial results', async () => {
  const result = await runCLI(process.execPath, ['-e', 'process.stdout.write(JSON.stringify(process.argv.slice(1)));process.exit(2)', 'a b'], process.cwd());
  assert.equal(result.exitCode, 2);
  assert.equal(result.stdout, '["a b"]');
});

test('CLI runner cancels a pending process', async () => {
  const controller = new AbortController();
  const pending = runCLI(process.execPath, ['-e', 'setTimeout(()=>{},10000)'], process.cwd(), controller.signal);
  controller.abort();
  await assert.rejects(pending);
});
