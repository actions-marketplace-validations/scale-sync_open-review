import type { Metadata } from 'next';

import PageHeading from '../../components/PageHeading';

export const metadata: Metadata = {
  title: 'Distribution | open-review',
  description:
    'Install the Open Review SDKs and agent skill from public registries and marketplaces.',
};

const sdks = [
  {
    label: 'TypeScript SDK',
    detail: 'Install the scoped package from npm.',
    href: 'https://www.npmjs.com/package/@scalesync/open-review',
    target: 'npmjs.com/@scalesync/open-review',
  },
  {
    label: 'Python SDK',
    detail: 'Install the published package from PyPI.',
    href: 'https://pypi.org/project/openreview-sdk/',
    target: 'pypi.org/project/openreview-sdk',
  },
  {
    label: 'Rust SDK',
    detail: 'Add the crate from crates.io.',
    href: 'https://crates.io/crates/openreview-sdk',
    target: 'crates.io/crates/openreview-sdk',
  },
  {
    label: 'Go SDK',
    detail: 'Import the Go module from pkg.go.dev.',
    href: 'https://pkg.go.dev/github.com/scale-sync/open-review/sdk/go',
    target: 'pkg.go.dev/.../sdk/go',
  },
  {
    label: 'Binaries and releases',
    detail: 'Download release assets and checksums from GitHub.',
    href: 'https://github.com/scale-sync/open-review/releases',
    target: 'github.com/scale-sync/open-review/releases',
  },
];

const agents = [
  {
    label: 'Codex / ChatGPT',
    detail: 'Portable plugin and skill package; submit through the OpenAI plugin directory.',
    href: 'https://developers.openai.com/plugins/deploy/submission',
  },
  {
    label: 'Claude Code',
    detail: 'Claude plugin marketplace manifest with the shared open-review skill.',
    href: 'https://code.claude.com/docs/en/plugins',
  },
  {
    label: 'Cursor',
    detail: 'Cursor plugin manifest ready for Marketplace submission and local installation.',
    href: 'https://cursor.com/marketplace/publish',
  },
  {
    label: 'Gemini CLI',
    detail: 'Gemini extension installable directly from this GitHub repository.',
    href: 'https://github.com/google-gemini/gemini-cli/blob/main/docs/extensions/reference.md',
  },
];

export default function Distribution() {
  return (
    <main>
      <PageHeading
        eyebrow="PUBLIC DISTRIBUTION"
        title="Install Open Review where your team already works."
        description="The v1.12.3 release is available through SDK registries, GitHub releases, and a shared agent skill packaged for Codex, Claude Code, Cursor, and Gemini CLI."
      />
      <section className="shell feature-grid" aria-labelledby="sdk-title">
        <div className="section-heading">
          <span className="eyebrow">SDKs AND RELEASES</span>
          <h2 id="sdk-title">Use the review engine from your stack.</h2>
          <p>Choose the registry or release channel that matches your application.</p>
        </div>
        {sdks.map((item, index) => (
          <article className="feature-card" key={item.label}>
            <span>{String(index + 1).padStart(2, '0')}</span>
            <h2>{item.label}</h2>
            <p>{item.detail}</p>
            <a className="text-link" href={item.href} target="_blank" rel="noreferrer">
              {item.target} <span aria-hidden="true">↗</span>
            </a>
          </article>
        ))}
      </section>
      <section className="section-band" aria-labelledby="agent-title">
        <div className="shell feature-grid">
          <div className="section-heading">
            <span className="eyebrow">AGENT PLUGINS AND SKILLS</span>
            <h2 id="agent-title">One shared review workflow across agents.</h2>
            <p>
              Install the plugin package, then install the Go CLI used by the skill. The skill
              previews coverage before delegation and returns actionable findings without editing
              the reviewed repository.
            </p>
          </div>
          {agents.map((item, index) => (
            <article className="feature-card" key={item.label}>
              <span>{String(index + 1).padStart(2, '0')}</span>
              <h2>{item.label}</h2>
              <p>{item.detail}</p>
              <a className="text-link" href={item.href} target="_blank" rel="noreferrer">
                Installation and submission <span aria-hidden="true">↗</span>
              </a>
            </article>
          ))}
        </div>
      </section>
    </main>
  );
}
