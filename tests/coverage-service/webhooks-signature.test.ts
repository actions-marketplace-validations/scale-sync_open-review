import { createHmac } from 'node:crypto';

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { WebhooksController } from '../../coverage-service/api/src/webhooks/webhooks.controller';
import type { WebhooksService } from '../../coverage-service/api/src/webhooks/webhooks.service';

function sign(body: Buffer, secret: string): string {
  return 'sha256=' + createHmac('sha256', secret).update(body).digest('hex');
}

function makeReq(rawBody: Buffer | undefined) {
  return { rawBody } as never;
}

describe('WebhooksController — GitHub signature verification', () => {
  const originalSecret = process.env.WEBHOOK_SECRET;
  let service: { handlePullRequest: ReturnType<typeof vi.fn>; handleInstallation: ReturnType<typeof vi.fn> };
  let controller: WebhooksController;

  beforeEach(() => {
    service = {
      handlePullRequest: vi.fn().mockResolvedValue({ ok: true }),
      handleInstallation: vi.fn().mockResolvedValue({ ok: true }),
    };
    controller = new WebhooksController(service as unknown as WebhooksService);
    process.env.WEBHOOK_SECRET = 'test-secret';
  });

  afterEach(() => {
    if (originalSecret === undefined) {
      delete process.env.WEBHOOK_SECRET;
    } else {
      process.env.WEBHOOK_SECRET = originalSecret;
    }
  });

  it('accepts a request whose signature matches the exact raw bytes', async () => {
    const payload = { action: 'opened' };
    const rawBody = Buffer.from(JSON.stringify(payload));
    const signature = sign(rawBody, 'test-secret');

    await expect(
      controller.handleGitHub('pull_request', signature, makeReq(rawBody), payload),
    ).resolves.toEqual({ ok: true });
    expect(service.handlePullRequest).toHaveBeenCalledWith(payload);
  });

  it('rejects a signature computed over a re-serialized body (key-order drift)', async () => {
    // Regression guard: GitHub signs the exact raw bytes it sent. The original
    // bug re-serialized the Nest-parsed object via JSON.stringify() before
    // hashing, which silently breaks whenever key order/whitespace differs
    // from what GitHub actually sent.
    const rawBody = Buffer.from('{"b":2,"a":1}');
    const reserialized = Buffer.from(JSON.stringify({ a: 1, b: 2 }));
    const signature = sign(reserialized, 'test-secret');

    await expect(
      controller.handleGitHub('pull_request', signature, makeReq(rawBody), { a: 1, b: 2 }),
    ).rejects.toThrow();
    expect(service.handlePullRequest).not.toHaveBeenCalled();
  });

  it('rejects when the signature header is missing', async () => {
    const rawBody = Buffer.from('{}');
    await expect(
      controller.handleGitHub('pull_request', undefined as unknown as string, makeReq(rawBody), {}),
    ).rejects.toThrow();
  });

  it('rejects when the raw body was not captured', async () => {
    const signature = sign(Buffer.from('{}'), 'test-secret');
    await expect(
      controller.handleGitHub('pull_request', signature, makeReq(undefined), {}),
    ).rejects.toThrow();
  });

  it('fails closed (rejects) when WEBHOOK_SECRET is not configured', async () => {
    delete process.env.WEBHOOK_SECRET;
    const rawBody = Buffer.from('{}');

    await expect(
      controller.handleGitHub('pull_request', 'sha256=whatever', makeReq(rawBody), {}),
    ).rejects.toThrow();
    expect(service.handlePullRequest).not.toHaveBeenCalled();
  });

  it('rejects a tampered payload whose signature no longer matches', async () => {
    const rawBody = Buffer.from(JSON.stringify({ action: 'opened' }));
    const signature = sign(rawBody, 'a-different-secret');

    await expect(
      controller.handleGitHub('pull_request', signature, makeReq(rawBody), { action: 'opened' }),
    ).rejects.toThrow();
  });
});
