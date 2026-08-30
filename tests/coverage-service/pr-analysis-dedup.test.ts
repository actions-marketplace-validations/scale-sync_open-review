import { beforeEach, describe, expect, it, vi } from 'vitest';

import { PrAnalysisService } from '../../coverage-service/api/src/analysis/pr-analysis.service';
import type { PrismaService } from '../../coverage-service/api/src/prisma/prisma.service';
import type { QueueService } from '../../coverage-service/api/src/queue/queue.service';

function makeInput() {
  return {
    repositoryId: 'repo-1',
    prNumber: 42,
    headBranch: 'feature-x',
    headSha: 'abc123',
    baseBranch: 'main',
  };
}

describe('PrAnalysisService.trigger — dedup', () => {
  let prisma: {
    repository: { findUnique: ReturnType<typeof vi.fn> };
    pullRequestRun: {
      findFirst: ReturnType<typeof vi.fn>;
      create: ReturnType<typeof vi.fn>;
    };
  };
  let queue: { enqueuePrAnalysis: ReturnType<typeof vi.fn> };
  let service: PrAnalysisService;

  beforeEach(() => {
    prisma = {
      repository: {
        findUnique: vi.fn().mockResolvedValue({ id: 'repo-1', githubRepo: 'acme/widgets' }),
      },
      pullRequestRun: {
        findFirst: vi.fn(),
        create: vi.fn(),
      },
    };
    queue = { enqueuePrAnalysis: vi.fn().mockResolvedValue(undefined) };
    service = new PrAnalysisService(
      prisma as unknown as PrismaService,
      queue as unknown as QueueService,
    );
  });

  it('reuses an in-flight run instead of creating a duplicate for the same head SHA', async () => {
    prisma.pullRequestRun.findFirst.mockResolvedValue({
      id: 'existing-run',
      status: 'RUNNING_COVERAGE',
    });

    const result = await service.trigger(makeInput());

    expect(result).toEqual({
      prRunId: 'existing-run',
      status: 'duplicate',
      repository: 'acme/widgets',
      prNumber: 42,
    });
    expect(prisma.pullRequestRun.create).not.toHaveBeenCalled();
    expect(queue.enqueuePrAnalysis).not.toHaveBeenCalled();
  });

  it('creates and enqueues a new run when no non-terminal run exists for this head SHA', async () => {
    prisma.pullRequestRun.findFirst.mockResolvedValue(null);
    prisma.pullRequestRun.create.mockResolvedValue({ id: 'new-run' });

    const result = await service.trigger(makeInput());

    expect(result).toEqual({
      prRunId: 'new-run',
      status: 'enqueued',
      repository: 'acme/widgets',
      prNumber: 42,
    });
    expect(prisma.pullRequestRun.create).toHaveBeenCalledOnce();
    expect(queue.enqueuePrAnalysis).toHaveBeenCalledWith(
      expect.objectContaining({ prRunId: 'new-run' }),
    );
  });

  it('starts a new run once the previous one for the same head SHA has terminated', async () => {
    // findFirst filters out COMPLETED/FAILED runs, so a prior finished run
    // for this exact head SHA must not block a fresh trigger (e.g. a manual retry).
    prisma.pullRequestRun.findFirst.mockResolvedValue(null);
    prisma.pullRequestRun.create.mockResolvedValue({ id: 'retry-run' });

    await service.trigger(makeInput());

    expect(prisma.pullRequestRun.findFirst).toHaveBeenCalledWith(
      expect.objectContaining({
        where: expect.objectContaining({
          status: { notIn: ['COMPLETED', 'FAILED'] },
        }),
      }),
    );
    expect(prisma.pullRequestRun.create).toHaveBeenCalledOnce();
  });
});
