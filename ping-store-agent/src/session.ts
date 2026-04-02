/**
 * Agent Session Management
 *
 * Maintains per-user conversation history. A fresh agent and delegated token
 * are created for every request (no token caching), but conversation history
 * is preserved so the agent remembers prior messages.
 */

import { Agent, Message } from "@strands-agents/sdk";
import { exchangeForDelegatedToken } from "./token-exchange.js";
import { createStoreAgent } from "./agent.js";

const conversationHistory = new Map<string, Message[]>();

function sessionKey(token: string): string {
  // Key by user identity (sub claim) extracted from the first part of the token
  return token.slice(0, 32);
}

/**
 * Performs a fresh token exchange, creates a new agent seeded with
 * the user's conversation history, and returns it.
 */
export async function getOrCreateSession(subjectToken: string): Promise<Agent> {
  const key = sessionKey(subjectToken);
  const { access_token } = await exchangeForDelegatedToken(subjectToken);
  const history = conversationHistory.get(key) ?? [];
  const agent = await createStoreAgent(access_token, history);
  return agent;
}

/** Persist the agent's conversation history for the next request. */
export function saveConversation(subjectToken: string, messages: Message[]): void {
  conversationHistory.set(sessionKey(subjectToken), messages);
}
