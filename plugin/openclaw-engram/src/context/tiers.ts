export type ContextTier = 'FULL' | 'TARGETED' | 'MINIMAL' | 'NONE';

export interface TierConfig {
  /** How often to do a FULL search (every N turns). Default: 8. */
  fullSearchInterval: number;
  /** How often to do a MINIMAL search if no other trigger. Default: 4. */
  minimalInterval: number;
  /** Maximum prompt length (chars) to be classified as NONE. Default: 30. */
  shortPromptMaxChars: number;
}

export const DEFAULT_TIER_CONFIG: TierConfig = {
  fullSearchInterval: 8,
  minimalInterval: 4,
  shortPromptMaxChars: 30,
};

export interface TierResult {
  tier: ContextTier;
  tokenBudget: number;
  reason: string;
}

const TOKEN_BUDGETS: Record<ContextTier, number> = {
  FULL: 2000,
  TARGETED: 1000,
  MINIMAL: 500,
  NONE: 0,
};

const RECALL_PHRASES = [
  'remember',
  'recall',
  'what do you know about',
  'prior decision',
  'previous session',
  'what was decided',
  'engram',
  'memory',
];

const STOP_WORDS = new Set([
  'the', 'this', 'that', 'with', 'from', 'have', 'been',
  'will', 'would', 'could', 'should', 'about', 'into',
  'what', 'when', 'where', 'which', 'there', 'their',
  'also', 'just', 'more', 'some', 'than', 'them', 'then',
  'very', 'your', 'make', 'like', 'does', 'each', 'only',
  'need', 'want', 'please', 'can', 'are', 'for', 'and',
  'but', 'not', 'you', 'all', 'any', 'her', 'was', 'one',
  'our', 'out', 'has', 'had', 'how', 'its', 'may', 'new',
  'now', 'old', 'see', 'way', 'who', 'did', 'get', 'let',
  'say', 'she', 'too', 'use',
]);

function extractSignificantWords(text: string): Set<string> {
  return new Set(
    text
      .toLowerCase()
      .split(/\W+/)
      .filter((w) => w.length > 3 && !STOP_WORDS.has(w)),
  );
}

function jaccardSimilarity(a: Set<string>, b: Set<string>): number {
  if (a.size === 0 && b.size === 0) return 1;
  const intersection = new Set([...a].filter((w) => b.has(w)));
  const union = new Set([...a, ...b]);
  return union.size === 0 ? 1 : intersection.size / union.size;
}

function isTopicShift(current: string, previous: string): boolean {
  if (!previous) return false;
  const currentWords = extractSignificantWords(current);
  const previousWords = extractSignificantWords(previous);
  return jaccardSimilarity(currentWords, previousWords) < 0.3;
}

function makeTier(tier: ContextTier, reason: string): TierResult {
  return { tier, tokenBudget: TOKEN_BUDGETS[tier], reason };
}

export class TurnTracker {
  private turnCount = 0;
  private lastPrompt = '';
  private readonly config: TierConfig;

  constructor(config?: Partial<TierConfig>) {
    this.config = { ...DEFAULT_TIER_CONFIG, ...config };
  }

  /** Classify the current turn and return the appropriate tier. */
  classify(prompt: string): TierResult {
    this.turnCount++;

    const lowerPrompt = prompt.toLowerCase();
    const hasRecallPhrase = RECALL_PHRASES.some((phrase) => lowerPrompt.includes(phrase));

    // Rule 1: First turn always gets full context.
    if (this.turnCount === 1) {
      this.lastPrompt = prompt;
      return makeTier('FULL', 'first turn');
    }

    // Rule 2: Explicit recall phrases trigger FULL.
    if (hasRecallPhrase) {
      this.lastPrompt = prompt;
      return makeTier('FULL', 'explicit recall');
    }

    // Rule 3: Short prompts without a question mark → NONE.
    if (prompt.length <= this.config.shortPromptMaxChars && !prompt.includes('?')) {
      this.lastPrompt = prompt;
      return makeTier('NONE', 'short response');
    }

    // Rule 4: Periodic FULL.
    if (this.turnCount % this.config.fullSearchInterval === 0) {
      this.lastPrompt = prompt;
      return makeTier('FULL', 'periodic full');
    }

    // Rule 5: Periodic MINIMAL.
    if (this.turnCount % this.config.minimalInterval === 0) {
      this.lastPrompt = prompt;
      return makeTier('MINIMAL', 'periodic minimal');
    }

    // Rule 6: Topic shift → TARGETED.
    if (isTopicShift(prompt, this.lastPrompt)) {
      this.lastPrompt = prompt;
      return makeTier('TARGETED', 'topic shift');
    }

    // Default: no trigger.
    this.lastPrompt = prompt;
    return makeTier('NONE', 'no trigger');
  }

  /** Reset tracker (e.g., on new session). */
  reset(): void {
    this.turnCount = 0;
    this.lastPrompt = '';
  }
}
