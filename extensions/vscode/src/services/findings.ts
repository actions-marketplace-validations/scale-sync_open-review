import * as path from 'node:path';

export function findingPath(root: string, file: string): string | undefined {
  const absolute = path.resolve(root, file);
  const relative = path.relative(root, absolute);
  return relative && relative !== '..' && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative) ? absolute : undefined;
}
