import React, { useRef, useState } from 'react';
import { Box, Text, useApp, useInput } from 'ink';
import { ProviderPicker } from './ProviderPicker';
import { AddProvider } from './AddProvider';
import { Logo } from './Logo';
import { ThinkingSpinner } from './Spinner';
import { MessageRenderer } from './MessageRenderer';
import { ToolCallRenderer } from './ToolCallRenderer';
import { Header } from './Header';
import { TodoRail } from './TodoRail';
import { configManager } from '../config/manager';
import { loadProviderConfig } from '../config/provider';
import { OpenAIProvider } from '../providers/openai';
import { runAgent } from '../agent/loop';
import type { PlanItem } from '../tools/plan';

type Screen = 'chat' | 'provider-picker' | 'add-provider';

type Usage = {
  promptTokens: number;
  completionTokens: number;
};

type ProviderHeaderInfo = {
  name: string;
  model: string;
};

function getProviderHeaderInfo(): ProviderHeaderInfo {
  const activeProfile = configManager.getActiveProvider();
  return {
    name: activeProfile?.name || (process.env.ZERO_PROVIDER_COMMAND ? 'command' : 'env'),
    model: activeProfile?.model || process.env.OPENAI_MODEL || 'default',
  };
}

// Map low-level errors back to actionable guidance for the user. The full
// error object is still surfaced separately when debug mode is on.
function toFriendlyError(err: any): string {
  const raw = err?.message || String(err);
  const lower = raw.toLowerCase();

  if (lower.includes('no llm provider configured') || lower.includes('no provider')) {
    return 'No provider set up. Type /provider to add one.';
  }

  if (lower.includes('auth') || lower.includes('unauthorized') || lower.includes('invalid') || lower.includes('401') || lower.includes('api key')) {
    return `Authentication failed — check your API key. Type /provider to update it.\n(${raw})`;
  }

  if (lower.includes('rate') || lower.includes('quota')) {
    return `Provider rate limit or quota reached. Try again shortly.\n(${raw})`;
  }

  if (lower.includes('enotfound') || lower.includes('econnrefused') || lower.includes('etimedout') || lower.includes('fetch failed') || lower.includes('network')) {
    return `Network error reaching the provider. Check your connection and base URL.\n(${raw})`;
  }

  return `Error: ${raw}`;
}

type ChatMessage =
  | { type: 'user'; content: string }
  | { type: 'assistant'; content: string }
  | { type: 'tool-call'; name: string; args: string; result?: string }
  | { type: 'tool-result'; content: string } // legacy - results now attach to tool-call
  | { type: 'system'; content: string };

export const App: React.FC = () => {
  const { exit } = useApp();
  const [screen, setScreen] = useState<Screen>('chat');
  const [input, setInput] = useState('');
  const [messages, setMessages] = useState<ChatMessage[]>([
    { type: 'system', content: 'Welcome to zero. Type /provider to manage providers.' },
    { type: 'system', content: 'Type /help for available commands.' },
  ]);

  // Check on startup if we have any usable provider
  React.useEffect(() => {
    const checkProvider = async () => {
      try {
        await loadProviderConfig();
      } catch (err: any) {
        if (err.message?.includes('No LLM provider configured')) {
          setMessages((prev) => [
            ...prev,
            { 
              type: 'system', 
              content: '⚠️  No provider configured yet. Use /provider to add one (OpenGateway recommended).' 
            }
          ]);
        }
      }
    };
    checkProvider();
  }, []);
  const [isThinking, setIsThinking] = useState(false);
  const [streamingMessageIndex, setStreamingMessageIndex] = useState<number | null>(null);
  const streamingMessageIndexRef = useRef<number | null>(null);

  // Plan Mode (inspired by OpenClaude / Claude Code)
  const [isPlanMode, setIsPlanMode] = useState(false);

  // Debug mode - when enabled, prints full error objects to console
  const [debugMode, setDebugMode] = useState(false);
  const [lastError, setLastError] = useState<any>(null);

  // Tools enabled (useful for debugging provider errors)
  const [toolsEnabled, setToolsEnabled] = useState(true);
  const [usage, setUsage] = useState<Usage>({ promptTokens: 0, completionTokens: 0 });
  const [plan, setPlan] = useState<PlanItem[]>([]);
  const [todoRailOpen, setTodoRailOpen] = useState(false);
  const [todoRailHidden, setTodoRailHidden] = useState(false);
  const planLengthRef = useRef(0);
  const todoRailHiddenRef = useRef(false);
  const [providerHeaderInfo, setProviderHeaderInfo] = useState<ProviderHeaderInfo>(() => getProviderHeaderInfo());

  // Command suggestions
  const [suggestions, setSuggestions] = useState<string[]>([]);

  const knownCommands = ['/provider', '/plan', '/todo', '/debug-mode', '/debug', '/tools', '/help', '/exit', '/quit'];

  // Update suggestions when input changes
  React.useEffect(() => {
    if (input.startsWith('/')) {
      const query = input.toLowerCase();
      const matches = knownCommands.filter(cmd => cmd.startsWith(query));
      setSuggestions(matches.slice(0, 6)); // limit suggestions
    } else {
      setSuggestions([]);
    }
  }, [input]);

  // Current provider info for the input bar (Grok Build style)
  const currentProviderName = providerHeaderInfo.name;
  const currentModel = providerHeaderInfo.model;
  const totalTokens = usage.promptTokens + usage.completionTokens;
  const hasPlanItems = plan.length > 0;
  const hasActivePlanItems = plan.some((item) => item.status !== 'completed');
  const showTodoRail = todoRailOpen || (hasPlanItems && hasActivePlanItems && !todoRailHidden);

  // Only capture main chat input when we're actually in the chat screen
  const isInChat = screen === 'chat';

  useInput((inputChar, key) => {
    if (key.ctrl && inputChar === 'c') {
      exit();
      return;
    }

    // Don't process chat input while in provider picker or add flow
    if (!isInChat) return;

    if (!input && key.ctrl && inputChar === 't') {
      toggleTodoRail();
      return;
    }

    if (key.return) {
      handleSubmit();
      return;
    }

    // Autocomplete first suggestion with Tab when typing a command
    if (key.tab && suggestions.length > 0) {
      setInput(suggestions[0] + ' ');
      setSuggestions([]);
      return;
    }

    if (key.backspace || key.delete) {
      setInput((prev) => prev.slice(0, -1));
      return;
    }

    if (inputChar && !key.ctrl && !key.meta) {
      setInput((prev) => prev + inputChar);
    }
  }, { isActive: isInChat });

  const handleSubmit = () => {
    if (!input.trim()) return;

    const trimmed = input.trim();
    setInput('');
    setSuggestions([]);

    // Handle slash commands
    if (trimmed.startsWith('/')) {
      setMessages((prev) => [...prev, { type: 'user', content: trimmed }]);
      handleSlashCommand(trimmed);
      return;
    }

    // Regular message → send to agent
    setMessages((prev) => [...prev, { type: 'user', content: trimmed }]);

    const runAgentLoop = async () => {
      setIsThinking(true);

      try {
        const providerConfig = await loadProviderConfig();
        const provider = new OpenAIProvider({
          apiKey: providerConfig.apiKey || '',
          baseURL: providerConfig.baseURL,
          model: providerConfig.model,
        });

        // Add empty assistant message that we'll stream into
        setMessages((prev) => {
          const newMessages = [...prev, { type: 'assistant' as const, content: '' }];
          const nextIndex = newMessages.length - 1;
          streamingMessageIndexRef.current = nextIndex;
          setStreamingMessageIndex(nextIndex);
          return newMessages;
        });

        await runAgent(trimmed, provider, {
          debug: debugMode,
          toolsEnabled,
          planMode: isPlanMode,
          onText: (text: string) => {
            setIsThinking(false);
            setMessages((prev) => {
              const newMessages = [...prev];
              const idx = streamingMessageIndexRef.current ?? newMessages.length - 1;

              if (newMessages[idx]?.type === 'assistant') {
                const current = newMessages[idx] as { type: 'assistant'; content: string };
                newMessages[idx] = {
                  ...current,
                  content: current.content + text,
                };
              }
              return newMessages;
            });
          },
          onToolCall: (tc) => {
            setIsThinking(false);
            setMessages((prev) => [
              ...prev,
              { type: 'tool-call', name: tc.name, args: tc.arguments },
            ]);
          },
          onToolResult: (result) => {
            // Attach result to the most recent tool call that doesn't have one yet
            setMessages((prev) => {
              const newMessages = [...prev];
              for (let i = newMessages.length - 1; i >= 0; i--) {
                const msg = newMessages[i];
                if (msg && msg.type === 'tool-call' && (msg as any).result === undefined) {
                  (newMessages as any)[i] = {
                    ...msg,
                    result: result.result,
                  };
                  break;
                }
              }
              return newMessages;
            });
          },
          onUsage: (nextUsage) => {
            setUsage((current) => ({
              promptTokens: current.promptTokens + nextUsage.promptTokens,
              completionTokens: current.completionTokens + nextUsage.completionTokens,
            }));
          },
          onPlanUpdate: (nextPlan) => {
            if (nextPlan.length > 0 && planLengthRef.current === 0 && !todoRailHiddenRef.current) {
              setTodoRailOpen(false);
            }
            planLengthRef.current = nextPlan.length;
            setPlan(nextPlan);
          },
        });
      } catch (err: any) {
        setIsThinking(false);

        if (debugMode) {
          setLastError(err);
          try {
            const red = '\x1b[31m';
            const reset = '\x1b[0m';
            const border = '─'.repeat(50);

            console.error(`\n${red}┌${border}┐`);
            console.error(`│  FULL PROVIDER ERROR${' '.repeat(29)}│`);
            console.error(`├${border}┤`);
            console.error(`│ Message: ${(err?.message || String(err)).slice(0, 40)}${' '.repeat(9)}│`);
            console.error(`│ Name:    ${err?.name || 'Error'}${' '.repeat(42 - (err?.name || 'Error').length)}│`);

            if (err?.response?.status) {
              console.error(`│ Status:  ${err.response.status}${' '.repeat(42 - String(err.response.status).length)}│`);
            }

            console.error(`└${border}┘${reset}`);
            console.error('Full object:');
            console.dir(err, { depth: 6 });
            console.error(`${red}${'='.repeat(52)}${reset}\n`);
          } catch (logErr) {
            console.error('Failed to log full error:', logErr);
          }
        } else {
          setLastError(null);
        }

        const friendlyMessage = toFriendlyError(err);
        setMessages((prev) => [...prev, { type: 'system', content: friendlyMessage }]);
      } finally {
        setIsThinking(false);
        streamingMessageIndexRef.current = null;
        setStreamingMessageIndex(null);
      }
    };

    runAgentLoop();
  };

  const handleSlashCommand = (command: string) => {
    const parts = command.trim().split(/\s+/);
    const cmd = (parts[0] ?? '').toLowerCase();
    const arg = parts[1]?.toLowerCase();

    if (cmd === '/provider') {
      setScreen('provider-picker');
      return;
    }

    if (cmd === '/plan') {
      setIsPlanMode(prev => {
        const next = !prev;
        setMessages((msgs) => [
          ...msgs,
          { 
            type: 'system', 
            content: next 
              ? 'Plan mode enabled. The agent will focus on planning before making changes.' 
              : 'Plan mode disabled.' 
          },
        ]);
        return next;
      });
      return;
    }

    if (cmd === '/debug-mode' || cmd === '/debug') {
      // Support "/debug-mode true", "/debug false", or just toggle
      let nextDebug: boolean;

      if (arg === 'true') nextDebug = true;
      else if (arg === 'false') nextDebug = false;
      else nextDebug = !debugMode;

      setDebugMode(nextDebug);
      if (!nextDebug) setLastError(null);
      setMessages((prev) => [
        ...prev,
        { type: 'system', content: `Debug mode ${nextDebug ? 'enabled' : 'disabled'}.` },
      ]);
      return;
    }

    if (cmd === '/tools') {
      const arg2 = parts[1]?.toLowerCase();
      let nextEnabled: boolean;

      if (arg2 === 'on' || arg2 === 'true') nextEnabled = true;
      else if (arg2 === 'off' || arg2 === 'false') nextEnabled = false;
      else nextEnabled = !toolsEnabled;

      setToolsEnabled(nextEnabled);
      setMessages((prev) => [
        ...prev,
        { type: 'system', content: `Tool calling ${nextEnabled ? 'enabled' : 'disabled'}.` },
      ]);
      return;
    }

    if (cmd === '/todo') {
      const nextVisibility = toggleTodoRail();
      setMessages((prev) => [
        ...prev,
        { type: 'system', content: `Todo rail ${nextVisibility}.` },
      ]);
      return;
    }

    if (cmd === '/help') {
      setMessages((prev) => [
        ...prev,
        { type: 'system', content: 'Available commands:' },
        { type: 'system', content: '  /provider     - Manage LLM providers (fix provider errors here)' },
        { type: 'system', content: '  /plan         - Toggle Plan Mode (agent plans first, makes no edits)' },
        { type: 'system', content: '  /todo         - Show or hide the current plan rail' },
        { type: 'system', content: '  /debug-mode   - Toggle debug mode (prints full errors to console)' },
        { type: 'system', content: '  /tools        - Toggle tool calling (useful for debugging provider errors)' },
        { type: 'system', content: '  /help         - Show this help' },
        { type: 'system', content: '  /exit         - Quit' },
      ]);
      return;
    }

    if (cmd === '/exit' || cmd === '/quit') {
      exit();
      return;
    }

    setMessages((prev) => [...prev, { type: 'system', content: `Unknown command: ${command}` }]);
  };

  const toggleTodoRail = (): 'shown' | 'hidden' => {
    const nextVisibility = showTodoRail ? 'hidden' : 'shown';

    if (showTodoRail) {
      setTodoRailOpen(false);
      todoRailHiddenRef.current = true;
      setTodoRailHidden(true);
    } else {
      todoRailHiddenRef.current = false;
      setTodoRailHidden(false);
      setTodoRailOpen(true);
    }

    return nextVisibility;
  };

  const handleProviderSelected = (name: string) => {
    const success = configManager.setActiveProvider(name);
    if (success) {
      setProviderHeaderInfo(getProviderHeaderInfo());
      setMessages((prev) => [...prev, { type: 'system', content: `Switched to provider: ${name}` }]);
    }
    setScreen('chat');
  };

  const handleProviderPickerCancel = () => {
    setScreen('chat');
  };

  const handleOpenAddProvider = () => {
    setScreen('add-provider');
  };

  const handleAddProviderDone = (providerName?: string) => {
    setScreen('chat');

    if (providerName) {
      // Automatically switch to the newly added provider
      const switched = configManager.setActiveProvider(providerName);

      if (switched) {
        setProviderHeaderInfo(getProviderHeaderInfo());
        setMessages((prev) => [
          ...prev,
          { type: 'system', content: `Added and switched to provider: ${providerName}` },
        ]);
      } else {
        setMessages((prev) => [
          ...prev,
          { type: 'system', content: `Provider added: ${providerName}` },
        ]);
      }
    } else {
      setMessages((prev) => [...prev, { type: 'system', content: 'Provider added successfully.' }]);
    }
  };

  const handleAddProviderCancel = () => {
    setScreen('provider-picker');
  };

  if (screen === 'add-provider') {
    return (
      <AddProvider
        onDone={handleAddProviderDone}
        onCancel={handleAddProviderCancel}
      />
    );
  }

  if (screen === 'provider-picker') {
    return (
      <ProviderPicker
        onSelect={handleProviderSelected}
        onCancel={handleProviderPickerCancel}
        onAddNew={handleOpenAddProvider}
      />
    );
  }

  const showLogo = messages.length <= 2;

  return (
    <Box flexDirection="column" height="100%">
      <Header
        provider={currentProviderName}
        model={currentModel}
        usage={usage}
        totalTokens={totalTokens}
        planMode={isPlanMode}
      />

      {/* Message area; render the transcript directly so terminal scrollback stays useful. */}
      <Box 
        flexGrow={1} 
        flexDirection="row"
      >
        {/* Main chat content */}
        <Box 
          flexGrow={1} 
          flexDirection="column" 
          paddingX={1} 
          paddingTop={1}
        >
        {showLogo && <Logo />}

        <Box flexDirection="column">
          {messages.map((msg, realIndex) => {
            if (msg.type === 'user') {
              return (
                <Box key={realIndex} marginBottom={1}>
                  <Text color="blueBright">
                    {`> ${msg.content}`}
                  </Text>
                </Box>
              );
            }

            if (msg.type === 'assistant') {
              const isStreaming = realIndex === streamingMessageIndex;
              return (
                <Box key={realIndex} marginBottom={1} flexDirection="row">
                  <Text color="cyan" dimColor>● </Text>
                  <Box flexDirection="column" flexGrow={1}>
                    <MessageRenderer content={msg.content} />
                    {isStreaming && (
                      <Text color="cyan" dimColor>▌</Text>
                    )}
                  </Box>
                </Box>
              );
            }

            if (msg.type === 'tool-call') {
              const hasResult = !!msg.result;
              return (
                <Box key={realIndex} marginBottom={0}>
                  <ToolCallRenderer
                    name={msg.name}
                    args={msg.args}
                    result={msg.result}
                    status={hasResult ? 'success' : 'running'}
                  />
                </Box>
              );
            }

            if (msg.type === 'tool-result') {
              // Legacy separate results are no longer created; ignore for cleanliness
              return null;
            }

            // system messages
            return (
              <Box key={realIndex} marginBottom={1}>
                <Text color="gray" dimColor>
                  {msg.content}
                </Text>
              </Box>
            );
          })}

          {isThinking && <ThinkingSpinner />}
        </Box>
        </Box>

        {showTodoRail && (
          <TodoRail plan={plan} />
        )}
      </Box>

      {/* Command suggestions */}
      {suggestions.length > 0 && (
        <Box paddingX={2} paddingBottom={0}>
          <Text color="gray" dimColor>
            Suggestions: {suggestions.map((s, i) => (
              <Text key={i} color={i === 0 ? 'cyan' : 'gray'}>{s}{i < suggestions.length - 1 ? '  ' : ''}</Text>
            ))} (Tab to autocomplete)
          </Text>
        </Box>
      )}

      {/* Debug error box */}
      {debugMode && lastError && (
        <Box 
          borderStyle="single" 
          borderColor="red" 
          paddingX={1} 
          paddingY={0} 
          marginBottom={1}
        >
          <Text color="red" bold>⚠ Debug Error</Text>
          <Text color="gray" dimColor>
            {lastError.message || String(lastError)}
          </Text>
          {lastError.stack && (
            <Text color="gray" dimColor>
              {lastError.stack.split('\n').slice(0, 8).join('\n')}
            </Text>
          )}
          <Text color="cyan" dimColor>
            (Full details in terminal • /debug-mode false to hide)
          </Text>
        </Box>
      )}

      {/* Input box at the bottom */}
      <Box
        borderStyle="single"
        borderColor={isPlanMode ? 'green' : 'gray'}
        paddingX={1}
        paddingY={0}
        flexDirection="row"
        justifyContent="space-between"
        alignItems="center"
      >
        {/* Left: prompt + input */}
        <Box flexDirection="row">
          <Text color={isPlanMode ? 'green' : 'greenBright'}>› </Text>
          <Text color="white">{input}</Text>
          <Text color="gray">█</Text>
        </Box>

        {/* Right: Current provider + model */}
        <Box flexDirection="row">
          <Text color="cyan" bold>{currentProviderName}</Text>
          <Text color="gray"> • </Text>
          <Text color="magenta" dimColor>{currentModel}</Text>
        </Box>
      </Box>

      {/* Very subtle status line */}
      <Box paddingX={1} flexDirection="row">
        <Text color="gray" dimColor>
          /help • /todo or Ctrl+T todo • Ctrl+C exit
        </Text>
        {isPlanMode && (
          <Text color="green"> • PLAN MODE</Text>
        )}
      </Box>
    </Box>
  );
};

