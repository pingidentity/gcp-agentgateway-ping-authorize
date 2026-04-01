/**
 * Agent Session Management
 *
 * Maintains per-user delegated tokens keyed by subject token. When a token
 * expires, a new token exchange is performed. A fresh agent is created for
 * each request to avoid MCP transport reuse issues.
 */

import type { Agent } from "@strands-agents/sdk";
import { exchangeForDelegatedToken } from "./token-exchange.js";
import { createStoreAgent } from "./agent.js";

interface TokenSession {
  delegatedToken: string;
  expiresAt: number;
}

const sessions = new Map<string, TokenSession>();

function sessionKey(token: string): string {
  return token.slice(0, 32);
}

/**
 * Returns an agent for the user, creating a fresh one each request.
 * The delegated token is cached and reused until it expires.
 */
export async function getOrCreateSession(subjectToken: string): Promise<Agent> {
  const key = sessionKey(subjectToken);
  let session = sessions.get(key);

  if (!session || Date.now() > session.expiresAt) {
    const { access_token, expires_in } = await exchangeForDelegatedToken(subjectToken);
    session = { delegatedToken: access_token, expiresAt: Date.now() + expires_in * 1000 };
    sessions.set(key, session);
  }

  return createStoreAgent(session.delegatedToken);
}
