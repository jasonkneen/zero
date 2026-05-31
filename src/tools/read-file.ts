import { z } from 'zod';
import { readFile, stat } from 'fs/promises';
import type { Tool } from './types';

const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10MB

const ReadFileParams = z.object({
  path: z.string().min(1),
});

export const readFileTool: Tool = {
  name: 'read_file',
  description: 'Read the contents of a file from the filesystem.',
  parameters: ReadFileParams,
  async execute(args) {
    const { path } = ReadFileParams.parse(args);

    try {
      const fileStat = await stat(path);
      if (fileStat.size > MAX_FILE_SIZE) {
        return `Error: File ${path} is too large (${(fileStat.size / 1024 / 1024).toFixed(1)}MB). Maximum size is 10MB.`;
      }

      // Check for binary content by reading first 8KB
      const buffer = Buffer.alloc(Math.min(8192, fileStat.size));
      const file = await import('fs/promises').then(fs => fs.open(path, 'r'));
      await file.read(buffer, 0, buffer.length, 0);
      await file.close();

      // If file contains null bytes, it's likely binary
      if (buffer.includes(0)) {
        return `Error: File ${path} appears to be binary and cannot be read as text.`;
      }

      const content = await readFile(path, 'utf-8');
      return `File: ${path}\n\n${content}`;
    } catch (err: any) {
      return `Error reading file ${path}: ${err.message}`;
    }
  },
};
