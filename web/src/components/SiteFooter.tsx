import Link from 'next/link';

const repository = 'https://github.com/scale-sync/open-review';

export default function SiteFooter() {
  return (
    <footer className="shell footer">
      <Link className="brand" href="/">
        <span className="brand-mark" aria-hidden="true">
          /
        </span>
        <span>open-review</span>
      </Link>
      <span>Open source under the MIT license.</span>
      <div className="footer-links">
        <Link href="/docs">Docs</Link>
        <Link href="/blog">Journal</Link>
        <a href={`${repository}/releases`}>Releases ↗</a>
      </div>
    </footer>
  );
}
