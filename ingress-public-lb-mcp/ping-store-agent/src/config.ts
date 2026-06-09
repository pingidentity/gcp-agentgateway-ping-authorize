import { OpenAIModel } from "@strands-agents/sdk/openai";
import { requiredEnv } from "./util.js";

export const LB_URL = requiredEnv("LB_URL");
export const CORS_ORIGIN_CHAT_UI_STOREFRONT = requiredEnv("CORS_ORIGIN_CHAT_UI_STOREFRONT");
export const PINGONE_AIC_ISSUER = requiredEnv("PINGONE_AIC_ISSUER");
export const AGENT_PORT = requiredEnv("AGENT_PORT");
export const AGENT_CLIENT_ID = requiredEnv("AGENT_CLIENT_ID");
export const AGENT_CLIENT_SECRET = requiredEnv("AGENT_CLIENT_SECRET");
export const AGENT_REQUIRED_SCOPES = requiredEnv("AGENT_REQUIRED_SCOPES");

export const LLM_MODEL = new OpenAIModel({
  apiKey: requiredEnv("OPENAI_API_KEY"),
  modelId: requiredEnv("OPENAI_MODEL"),
});

export const SYSTEM_PROMPT = `You are a helpful shopping assistant for an online store powered by Stripe.
You can help users browse products, check prices, and make purchases.
Use the available MCP tools to interact with the Stripe catalog and payment system.
Always confirm with the user before initiating any purchase or payment action.
Be concise and friendly.`;
