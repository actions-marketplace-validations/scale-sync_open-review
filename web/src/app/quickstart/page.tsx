import type { Metadata } from 'next';
import Link from 'next/link';

import PageHeading from '../../components/PageHeading';

export const metadata: Metadata = { title: 'Quick start | open-review' };

const steps = [
  {
    number: '01',
    title: 'Build from source',
    copy: 'Clone the repository and build the Go review engine. Go 1.23 or later is required.',
    command:
      'git clone https://github.com/scale-sync/open-review.git\ncd open-review\ngo build -o open-review ./cmd/open-review',
  },
  {
    number: '02',
    title: 'Choose a model',
    copy: 'Save your provider and model locally. The setup command accepts the key without printing it.',
    command: './open-review config setup\n./open-review config test',
  },
  {
    number: '03',
    title: 'Review your work',
    copy: 'Run inside the repository you want to review. Workspace mode includes staged, unstaged, and untracked changes.',
    command: '/path/to/open-review/open-review review --workspace',
  },
];

export default function Quickstart() {
  return (
    <main>
      <PageHeading
        eyebrow="QUICK START"
        title="From checkout to first review."
        description="Run the Go CLI locally, inspect the findings, then add pull request review when you are ready."
      />
      <div className="shell quickstart-layout">
        <div className="steps-list">
          {steps.map((step) => (
            <section className="quick-step" key={step.number}>
              <span className="step-number">{step.number}</span>
              <div>
                <h2>{step.title}</h2>
                <p>{step.copy}</p>
                <pre>
                  <code>{step.command}</code>
                </pre>
              </div>
            </section>
          ))}
        </div>
        <aside className="side-note">
          <span className="eyebrow">NEXT PATHS</span>
          <h2>Use it where you work.</h2>
          <p>
            Review a branch or commit in the terminal, or run the bundled Action on pull requests.
          </p>
          <Link href="/docs/getting-started">
            Local guide <span aria-hidden="true">↗</span>
          </Link>
          <Link href="/docs/github-action">
            GitHub Action guide <span aria-hidden="true">↗</span>
          </Link>
          <Link href="/distribution">
            SDKs and agent plugins <span aria-hidden="true">↗</span>
          </Link>
          <p className="note-small">
            SDK packages and the shared agent skill are available for installation.
          </p>
        </aside>
      </div>
    </main>
  );
}
