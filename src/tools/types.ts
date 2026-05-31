import { z } from 'zod';
import type { ToolCall } from '../providers/types';

export type { ToolCall };

export interface Tool {
  name: string;
  description: string;
  parameters: z.ZodObject<any>; // Zod schema for validation
  execute: (args: any) => Promise<string>; // Returns tool result as string
}

export interface ToolResult {
  toolCallId: string;
  result: string;
}
