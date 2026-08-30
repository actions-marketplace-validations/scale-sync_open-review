import { createHmac, timingSafeEqual } from 'crypto';

import {
  Body,
  Controller,
  Headers,
  HttpCode,
  Post,
  Req,
  UnauthorizedException,
  type RawBodyRequest,
} from '@nestjs/common';
import type { Request } from 'express';

import { WebhooksService } from './webhooks.service';

@Controller('webhooks')
export class WebhooksController {
  constructor(private readonly webhooksService: WebhooksService) {}

  @Post('github')
  @HttpCode(200)
  async handleGitHub(
    @Headers('x-github-event') event: string,
    @Headers('x-hub-signature-256') signature: string,
    @Req() req: RawBodyRequest<Request>,
    @Body() payload: Record<string, unknown>,
  ) {
    this.verifySignature(req.rawBody, signature);

    if (event === 'pull_request') {
      return this.webhooksService.handlePullRequest(payload);
    }

    // GitHub App installation events — fired when someone installs/uninstalls
    // the app or grants/revokes access to individual repos.
    if (event === 'installation' || event === 'installation_repositories') {
      return this.webhooksService.handleInstallation(payload);
    }

    return { received: true, event };
  }

  /**
   * Verify against the raw request bytes, not a re-serialized JSON.stringify
   * of the parsed body — GitHub signs the exact bytes it sent, and re-encoding
   * (key order, whitespace, unicode escaping) will not byte-match in general.
   */
  private verifySignature(rawBody: Buffer | undefined, signature?: string) {
    const secret = process.env.WEBHOOK_SECRET;
    if (!secret) {
      throw new UnauthorizedException(
        'WEBHOOK_SECRET is not configured — refusing to accept unauthenticated webhooks',
      );
    }

    if (!rawBody) {
      throw new UnauthorizedException('Missing raw request body');
    }

    if (!signature) {
      throw new UnauthorizedException('Missing webhook signature');
    }

    const expected =
      'sha256=' + createHmac('sha256', secret).update(rawBody).digest('hex');

    const sigBuffer = Buffer.from(signature);
    const expBuffer = Buffer.from(expected);

    if (
      sigBuffer.length !== expBuffer.length ||
      !timingSafeEqual(sigBuffer, expBuffer)
    ) {
      throw new UnauthorizedException('Invalid webhook signature');
    }
  }
}
