import { z } from 'zod';
import { execa } from 'execa';
import type { Tool } from './types';

const BashParams = z.object({
  command: z.string().min(1),
  cwd: z.string().optional(),
});

export const bashTool: Tool = {
  name: 'bash',
  description: 'Execute a shell command and return the output. Use for running commands, git, tests, etc.',
  parameters: BashParams,
  async execute(args) {
    const { command, cwd } = BashParams.parse(args);

    try {
      const result = await execa(command, {
        cwd: cwd || process.cwd(),
        shell: true,
        timeout: 120_000, // 2 minutes
        reject: false, // Don't throw on non-zero exit
      });

      let output = '';
      if (result.exitCode !== 0) {
        output += `[Command failed with exit code ${result.exitCode}]\n`;
      }
      if (result.stdout) output += `stdout:\n${result.stdout}\n`;
      if (result.stderr) output += `stderr:\n${result.stderr}\n`;

      return output.trim() || 'Command completed with no output.';
    } catch (err: any) {
      if (err.timedOut) {
        return 'Error: Command timed out after 2 minutes.';
      }
      return `Error executing command: ${err.message}`;
    }
  },
};
