import { createHash } from 'node:crypto';
import { readFileSync, readdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';

const [directory, version] = process.argv.slice(2);
if (!directory || !/^v\d+\.\d+\.\d+$/.test(version ?? '')) {
  throw new Error('usage: node manifest.mjs DIRECTORY v1.12.0');
}
const assets = readdirSync(directory)
  .filter((name) => name.startsWith(`open-review_${version}_`) && /\.(tar\.gz|zip)$/.test(name))
  .sort()
  .map((name) => {
    const [, os, arch] = name.match(/^open-review_v\d+\.\d+\.\d+_([^_]+)_([^_.]+)\./) ?? [];
    if (!os || !arch) throw new Error(`invalid asset name ${name}`);
    return {
      name,
      os,
      arch,
      sha256: createHash('sha256').update(readFileSync(path.join(directory, name))).digest('hex'),
    };
  });
if (assets.length !== 5) throw new Error(`expected five binary assets, found ${assets.length}`);
const manifest = {
  version,
  contractVersion: 2,
  sdk: { go: version, typescript: '1.11.0', python: '1.11.0', rust: '1.12.0' },
  assets,
};
writeFileSync(path.join(directory, `manifest-${version}.json`), `${JSON.stringify(manifest, null, 2)}\n`);
writeFileSync(path.join(directory, `SHA256SUMS-${version}`),
  assets.map(({ name, sha256 }) => `${sha256}  ${name}`).join('\n') + '\n');
