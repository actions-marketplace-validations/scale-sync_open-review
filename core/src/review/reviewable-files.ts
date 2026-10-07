const SKIP_PATTERNS: RegExp[] = [
  /^yarn\.lock$/,
  /^pnpm-lock\.yaml$/,
  /^package-lock\.json$/,
  /\.lock$/,
  /\.min\.(js|css)$/,
  /\.map$/,
  /\.snap$/,
  /\.svg$/,
  /\.png$/,
  /\.jpg$/,
  /\.jpeg$/,
  /\.gif$/,
  /\.ico$/,
  /\.woff2?$/,
  /\.ttf$/,
  /\.eot$/,
  /\.pdf$/,
  /dist\//,
  /\.generated\./,
  /vendor\//,
];

export function isReviewableFile(filename: string): boolean {
  return !SKIP_PATTERNS.some((pattern) => pattern.test(filename));
}

export function isContextCandidate(filename: string): boolean {
  return (
    isReviewableFile(filename) &&
    !filename.startsWith('/') &&
    filename
      .split('/')
      .every(
        (part) =>
          part !== '' &&
          part !== '.' &&
          part !== '..' &&
          !part.startsWith('.env') &&
          !['node_modules', 'vendor', 'dist'].includes(part),
      ) &&
    /\.(ts|tsx|js|jsx|go|rs|py|java|kt|md|txt|json|ya?ml|toml|sh|sql|html|css)$/.test(filename)
  );
}

export function filterReviewableFiles(files: string[]): string[] {
  return files.filter(isReviewableFile).sort();
}
