import type { Metadata } from 'next';

import './globals.css';
import SiteFooter from '../components/SiteFooter';
import SiteHeader from '../components/SiteHeader';

export const metadata: Metadata = {
  title: 'open-review | Code review with context',
  description:
    'Open source code review for the changes that matter. Run locally or in GitHub Actions.',
  icons: {
    icon: '/brand/open-review-mark-32.png',
    apple: '/brand/open-review-mark-180.png',
  },
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>
        <SiteHeader />
        {children}
        <SiteFooter />
      </body>
    </html>
  );
}
