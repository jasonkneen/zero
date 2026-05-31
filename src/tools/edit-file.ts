import { z } from 'zod';
import { readFile, writeFile } from 'fs/promises';
import type { Tool } from './types';

const EditFileParams = z.object({
  path: z.string().min(1),
  old_string: z.string().min(1),
  new_string: z.string(),
});

export const editFileTool: Tool = {
  name: 'edit_file',
  description: 'Edit a file by replacing one exact string with another. Use this for precise code changes.',
  parameters: EditFileParams,
  async execute(args) {
    const { path, old_string, new_string } = EditFileParams.parse(args);

    // Detect no-op edits
    if (old_string === new_string) {
      return `No-op edit: old_string and new_string are identical in ${path}.`;
    }

    try {
      const content = await readFile(path, 'utf-8');

      if (!content.includes(old_string)) {
        return `Error: Could not find the exact string to replace in ${path}.`;
      }

      // Warn if multiple matches exist
      const matchCount = content.split(old_string).length - 1;
      if (matchCount > 1) {
        return `Error: Found ${matchCount} occurrences of the string in ${path}. Please provide a more specific string to match a single occurrence.`;
      }

      // Use function replacement to avoid $&/$'/$` pattern expansion
      const newContent = content.replace(old_string, () => new_string);
      await writeFile(path, newContent, 'utf-8');

      return `Successfully edited ${path}.`;
    } catch (err: any) {
      return `Error editing file ${path}: ${err.message}`;
    }
  },
};
