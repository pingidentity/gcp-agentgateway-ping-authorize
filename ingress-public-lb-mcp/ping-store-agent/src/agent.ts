import { Agent, McpClient, Message } from "@strands-agents/sdk";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import { exchangeDelegatedToken } from "./auth.js";
import { LB_URL, LLM_MODEL, SYSTEM_PROMPT } from "./config.js";

/** In-memory conversation history for the agent keyed by user sub claim for multi-turn support. */
const agentConversationHistory = new Map<string, Message[]>();

/**
 * Creates a Strands agent connected to the stripe-mcp server via the regional load balancer.
 * The delegated token (with sub + act claims) is passed as a Bearer token
 * so PingAuthorize at the load balancer can enforce per-user, per-agent policies.
 */
const createStoreAgent = async (delegatedToken: string, messages: Message[]): Promise<Agent> => {
  const transport = new StreamableHTTPClientTransport(new URL(`${LB_URL}/mcp`), {
    requestInit: {
      headers: { Authorization: `Bearer ${delegatedToken}` },
    },
  });

  return new Agent({
    model: LLM_MODEL,
    tools: [new McpClient({ transport })],
    systemPrompt: SYSTEM_PROMPT,
    ...(messages.length ? { messages } : {}),
  });
};

/** Exchanges the user's token for a delegated token and returns an agent with prior conversation history. */
export const getOrCreateAgentSession = async (userId: string, subjectToken: string): Promise<Agent> => {
  const { access_token } = await exchangeDelegatedToken(subjectToken);
  const history = agentConversationHistory.get(userId) ?? [];
  return createStoreAgent(access_token, history);
};

/** Persist the agent's conversation history for the next request. */
export const saveAgentConversation = (userId: string, messages: Message[]): void => {
  agentConversationHistory.set(userId, messages);
};
