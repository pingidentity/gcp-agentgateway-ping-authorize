const AGENT_BACKEND_URL = import.meta.env.VITE_PING_STORE_AGENT_URL as string;

/**
 * Sends a user message to the agent backend for processing.
 *
 * The agent backend receives the subject token (user's AIC access token with
 * may_act claim), performs RFC 8693 token exchange to obtain a delegated token,
 * then invokes MCP tools on the load balancer on behalf of the user.
 *
 * The backend maintains agent sessions keyed by subject token, so conversation
 * history is preserved across calls for the lifetime of the token.
 *
 * @param message - The user's chat message.
 * @param subjectToken - The user's PingOne AIC access token (subject token).
 * @returns The agent's text response.
 */
export async function invokePingStoreAgent(message: string, subjectToken: string): Promise<string> {
  const response = await fetch(`${AGENT_BACKEND_URL}/chat`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${subjectToken}`,
    },
    body: JSON.stringify({ message }),
  });

  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(body.error || `Agent request failed: ${response.status}`);
  }

  const { response: agentReply } = await response.json();
  return agentReply;
}
