import { Agent, McpClient } from "@strands-agents/sdk";
import { OpenAIModel } from "@strands-agents/sdk/openai";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";

const SYSTEM_PROMPT = `You are a helpful shopping assistant for an online store powered by Stripe.
You can help users browse products, check prices, and make purchases.
Use the available MCP tools to interact with the Stripe catalog and payment system.
Always confirm with the user before initiating any purchase or payment action.
Be concise and friendly.`;

/**
 * Creates a Strands agent connected to the MCP server via Agent Gateway.
 *
 * The exchanged token (with sub + act claims) is passed as a Bearer token
 * so PingOne Authorize at the gateway can enforce per-user, per-agent policies.
 */
export async function createStoreAgent(delegatedToken: string): Promise<Agent> {
  const gatewayUrl = process.env.AGENT_GATEWAY_URL;
  if (!gatewayUrl) throw new Error("Missing required env var: AGENT_GATEWAY_URL");

  const apiKey = process.env.OPENAI_API_KEY;
  if (!apiKey) throw new Error("Missing required env var: OPENAI_API_KEY");

  // MCP client connects to the Agent Gateway with the delegated token
  const mcpClient = new McpClient({
    transport: new StreamableHTTPClientTransport(
      new URL(gatewayUrl),
      {
        requestInit: {
          headers: {
            Authorization: `Bearer ${delegatedToken}`,
          },
        },
      },
    ),
  });

  const model = new OpenAIModel({
    apiKey,
    modelId: "gpt-4o",
  });

  const agent = new Agent({
    model,
    tools: [mcpClient],
    systemPrompt: SYSTEM_PROMPT,
  });

  return agent;
}
