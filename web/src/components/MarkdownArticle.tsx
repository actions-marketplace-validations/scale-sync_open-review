import path from 'node:path';
import type { ReactNode } from 'react';
import Link from 'next/link';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

import { documentUrl, repository } from '../content/library';

export function headingId(children: ReactNode): string {
  return String(children)
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, '')
    .trim()
    .replace(/\s+/g, '-');
}

export default function MarkdownArticle({ content, source }: { content: string; source: string }) {
  return (
    <article className="markdown-article">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          h2: ({ children }) => <h2 id={headingId(children)}>{children}</h2>,
          h3: ({ children }) => <h3 id={headingId(children)}>{children}</h3>,
          a: ({ href = '', children }) => {
            if (href.startsWith('#')) return <a href={href}>{children}</a>;
            if (href.startsWith('/')) return <Link href={href}>{children}</Link>;
            if (/^https?:\/\//.test(href))
              return (
                <a href={href} target="_blank" rel="noreferrer">
                  {children}
                </a>
              );
            const resolved = path.posix.normalize(
              path.posix.join(path.posix.dirname(source), href),
            );
            const local = documentUrl(resolved);
            return local ? (
              <Link href={local}>{children}</Link>
            ) : (
              <a href={`${repository}/blob/main/${resolved}`} target="_blank" rel="noreferrer">
                {children}
              </a>
            );
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </article>
  );
}
