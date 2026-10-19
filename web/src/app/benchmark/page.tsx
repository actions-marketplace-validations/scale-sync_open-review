import type { Metadata } from 'next';
import Link from 'next/link';

import BenchmarkTable, { type BenchmarkReport } from '../../components/BenchmarkTable';
import PageHeading from '../../components/PageHeading';
import reports from '../../content/benchmark-results.json';

export const metadata: Metadata = { title: 'Benchmarks | open-review' };

const corpus = [
  { number: '200', label: 'real pull requests' },
  { number: '50', label: 'source projects' },
  { number: '10', label: 'programming languages' },
  { number: '1,505', label: 'verified issues' },
];

const measures = [
  {
    title: 'Finding quality',
    description:
      'Precision, recall, and F1 use one human-confirmed match per verified issue. Every unmatched finding counts as noise.',
  },
  {
    title: 'Location accuracy',
    description:
      'A matched finding earns a location hit only when its file and changed-line range match the annotated issue.',
  },
  {
    title: 'Time and tokens',
    description:
      'Elapsed time and model token use are averaged per pull request. Estimated usage is reported separately.',
  },
  {
    title: 'Failure visibility',
    description:
      'Partial reviews, missing cases, and findings without a judgment block publication of a score.',
  },
];

export default function Benchmark() {
  return (
    <main>
      <PageHeading
        eyebrow="OPEN BENCHMARK"
        title="Measure the review, not the promise."
        description="A fixed, real-world pull request corpus for comparing review quality, placement, time, and model use under the same conditions."
      />
      <section className="shell benchmark-corpus" aria-label="Benchmark corpus">
        {corpus.map((item) => (
          <div key={item.label}>
            <strong>{item.number}</strong>
            <span>{item.label}</span>
          </div>
        ))}
      </section>
      <section className="shell leaderboard-section">
        <div className="section-heading">
          <span className="eyebrow">OPEN-REVIEW RESULTS</span>
          <h2>Results with a trail back to the run.</h2>
          <p>
            Sort by F1, precision, recall, location, noise, time, or tokens. Filter by language when
            verified runs are available.
          </p>
        </div>
        <BenchmarkTable reports={reports as BenchmarkReport[]} />
        <p className="benchmark-note">
          No scored open-review runs have been completed yet. Results appear here only after all 200
          reviews and their findings are checked.
        </p>
      </section>
      <section className="section-band">
        <div className="shell">
          <div className="section-heading">
            <span className="eyebrow">EVALUATION METHOD</span>
            <h2>A reproducible score, with failures visible.</h2>
            <p>
              The dataset contains 2,145 review comments: 1,505 expert-verified issues and 640
              rejected comments. The same pinned pull request revisions are used for every model and
              effort setting.
            </p>
          </div>
          <div className="metrics-grid">
            {measures.map((measure) => (
              <article key={measure.title}>
                <h3>{measure.title}</h3>
                <p>{measure.description}</p>
              </article>
            ))}
          </div>
          <div className="section-actions">
            <Link className="text-link" href="/docs/benchmark-method">
              Read the method ↗
            </Link>
            <Link className="text-link" href="/docs/model-selection">
              Model settings ↗
            </Link>
          </div>
        </div>
      </section>
      <section className="shell benchmark-protocol">
        <div className="section-heading">
          <span className="eyebrow">PUBLICATION RULE</span>
          <h2>Scores wait for evidence.</h2>
          <p>
            Each release needs the dataset checksum, reviewer version, model identity, effort,
            complete per-PR outputs, and a human-reviewed one-to-one match file. The site will add
            results only after those artifacts can be reproduced.
          </p>
        </div>
        <div className="benchmark-status">
          <span className="status-dot" aria-hidden="true" />
          <div>
            <strong>Benchmark prepared · no paid model runs</strong>
            <p>
              The corpus is validated and the Go runner and scorer are available locally. Model
              comparisons remain pending.
            </p>
          </div>
        </div>
      </section>
    </main>
  );
}
