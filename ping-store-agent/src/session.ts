/**
 * Agent Session Management
 *
 * Maintains per-user agent sessions keyed by subject token. Each session holds
 * a Strands agent instance (preserving conversation history) and the expiry time
 * of the delegated token. When a session expires, a new token exchange is performed
 * and a fresh agent is created.
 */

import type { Agent } from "@strands-agents/sdk";
import { exchangeForDelegatedToken } from "./token-exchange.js";
import { createStoreAgent } from "./agent.js";

interface AgentSession {
  agent: Agent;
  expiresAt: number;
}

const sessions = new Map<string, AgentSession>();

function sessionKey(token: string): string {
  return token.slice(0, 32);
}

/**
 * Returns an active agent session for the user, creating one if needed.
 * Performs token exchange on first call or when the delegated token expires.
 */
export async function getOrCreateSession(subjectToken: string): Promise<Agent> {
  const key = sessionKey(subjectToken);
  let session = sessions.get(key);

  if (!session || Date.now() > session.expiresAt) {
    const { access_token, expires_in } = await exchangeForDelegatedToken(subjectToken);
    const agent = await createStoreAgent(access_token);
    session = { agent, expiresAt: Date.now() + expires_in * 1000 };
    sessions.set(key, session);
  }

  return session.agent;
}
