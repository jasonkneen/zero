import { z } from 'zod';
import { execa } from 'execa';
import type { Tool } from './types';

const GrepParams = z.object({
  pattern: z.string().min(1).describe('Regular expression pattern to search for'),
  path: z.string().optional().describe('Directory or file to search in. Defaults to current working directory.'),
  glob: z.string().optional().describe('Glob pattern to filter files (e.g. "**/*.ts", "*.tsx")'),
  output_mode: z.enum(['content', 'files_with_matches', 'count']).optional().default('content'),
  case_insensitive: z.boolean().optional().describe('Case insensitive search'),
  show_line_numbers: z.boolean().optional().default(true).describe('Show line numbers'),
  head_limit: z.number().int().min(1).optional().default(50).describe('Maximum number of results to return'),
});

export const grepTool: Tool = {
  name: 'grep',
  description: `Fast content search using ripgrep (if available) or fallback.

This is the preferred tool for searching code. Use it instead of running grep via bash when possible.

Features:
- Full regex support
- Filter by glob
- Different output modes (content, files_with_matches, count)
- Automatically respects .gitignore`,
  parameters: GrepParams,
  async execute(args) {
    const params = GrepParams.parse(args);
    const { pattern, path = '.', glob, output_mode, case_insensitive, show_line_numbers, head_limit } = params;

    const cwd = process.cwd();
    const targetPath = path === '.' ? cwd : path;

    // Try ripgrep first (much faster and better)
    try {
      const rgArgs = ['--json', '--no-heading', '--with-filename'];

      if (case_insensitive) rgArgs.push('-i');
      if (show_line_numbers) rgArgs.push('-n');
      if (glob) rgArgs.push('--glob', glob);

      rgArgs.push('-m', String(head_limit));
      rgArgs.push(pattern, targetPath);

      const { stdout } = await execa('rg', rgArgs, {
        cwd,
        reject: false,
      });

      if (!stdout) {
        return output_mode === 'count' ? '0 matches found' : 'No matches found.';
      }

      // Parse ripgrep JSON output
      const lines = stdout.trim().split('\n');
      const results: any[] = [];
      const parseErrors: string[] = [];

      for (const line of lines) {
        try {
          const parsed = JSON.parse(line);
          if (parsed.type === 'match') {
            results.push(parsed);
          }
        } catch {
          parseErrors.push(line);
        }
      }

      if (parseErrors.length > 0 && results.length === 0) {
        return `Error: Failed to parse ripgrep output. Raw output:\n${stdout.slice(0, 500)}`;
      }

      if (results.length === 0) {
        return 'No matches found.';
      }

      if (output_mode === 'count') {
        return `${results.length} matches found (limited to first ${head_limit})`;
      }

      if (output_mode === 'files_with_matches') {
        const files = new Set(results.map(r => r.data.path.text));
        return Array.from(files).slice(0, head_limit).join('\n');
      }

      // content mode
      const formatted = results.slice(0, head_limit).map((r: any) => {
        const file = r.data.path.text;
        const lineNum = r.data.line_number;
        const text = r.data.lines.text.trim();
        return `${file}:${lineNum}: ${text}`;
      });

      return formatted.join('\n');
    } catch (err) {
      // Fallback: use basic grep via bash
      try {
        const grepArgs = ['-r'];
        if (show_line_numbers) grepArgs.push('-n');
        if (case_insensitive) grepArgs.push('-i');
        grepArgs.push('-m', String(head_limit));

        let searchTarget = targetPath;
        if (glob) {
          // Use find to filter by glob pattern
          grepArgs.push('--include', glob);
        }

        grepArgs.push(pattern, searchTarget);

        const { stdout } = await execa('grep', grepArgs, {
          cwd,
          reject: false,
        });

        if (!stdout) {
          return output_mode === 'count' ? '0 matches found' : 'No matches found.';
        }

        const lines = stdout.trim().split('\n');

        if (output_mode === 'count') {
          return `${lines.length} matches found (limited to first ${head_limit})`;
        }

        if (output_mode === 'files_with_matches') {
          const files = new Set(lines.map(l => l.split(':')[0]));
          return Array.from(files).slice(0, head_limit).join('\n');
        }

        return lines.slice(0, head_limit).join('\n');
      } catch {
        return `Search failed: ripgrep (rg) not available and grep fallback failed. Please install ripgrep for best results.`;
      }
    }
  },
};
