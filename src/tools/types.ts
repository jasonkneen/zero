import { z } from 'zod';

export interface ToolExecutionEffects {
  planUpdated?: boolean;
}

export interface Tool {
  name: string;
  description: string;
  parameters: z.ZodObject<any>; // Zod schema for validation
  execute: (args: any) => Promise<string>; // Returns tool result as string
  onAfterExecute?: (result: string) => ToolExecutionEffects | void;
}

export interface ToolCall {
  id: string;
  name: string;
  arguments: string; // raw JSON string from model
}

export interface ToolResult {
  toolCallId: string;
  result: string;
}
