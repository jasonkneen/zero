import { createHighlighter } from 'shiki';

export type HighlightedToken = {
  content: string;
  color?: string;
};

export type HighlightedLine = HighlightedToken[];

let highlighterPromise: ReturnType<typeof createHighlighter> | undefined;
const highlightCache = new Map<string, Promise<HighlightedLine[]>>();

function hashCode(input: string) {
  let hash = 0;
  for (let i = 0; i < input.length; i++) {
    hash = ((hash << 5) - hash + input.charCodeAt(i)) | 0;
  }
  return hash.toString(36);
}

export async function highlightCode(code: string, lang = 'text'): Promise<HighlightedLine[]> {
  const cacheKey = `${lang}:${code.length}:${hashCode(code)}`;
  const cached = highlightCache.get(cacheKey);
  if (cached) return cached;

  const promise = tokenizeCode(code, lang);
  highlightCache.set(cacheKey, promise);
  return promise;
}

async function tokenizeCode(code: string, lang: string): Promise<HighlightedLine[]> {
  try {
    highlighterPromise ??= createHighlighter({
      themes: ['github-dark'],
      langs: [
        'bash',
        'css',
        'dockerfile',
        'go',
        'html',
        'javascript',
        'json',
        'jsx',
        'markdown',
        'python',
        'rust',
        'shell',
        'sql',
        'toml',
        'tsx',
        'typescript',
        'yaml',
      ],
    });

    const highlighter = await highlighterPromise;
    const result = highlighter.codeToTokens(code, {
      lang: (lang || 'text') as any,
      theme: 'github-dark',
    });

    return result.tokens.map((line) =>
      line.map((token) => ({
        content: token.content,
        color: token.color,
      })),
    );
  } catch {
    return code.split(/\r?\n/).map((line) => [{ content: line }]);
  }
}
