import { readFileSync } from 'node:fs';
import path from 'node:path';

export type Release = {
  version: string;
  name: string;
  date: string;
  features: string;
  fixes: string;
  impact: string;
};

function cleanCell(value: string) {
  return value.trim().replaceAll('`', '');
}

function readReleaseLedger(): Release[] {
  const source = readFileSync(path.join(process.cwd(), '..', 'RELEASES.md'), 'utf8');

  return source
    .split('\n')
    .filter((line) => line.startsWith('| `v'))
    .map((line) => {
      const cells = line.split('|').slice(1, -1).map(cleanCell);
      const [release, date, features, fixes, impact] = cells;
      const [version, name = ''] = release.split(' — ');

      return { version, name, date, features, fixes, impact };
    });
}

export const releases = readReleaseLedger();
