import { readFileSync } from 'node:fs';
import path from 'node:path';

export const repository = 'https://github.com/scale-sync/open-review';

export const documents = [
  {
    slug: 'getting-started',
    title: 'Getting started',
    group: 'Start here',
    source: 'GETTING_STARTED.md',
    summary: 'Build from source and run your first local review.',
  },
  {
    slug: 'installation',
    title: 'Installation and setup',
    group: 'Start here',
    source: 'SETUP.md',
    summary: 'Requirements, model keys, local review, and checks.',
  },
  {
    slug: 'review-engine',
    title: 'Review engine',
    group: 'Guides',
    source: 'review/README.md',
    summary: 'Selection, context, review rounds, rules, and results.',
  },
  {
    slug: 'github-action',
    title: 'GitHub Action',
    group: 'Guides',
    source: 'web/content/docs/github-action.md',
    summary: 'Pull request review from a workflow.',
  },
  {
    slug: 'model-selection',
    title: 'Model selection',
    group: 'Guides',
    source: 'web/content/docs/model-selection.md',
    summary: 'Provider support, effort levels, and review budgets.',
  },
  {
    slug: 'high-recall',
    title: 'High-recall mode',
    group: 'Guides',
    source: 'web/content/docs/high-recall.md',
    summary: 'Extended exploration rounds and deep context for high-risk changes.',
  },
  {
    slug: 'durable-learnings',
    title: 'Durable learnings',
    group: 'Guides',
    source: 'web/content/docs/durable-learnings.md',
    summary: 'Repository-scoped feedback, triage memory, and prompt safety.',
  },
  {
    slug: 'localization',
    title: 'Localization',
    group: 'Guides',
    source: 'web/content/docs/localization.md',
    summary: 'Multilingual CLI and guidance with invariant machine interfaces.',
  },
  {
    slug: 'benchmark-method',
    title: 'Benchmark method',
    group: 'Guides',
    source: 'benchmarks/README.md',
    summary: 'Dataset, runner, human judgments, metrics, and publication rules.',
  },
  {
    slug: 'release-notes',
    title: 'Release notes',
    group: 'Reference',
    source: '_plan_docs/releases/v1.9.0.md',
    summary: 'What shipped in the latest release.',
  },
  {
    slug: 'contributing',
    title: 'Contributing',
    group: 'Reference',
    source: 'CONTRIBUTING.md',
    summary: 'Work on the project and submit changes.',
  },
] as const;

export const posts = [
  {
    slug: 'precise-comments',
    title: 'Precise, actionable comments',
    date: 'October 16, 2026',
    summary: 'How citation checks, relocation, and suggestions changed review output.',
    source: '_plan_docs/releases/v1.3.0.md',
  },
  {
    slug: 'explainable-rules',
    title: 'Rules with a clear source',
    date: 'October 16, 2026',
    summary: 'Per-file guidance, precedence, and diagnostics in the review workflow.',
    source: '_plan_docs/releases/v1.2.0.md',
  },
] as const;

export function readProjectContent(source: string): string {
  switch (source) {
    case 'GETTING_STARTED.md':
      return readFileSync(path.join(process.cwd(), '..', 'GETTING_STARTED.md'), 'utf8');
    case 'SETUP.md':
      return readFileSync(path.join(process.cwd(), '..', 'SETUP.md'), 'utf8');
    case 'review/README.md':
      return readFileSync(path.join(process.cwd(), '..', 'review', 'README.md'), 'utf8');
    case 'web/content/docs/github-action.md':
      return readFileSync(path.join(process.cwd(), 'content', 'docs', 'github-action.md'), 'utf8');
    case 'web/content/docs/model-selection.md':
      return readFileSync(
        path.join(process.cwd(), 'content', 'docs', 'model-selection.md'),
        'utf8',
      );
    case 'web/content/docs/high-recall.md':
      return readFileSync(path.join(process.cwd(), 'content', 'docs', 'high-recall.md'), 'utf8');
    case 'web/content/docs/durable-learnings.md':
      return readFileSync(
        path.join(process.cwd(), 'content', 'docs', 'durable-learnings.md'),
        'utf8',
      );
    case 'web/content/docs/localization.md':
      return readFileSync(
        path.join(process.cwd(), 'content', 'docs', 'localization.md'),
        'utf8',
      );
    case 'benchmarks/README.md':
      return readFileSync(path.join(process.cwd(), '..', 'benchmarks', 'README.md'), 'utf8');
    case '_plan_docs/releases/v1.10.0.md':
      return readFileSync(
        path.join(process.cwd(), '..', '_plan_docs', 'releases', 'v1.10.0.md'),
        'utf8',
      );
    case '_plan_docs/releases/v1.9.0.md':
      return readFileSync(
        path.join(process.cwd(), '..', '_plan_docs', 'releases', 'v1.9.0.md'),
        'utf8',
      );
    case '_plan_docs/releases/v1.3.0.md':
      return readFileSync(
        path.join(process.cwd(), '..', '_plan_docs', 'releases', 'v1.3.0.md'),
        'utf8',
      );
    case '_plan_docs/releases/v1.2.0.md':
      return readFileSync(
        path.join(process.cwd(), '..', '_plan_docs', 'releases', 'v1.2.0.md'),
        'utf8',
      );
    case 'CONTRIBUTING.md':
      return readFileSync(path.join(process.cwd(), '..', 'CONTRIBUTING.md'), 'utf8');
    default:
      throw new Error(`Unknown content source: ${source}`);
  }
}

export function documentUrl(source: string): string | undefined {
  const document = documents.find((item) => item.source === source);
  return document ? `/docs/${document.slug}` : undefined;
}
