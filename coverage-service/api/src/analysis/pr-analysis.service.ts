import { Injectable, Logger, NotFoundException } from '@nestjs/common';

import { PrismaService } from '../prisma/prisma.service';
import { QueueService } from '../queue/queue.service';

export interface TriggerAnalysisInput {
  repositoryId: string;
  prNumber: number;
  headBranch: string;
  headSha: string;
  baseBranch: string;
}

@Injectable()
export class PrAnalysisService {
  private readonly logger = new Logger(PrAnalysisService.name);

  constructor(
    private readonly prisma: PrismaService,
    private readonly queue: QueueService,
  ) {}

  async trigger(input: TriggerAnalysisInput) {
    const repository = await this.prisma.repository.findUnique({
      where: { id: input.repositoryId },
    });
    if (!repository) {
      throw new NotFoundException('Repository not found');
    }

    // Two webhook retries (or two manual /analyze calls) for the same PR head
    // SHA would otherwise race to create two PullRequestRun rows and two
    // concurrent clone→coverage→LLM jobs. Reuse any run still in flight.
    const existing = await this.prisma.pullRequestRun.findFirst({
      where: {
        repositoryId: repository.id,
        prNumber: input.prNumber,
        headSha: input.headSha,
        status: { notIn: ['COMPLETED', 'FAILED'] },
      },
      orderBy: { startedAt: 'desc' },
    });

    if (existing) {
      this.logger.log(
        `Reusing in-progress PR analysis run ${existing.id} for ${repository.githubRepo}#${input.prNumber}@${input.headSha}`,
      );
      return {
        prRunId: existing.id,
        status: 'duplicate' as const,
        repository: repository.githubRepo,
        prNumber: input.prNumber,
      };
    }

    const prRun = await this.prisma.pullRequestRun.create({
      data: {
        repositoryId: repository.id,
        prNumber: input.prNumber,
        headBranch: input.headBranch,
        headSha: input.headSha,
        baseBranch: input.baseBranch,
        status: 'PENDING',
      },
    });

    await this.queue.enqueuePrAnalysis({
      prRunId: prRun.id,
      repositoryId: repository.id,
      prNumber: input.prNumber,
      baseBranch: input.baseBranch,
      headBranch: input.headBranch,
      headSha: input.headSha,
    });

    this.logger.log(
      `Enqueued PR analysis for ${repository.githubRepo}#${input.prNumber}`,
    );

    return {
      prRunId: prRun.id,
      status: 'enqueued' as const,
      repository: repository.githubRepo,
      prNumber: input.prNumber,
    };
  }
}
