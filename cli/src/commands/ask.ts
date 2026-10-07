import { createInterface } from 'node:readline';

import {
  GitHubClient,
  createMainLLM,
  extractDiffForFiles,
  isContextCandidate,
  streamChat,
} from '@openreview/core';
import type { Command } from 'commander';

import { loadLocalAskContext, markUnverifiedCitations } from './ask-context.js';

export function registerAskCommand(program: Command): void {
  program
    .command('ask')
    .description('Ask questions about a repository or PR')
    .option('--repo <path>', 'Local repository path', '.')
    .option('--url <url>', 'GitHub PR URL for context')
    .action(async (opts) => {
      try {
        console.log('open-review Ask — Interactive Q&A');
        console.log('Commands: reset, history, files, exit\n');

        const thread: Array<{ role: 'human' | 'ai'; content: string }> = [];
        let prContext = '';
        let prPaths: string[] = [];
        const prLines = new Map<string, number>();
        if (opts.url) {
          const { client, prNumber } = GitHubClient.fromPRUrl(opts.url);
          const [meta, diff, files] = await Promise.all([
            client.getPR(prNumber),
            client.getPRDiff(prNumber),
            client.getPRFiles(prNumber),
          ]);
          prPaths = files.map((file) => file.filename);
          const safePaths = prPaths.filter(isContextCandidate);
          const snippets: string[] = [];
          let remaining = 20_000;
          for (const path of safePaths.slice(0, 8)) {
            if (remaining <= 0) break;
            try {
              const content = (await client.getFileContent(path, meta.head.sha)).slice(
                0,
                Math.min(remaining, 3_000),
              );
              if (!content || content.includes('\0')) continue;
              const lines = content.split('\n');
              prLines.set(path, lines.length);
              snippets.push(
                path +
                  '\n' +
                  lines.map((line, index) => String(index + 1) + ': ' + line).join('\n'),
              );
              remaining -= content.length;
            } catch {
              continue;
            }
          }
          prContext =
            'You are open-review. Answer from this bounded PR diff and file excerpts. Cite code as path:line. If the answer is not present, say so.\n' +
            'Repository: ' +
            client.owner +
            '/' +
            client.repo +
            '\n' +
            'PR #' +
            prNumber +
            ': ' +
            meta.title +
            '\n' +
            'Diff:\n' +
            extractDiffForFiles(diff, safePaths).slice(0, 20_000) +
            '\nFile excerpts:\n' +
            snippets.join('\n\n');
          console.log(
            'Loaded PR #' + prNumber + ': ' + meta.title + ' (' + files.length + ' files)\n',
          );
        }

        const rl = createInterface({
          input: process.stdin,
          output: process.stdout,
          prompt: 'open-review> ',
        });
        let busy = false;
        rl.prompt();
        rl.on('line', async (input) => {
          const question = input.trim();
          if (!question) {
            rl.prompt();
            return;
          }
          if (question === 'exit' || question === 'quit') {
            rl.close();
            return;
          }
          if (question === 'reset') {
            thread.length = 0;
            console.log('Thread cleared.');
            rl.prompt();
            return;
          }
          if (question === 'history') {
            for (const message of thread)
              console.log(
                (message.role === 'human' ? 'You: ' : 'AI: ') + message.content.slice(0, 100),
              );
            rl.prompt();
            return;
          }
          if (question === 'files') {
            const paths = opts.url ? prPaths : (await loadLocalAskContext(opts.repo, '')).paths;
            for (const path of paths) console.log('  ' + path);
            rl.prompt();
            return;
          }
          if (busy) {
            console.log('Waiting for the previous answer.');
            return;
          }
          busy = true;
          try {
            const local = opts.url ? null : await loadLocalAskContext(opts.repo, question);
            const messages = [
              { role: 'system', content: local?.prompt ?? prContext },
              ...thread.slice(-10),
              { role: 'human', content: question.slice(0, 2_000) },
            ];
            const llm = createMainLLM();
            let answer = '';
            for await (const chunk of streamChat(llm, messages)) answer += chunk;
            answer = markUnverifiedCitations(answer, local?.lineCounts ?? prLines);
            console.log(answer + '\n');
            thread.push(
              { role: 'human', content: question.slice(0, 2_000) },
              { role: 'ai', content: answer.slice(0, 4_000) },
            );
          } catch (err) {
            console.error('Error: ' + (err instanceof Error ? err.message : String(err)));
          } finally {
            busy = false;
            rl.prompt();
          }
        });
        rl.on('close', () => console.log('Goodbye.'));
      } catch (err) {
        process.stderr.write('Error: ' + (err instanceof Error ? err.message : String(err)) + '\n');
        process.exitCode = 1;
      }
    });
}
