import { createHash } from "node:crypto";

// Stage 1: Regex-based patterns for known secret formats
const SECRET_PATTERNS: RegExp[] = [
  // AWS access key
  /AKIA[0-9A-Z]{16}/g,
  // JWT tokens (header.payload.signature, all base64url)
  /eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/g,
  // GitHub tokens
  /gh[ps]_[A-Za-z0-9]{36,}/g,
  // GitLab tokens
  /glpat-[A-Za-z0-9_-]{20,}/g,
  // npm tokens
  /npm_[A-Za-z0-9]{36,}/g,
  // OpenAI API keys (sk- but not sk-ant-)
  /sk-(?!ant-)[A-Za-z0-9]{20,}/g,
  // Anthropic API keys
  /sk-ant-[A-Za-z0-9_-]{20,}/g,
  // SSH private keys (multiline)
  /-----BEGIN [A-Z ]+ PRIVATE KEY-----[\s\S]+?-----END [A-Z ]+ PRIVATE KEY-----/g,
  // Bearer tokens
  /Bearer\s+[A-Za-z0-9_\-./+=]{20,}/g,
  // Basic auth
  /Basic\s+[A-Za-z0-9+/=]{20,}/g,
  // Connection strings with embedded passwords (redact user:pass@ portion)
  /(?:mysql|postgres|mongodb|redis):\/\/[^:]+:[^@]+@/g,
  // Generic key/token/secret/password assignment patterns
  /(?:api[_-]?key|token|secret|password|credential|auth)["'\s:=]+["']?([A-Za-z0-9_\-./+=]{20,})["']?/gi,
];

function sha256Prefix(value: string): string {
  return createHash("sha256").update(value).digest("hex").slice(0, 8);
}

function redactToken(value: string): string {
  return `[REDACTED:${sha256Prefix(value)}]`;
}

// Applies Stage 1 regex heuristics to a text, returning the redacted result.
function applyRegexRedaction(text: string): string {
  let result = text;

  for (const pattern of SECRET_PATTERNS) {
    // Reset lastIndex for global regexes between calls
    pattern.lastIndex = 0;

    result = result.replace(pattern, (match, captureGroup?: string) => {
      // For generic key patterns the capture group holds the value portion
      if (captureGroup !== undefined) {
        const valueStart = match.lastIndexOf(captureGroup);
        if (valueStart !== -1) {
          const prefix = match.slice(0, valueStart);
          return `${prefix}${redactToken(captureGroup)}`;
        }
      }
      return redactToken(match);
    });
  }

  return result;
}

// Calculates Shannon entropy of a string.
function shannonEntropy(s: string): number {
  const freq = new Map<string, number>();
  for (const c of s) {
    freq.set(c, (freq.get(c) ?? 0) + 1);
  }
  let entropy = 0;
  for (const count of freq.values()) {
    const p = count / s.length;
    entropy -= p * Math.log2(p);
  }
  return entropy;
}

// Heuristics to skip tokens that are clearly not secrets.
function looksLikeFilePath(token: string): boolean {
  // Contains '/' and has path-like structure (e.g. /usr/local/bin or ./src/foo)
  return /^\.?\/[^\s]+$/.test(token) || /^[a-z]:[\\\/]/i.test(token);
}

function looksLikeUrl(token: string): boolean {
  return token.startsWith("http://") || token.startsWith("https://");
}

// camelCase or snake_case identifiers with low entropy are likely code.
function looksLikeCodeIdentifier(token: string): boolean {
  const hasWordBoundary = /[a-z][A-Z]/.test(token) || token.includes("_");
  return hasWordBoundary && shannonEntropy(token) <= 4.0;
}

// Applies Stage 2 high-entropy string detection to text that has already
// gone through regex redaction.
function applyEntropyRedaction(text: string): string {
  // Match candidate tokens: alphanumeric + allowed special chars, min 21 chars
  return text.replace(/[A-Za-z0-9_\-./+=]{21,}/g, (token) => {
    // Skip already-redacted placeholders
    if (token.startsWith("[REDACTED:")) {
      return token;
    }
    if (looksLikeUrl(token) || looksLikeFilePath(token)) {
      return token;
    }
    if (looksLikeCodeIdentifier(token)) {
      return token;
    }
    if (shannonEntropy(token) > 4.5) {
      return redactToken(token);
    }
    return token;
  });
}

/**
 * Redacts secrets from text before it is sent to the engram server.
 *
 * Applies two stages:
 *   1. Regex heuristics for known secret formats (AWS keys, JWTs, GitHub
 *      tokens, etc.)
 *   2. High-entropy string detection for unknown secrets
 *
 * Conservative by design: prefers missing an occasional secret over
 * redacting valid code identifiers.
 */
export function redactSecrets(text: string): string {
  const afterRegex = applyRegexRedaction(text);
  return applyEntropyRedaction(afterRegex);
}

/**
 * Returns true when redactSecrets would alter the provided text, i.e. when
 * the text appears to contain at least one secret.
 */
export function containsSecrets(text: string): boolean {
  return redactSecrets(text) !== text;
}
