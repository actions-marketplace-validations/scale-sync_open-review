import type { Metadata } from 'next';
import Link from 'next/link';

import PageHeading from '../../components/PageHeading';
import { posts } from '../../content/library';

export const metadata: Metadata = { title: 'Journal | open-review' };

export default function Blog() {
  return (
    <main>
      <PageHeading
        eyebrow="JOURNAL"
        title="Notes from the work."
        description="Release stories and practical notes on building reviews that developers can inspect and use."
      />
      <section className="shell post-list" aria-label="Journal articles">
        {posts.map((post) => (
          <Link className="post-card" href={`/blog/${post.slug}`} key={post.slug}>
            <span>{post.date} · ENGINEERING</span>
            <h2>{post.title}</h2>
            <p>{post.summary}</p>
            <strong>Read article ↗</strong>
          </Link>
        ))}
      </section>
    </main>
  );
}
