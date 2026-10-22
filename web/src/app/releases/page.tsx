import type { Metadata } from 'next';

import PageHeading from '../../components/PageHeading';
import { releases } from '../../content/releases';

export const metadata: Metadata = {
  title: 'Releases | open-review',
  description: 'A serialized record of every open-review release, from v0.5.0 onward.',
};

export default function Releases() {
  return (
    <main>
      <PageHeading
        eyebrow="RELEASE HISTORY"
        title="Everything that shipped, in order."
        description="A concise record of the features, fixes, and improvements behind each open-review release."
      />
      <section className="shell release-list" aria-label="Release history">
        {releases.map((release) => (
          <article className="release-entry" key={release.version}>
            <div className="release-meta">
              <span className="release-version">{release.version}</span>
              <time dateTime={release.date}>{release.date}</time>
            </div>
            <div className="release-content">
              <h2>{release.name}</h2>
              <div className="release-detail-grid">
                <div>
                  <h3>Features</h3>
                  <p>{release.features}</p>
                </div>
                <div>
                  <h3>Fixes &amp; improvements</h3>
                  <p>{release.fixes}</p>
                </div>
                <div>
                  <h3>Why it matters</h3>
                  <p>{release.impact}</p>
                </div>
              </div>
            </div>
          </article>
        ))}
      </section>
    </main>
  );
}
