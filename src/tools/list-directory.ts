import { z } from 'zod';
import { readdir, stat } from 'fs/promises';
import { join, relative } from 'path';
import type { Tool } from './types';

const ListDirectoryParams = z.object({
  path: z.string().optional().describe('Directory to list. Defaults to current working directory.'),
  recursive: z.boolean().optional().default(false).describe('Whether to list recursively'),
  max_depth: z.number().int().min(1).max(5).optional().default(2).describe('Max recursion depth when recursive=true'),
});

export const listDirectoryTool: Tool = {
  name: 'list_directory',
  description: `List files and directories in a given path. Use this when you need to explore the project structure.

This is the preferred tool for understanding the codebase layout before planning or making changes.
- Returns a clean tree-like structure
- Supports recursive listing with depth control
- Ignores common junk directories (.git, node_modules, dist, etc.) by default`,
  parameters: ListDirectoryParams,
  async execute(args) {
    const { path = '.', recursive, max_depth } = ListDirectoryParams.parse(args);

    const ignoreDirs = new Set([
      '.git', 'node_modules', 'dist', 'build', '.next', '.turbo', 'coverage',
      '.cache', 'tmp', 'temp', '.DS_Store',
    ]);

    try {
      const root = process.cwd();
      const targetPath = join(root, path);

      const entries = await listDirRecursive(targetPath, root, 0, recursive ? max_depth : 0, ignoreDirs);

      if (entries.length === 0) {
        return `Directory is empty: ${path}`;
      }

      return `Contents of ${path}:\n\n${entries.join('\n')}`;
    } catch (err: any) {
      return `Error listing directory ${path}: ${err.message}`;
    }
  },
};

async function listDirRecursive(
  currentPath: string,
  root: string,
  currentDepth: number,
  maxDepth: number,
  ignoreDirs: Set<string>,
): Promise<string[]> {
  const results: string[] = [];

  try {
    const dirents = await readdir(currentPath, { withFileTypes: true });

    for (const dirent of dirents) {
      const fullPath = join(currentPath, dirent.name);
      const relPath = relative(root, fullPath);

      if (dirent.isDirectory() || dirent.isSymbolicLink()) {
        if (ignoreDirs.has(dirent.name)) continue;

        // For symlinks, verify it's actually a directory
        if (dirent.isSymbolicLink()) {
          try {
            const linkStat = await stat(fullPath);
            if (!linkStat.isDirectory()) {
              // Symlink points to a file, treat as file
              const indent = '  '.repeat(currentDepth);
              results.push(`${indent}📄 ${dirent.name} -> symlink`);
              continue;
            }
          } catch {
            // Broken symlink
            const indent = '  '.repeat(currentDepth);
            results.push(`${indent}❓ ${dirent.name} -> broken symlink`);
            continue;
          }
        }

        const indent = '  '.repeat(currentDepth);
        results.push(`${indent}📁 ${dirent.name}/`);

        if (currentDepth < maxDepth) {
          const children = await listDirRecursive(fullPath, root, currentDepth + 1, maxDepth, ignoreDirs);
          results.push(...children);
        }
      } else {
        // Skip ignored files
        if (ignoreDirs.has(dirent.name)) continue;

        const indent = '  '.repeat(currentDepth);
        results.push(`${indent}📄 ${dirent.name}`);
      }
    }
  } catch (e: any) {
    // Only ignore permission errors; log others
    if (e.code === 'EACCES' || e.code === 'EPERM') {
      // Permission denied - skip silently
    } else {
      console.warn(`[list-directory] Error reading ${currentPath}: ${e.message}`);
    }
  }

  return results;
}
