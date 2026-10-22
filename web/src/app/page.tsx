import Image from 'next/image';
import Link from 'next/link';

const assetPrefix = process.env.GITHUB_ACTIONS ? '/open-review' : '';

const capabilities = [
  {
    label: 'Review the right lines',
    detail:
      'Findings stay tied to changed code. Ambiguous positions stay visible for checking instead of becoming misplaced comments.',
    marker: '01',
  },
  {
    label: 'Bring the codebase along',
    detail:
      'Read nearby files, apply path-specific rules, and use bounded review rounds to understand the change in context.',
    marker: '02',
  },
  {
    label: 'Keep the decision yours',
    detail:
      'See severity, explanation, cited range, and a suggested replacement. Review the evidence before applying a fix.',
    marker: '03',
  },
];

export default function Home() {
  return (
    <main>
      <section className="hero shell">
        <div className="hero-copy">
          <span className="eyebrow">OPEN SOURCE CODE REVIEW</span>
          <h1>
            Review the change.
            <br />
            <em>Understand the code.</em>
          </h1>
          <p>
            Open source reviews that pair diff evidence with repository context, then place findings
            where developers can act.
          </p>
          <div className="hero-actions">
            <Link className="button button-primary" href="/quickstart">
              Start reviewing <span aria-hidden="true">↗</span>
            </Link>
            <Link className="text-link" href="/features">
              Explore features <span aria-hidden="true">↗</span>
            </Link>
          </div>
        </div>
        <div className="hero-visual">
          <Image
            src={`${assetPrefix}/images/review-workflow-hero.png`}
            alt="A code review interface on a monitor above a notebook with a flow sketch"
            fill
            priority
            sizes="(max-width: 900px) 100vw, 52vw"
          />
        </div>
      </section>

      <section className="principles">
        <div className="shell principles-inner">
          <div className="section-intro">
            <span className="section-rule" aria-hidden="true" />
            <h2>A review is only useful when you can trust its location.</h2>
            <p>
              open-review checks findings against the diff before sharing them. You get a precise
              comment when the evidence fits, and a clear signal when it does not.
            </p>
          </div>
          <div className="capability-list">
            {capabilities.map((item) => (
              <article className="capability" key={item.marker}>
                <span className="capability-index">{item.marker}</span>
                <div>
                  <h3>{item.label}</h3>
                  <p>{item.detail}</p>
                </div>
                <span className="capability-arrow" aria-hidden="true">
                  ↗
                </span>
              </article>
            ))}
          </div>
        </div>
      </section>

      <section className="product-rail shell" aria-labelledby="review-paths-title">
        <div className="product-rail-heading">
          <span className="eyebrow">REVIEW PATHS</span>
          <h2 id="review-paths-title">One review engine. Three ways in.</h2>
        </div>
        <div className="product-rail-grid">
          <div>
            <strong>Workspace</strong>
            <p>Inspect staged, unstaged, and untracked changes before a commit.</p>
          </div>
          <div>
            <strong>Branch or commit</strong>
            <p>Compare a branch from its merge base, or review one commit.</p>
          </div>
          <div>
            <strong>Pull request</strong>
            <p>Use the bundled Action to place verified findings on the reviewed head.</p>
          </div>
        </div>
        <Link className="text-link" href="/features">
          Compare review paths <span aria-hidden="true">↗</span>
        </Link>
      </section>

      <section className="workflow shell">
        <div className="workflow-image">
          <Image
            src={`${assetPrefix}/images/review-process.png`}
            alt="Notebook flow sketch beside a keyboard and code diff"
            fill
            sizes="(max-width: 900px) 100vw, 47vw"
          />
        </div>
        <div className="workflow-copy">
          <span className="eyebrow">FITS THE WAY YOU WORK</span>
          <h2>From your terminal to the pull request.</h2>
          <p>
            Run the Go review engine on a local checkout, or let the bundled GitHub Action review a
            pull request. The same finding contract reaches both paths.
          </p>
          <div className="command" aria-label="Example local review command">
            <span className="command-prompt" aria-hidden="true">
              $
            </span>
            <code>go run ./cmd/open-review review --workspace</code>
          </div>
          <Link className="text-link" href="/docs">
            Read the documentation <span aria-hidden="true">↗</span>
          </Link>
        </div>
      </section>

      <section className="closing">
        <div className="shell closing-inner">
          <div>
            <h2>Make the next review count.</h2>
            <p>Start from source. Keep every finding open to inspection.</p>
          </div>
          <Link className="button button-light" href="/quickstart">
            Get started <span aria-hidden="true">↗</span>
          </Link>
        </div>
      </section>
    </main>
  );
}
