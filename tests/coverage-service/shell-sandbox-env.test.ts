import { EventEmitter } from 'node:events';

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const spawnMock = vi.fn();

vi.mock('child_process', () => ({
  spawn: spawnMock,
}));

function fakeProcess() {
  const proc = new EventEmitter() as EventEmitter & {
    stdout: EventEmitter;
    stderr: EventEmitter;
  };
  proc.stdout = new EventEmitter();
  proc.stderr = new EventEmitter();
  return proc;
}

describe('runCommand — sandboxed environment', () => {
  const secretVars = {
    OPENAI_API_KEY: 'sk-should-not-leak',
    ANTHROPIC_API_KEY: 'sk-ant-should-not-leak',
    GITHUB_APP_PRIVATE_KEY: '-----BEGIN RSA PRIVATE KEY-----should-not-leak',
    WEBHOOK_SECRET: 'super-secret',
    DATABASE_URL: 'postgres://user:pass@host/db',
  };
  const originalValues: Record<string, string | undefined> = {};

  beforeEach(() => {
    spawnMock.mockReset();
    for (const [key, value] of Object.entries(secretVars)) {
      originalValues[key] = process.env[key];
      process.env[key] = value;
    }
  });

  afterEach(() => {
    for (const key of Object.keys(secretVars)) {
      if (originalValues[key] === undefined) delete process.env[key];
      else process.env[key] = originalValues[key];
    }
  });

  it('never passes secret-shaped env vars to the spawned shell running untrusted PR code', async () => {
    let capturedEnv: NodeJS.ProcessEnv | undefined;
    spawnMock.mockImplementation((_cmd: string, _args: string[], opts: { env: NodeJS.ProcessEnv }) => {
      capturedEnv = opts.env;
      const proc = fakeProcess();
      queueMicrotask(() => proc.emit('close', 0));
      return proc;
    });

    const { runCommand } = await import('../../coverage-service/worker/src/lib/shell.js');
    const result = await runCommand('npm test', '/tmp/some-pr-checkout');

    expect(result.exitCode).toBe(0);
    expect(capturedEnv).toBeDefined();
    for (const key of Object.keys(secretVars)) {
      expect(capturedEnv![key]).toBeUndefined();
    }
    // Still needs the basics to actually run npm/node/git.
    expect(capturedEnv!.PATH).toBe(process.env.PATH);
  });
});
