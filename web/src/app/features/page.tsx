import type { Metadata } from 'next';
import Link from 'next/link';

import PageHeading from '../../components/PageHeading';

export const metadata: Metadata = { title: 'Features | open-review' };

const reviewPaths = [
  {
    path: 'Workspace',
    scope: 'Staged, unstaged, and untracked files',
    place: 'Local terminal',
    use: 'Before you commit',
  },
  {
    path: 'Branch',
    scope: 'Changes since the merge base',
    place: 'Local terminal',
    use: 'Before you open a pull request',
  },
  {
    path: 'Commit',
    scope: 'One selected commit',
    place: 'Local terminal',
    use: 'Inspect a focused change',
  },
  {
    path: 'Pull request',
    scope: 'Changed lines at a checked head',
    place: 'GitHub Action',
    use: 'Review with your team',
  },
];

const features = [
  {
    index: '01',
    title: 'Findings tied to evidence',
    copy: 'Every finding carries a location and explanation. The pull request path checks the citation against the diff before posting an inline comment.',
  },
  {
    index: '02',
    title: 'Repository-aware review',
    copy: 'Nearby code, related files, path-specific rules, and bounded tool calls give the model context around a change.',
  },
  {
    index: '03',
    title: 'A clear review budget',
    copy: 'Choose low, medium, or high effort. Token, request, and group limits keep a run within the budget you set.',
  },
  {
    index: '04',
    title: 'Rules you can trace',
    copy: 'Review guidance resolves by scope and path. Diagnostics show which rule applies to a file.',
  },
  {
    index: '05',
    title: 'Safe suggestions',
    copy: 'A suggested replacement stays beside its severity and explanation, ready for a developer to inspect.',
  },
  {
    index: '06',
    title: 'Open model choice',
    copy: 'Use OpenAI, Anthropic, Gemini, or an OpenAI-compatible endpoint through the same review flow.',
  },
];

export default function Features() {
  return (
    <main>
      <PageHeading
        eyebrow="CAPABILITIES"
        title="A review you can follow through."
        description="Pick the change you want to inspect. open-review brings the codebase context, checks the evidence, and leaves the decision with you."
      />
      <section className="shell feature-grid" aria-label="Capabilities">
        {features.map((feature) => (
          <article className="feature-card" key={feature.index}>
            <span>{feature.index}</span>
            <h2>{feature.title}</h2>
            <p>{feature.copy}</p>
          </article>
        ))}
      </section>
      <section className="section-band">
        <div className="shell">
          <div className="section-heading">
            <span className="eyebrow">REVIEW PATHS</span>
            <h2>Start at the right point in your workflow.</h2>
            <p>The same Go review engine covers local changes and pull requests.</p>
          </div>
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Path</th>
                  <th>Change set</th>
                  <th>Where</th>
                  <th>Best for</th>
                </tr>
              </thead>
              <tbody>
                {reviewPaths.map((path) => (
                  <tr key={path.path}>
                    <th>{path.path}</th>
                    <td>{path.scope}</td>
                    <td>{path.place}</td>
                    <td>{path.use}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="section-actions">
            <Link className="button button-primary" href="/quickstart">
              Start a review ↗
            </Link>
            <Link className="text-link" href="/docs/model-selection">
              Compare model settings ↗
            </Link>
          </div>
        </div>
      </section>
    </main>
  );
}
