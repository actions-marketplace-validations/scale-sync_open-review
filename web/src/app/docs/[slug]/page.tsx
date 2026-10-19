import type { Metadata } from 'next';
import Link from 'next/link';
import { notFound } from 'next/navigation';

import MarkdownArticle, { headingId } from '../../../components/MarkdownArticle';
import { documents, readProjectContent } from '../../../content/library';

export function generateStaticParams() {
  return documents.map(({ slug }) => ({ slug }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  const document = documents.find((item) => item.slug === slug);
  return { title: document ? `${document.title} | open-review Docs` : 'Docs | open-review' };
}

export default async function DocumentPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const document = documents.find((item) => item.slug === slug);
  if (!document) notFound();
  const content = readProjectContent(document.source);
  const sections = [...content.matchAll(/^## (.+)$/gm)].map((match) => ({
    label: match[1],
    id: headingId(match[1]),
  }));
  const groups = [...new Set(documents.map((item) => item.group))];

  return (
    <main className="docs-layout shell">
      <aside className="docs-sidebar" aria-label="Documentation">
        <Link className="docs-home" href="/docs">
          Documentation <span aria-hidden="true">↗</span>
        </Link>
        {groups.map((group) => (
          <div className="docs-nav-group" key={group}>
            <span>{group}</span>
            {documents
              .filter((item) => item.group === group)
              .map((item) => (
                <Link
                  aria-current={item.slug === slug ? 'page' : undefined}
                  href={`/docs/${item.slug}`}
                  key={item.slug}
                >
                  {item.title}
                </Link>
              ))}
          </div>
        ))}
      </aside>
      <div className="docs-main">
        <div className="docs-intro">
          <span className="eyebrow">DOCUMENTATION / {document.group.toUpperCase()}</span>
          <h1>{document.title}</h1>
          <p>{document.summary}</p>
        </div>
        <MarkdownArticle content={content} source={document.source} />
      </div>
      <aside className="docs-toc" aria-label="On this page">
        <span>ON THIS PAGE</span>
        {sections.map((section) => (
          <a href={`#${section.id}`} key={section.id}>
            {section.label}
          </a>
        ))}
      </aside>
    </main>
  );
}
