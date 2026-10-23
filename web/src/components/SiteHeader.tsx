import Link from 'next/link';

const repository = 'https://github.com/scale-sync/open-review';
const navigation = [
  { href: '/features', label: 'Features' },
  { href: '/benchmark', label: 'Benchmarks' },
  { href: '/quickstart', label: 'Quick start' },
  { href: '/distribution', label: 'Distribution' },
  { href: '/docs', label: 'Docs' },
  { href: '/releases', label: 'Releases' },
  { href: '/blog', label: 'Journal' },
];

export default function SiteHeader() {
  return (
    <header className="site-header">
      <div className="shell nav-inner">
        <Link className="brand" href="/" aria-label="open-review home">
          <span className="brand-mark" aria-hidden="true">
            /
          </span>
          <span>open-review</span>
        </Link>
        <nav className="main-nav" aria-label="Main navigation">
          {navigation.map((item) => (
            <Link href={item.href} key={item.href}>
              {item.label}
            </Link>
          ))}
        </nav>
        <div className="nav-actions">
          <a className="nav-source" href={repository} target="_blank" rel="noreferrer">
            View source <span aria-hidden="true">↗</span>
          </a>
          <details className="mobile-nav">
            <summary aria-label="Open menu">
              Menu <span aria-hidden="true">+</span>
            </summary>
            <nav aria-label="Mobile navigation">
              {navigation.map((item) => (
                <Link href={item.href} key={item.href}>
                  {item.label}
                </Link>
              ))}
            </nav>
          </details>
        </div>
      </div>
    </header>
  );
}
