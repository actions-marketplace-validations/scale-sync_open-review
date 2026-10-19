import type { Metadata } from 'next';
import Link from 'next/link';
import { notFound } from 'next/navigation';

import MarkdownArticle from '../../../components/MarkdownArticle';
import { posts, readProjectContent } from '../../../content/library';

export function generateStaticParams() {
  return posts.map(({ slug }) => ({ slug }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  const post = posts.find((item) => item.slug === slug);
  return { title: post ? `${post.title} | open-review Journal` : 'Journal | open-review' };
}

export default async function BlogPost({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const post = posts.find((item) => item.slug === slug);
  if (!post) notFound();
  const content = readProjectContent(post.source);
  return (
    <main className="article-page shell">
      <Link className="back-link" href="/blog">
        ← All articles
      </Link>
      <div className="article-head">
        <span className="eyebrow">JOURNAL / {post.date.toUpperCase()}</span>
        <h1>{post.title}</h1>
        <p>{post.summary}</p>
      </div>
      <MarkdownArticle content={content} source={post.source} />
      <div className="article-end">
        <Link href="/docs/release-notes">Read the latest release notes ↗</Link>
      </div>
    </main>
  );
}
