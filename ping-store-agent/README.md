# Ping Store Agent

Express backend that acts as a **delegated agent** — receives a user's access token, performs [RFC 8693 token exchange](https://datatracker.ietf.org/doc/html/rfc8693), then uses a Strands AI agent to invoke Stripe MCP tools through the GCP Agent Gateway on the user's behalf.

## Token Flow

**Inbound Auth**: `ping-chat-ui-storefront` → subject token (from auth code grant) → `ping-store-agent`  
**Outbound Auth**: `ping-store-agent` → delegated token (from token exchange grant) → GCP Agent Gateway → `stripe-mcp`

## Project Structure

```
src/
├── main.ts     # Entry point — Express server, routes (POST /chat, GET /health)
├── auth.ts     # JWT validation (jose), OIDC discovery, RFC 8693 token exchange
├── agent.ts    # Strands agent + MCP client creation, session management
├── config.ts   # Environment variable loading and LLM/system prompt config
└── util.ts     # Shared types (TokenExchangeResult, ChatRequest, HttpError) and helpers
```

## Environment Variables

| Variable | Description |
|---|---|
| `AGENT_GATEWAY_URL` | GCP Agent Gateway URL (e.g. `https://ping-gcp-agent-gateway.com`) |
| `CORS_ORIGIN_CHAT_UI_STOREFRONT` | Allowed CORS origin for the chat UI (e.g. `https://ping-store-chat-app.com`) |
| `PINGONE_AIC_ISSUER` | PingOne AIC issuer URL (used for OIDC discovery, JWKS, and token endpoint) |
| `AGENT_PORT` | Server port (e.g. `3000`) |
| `AGENT_CLIENT_ID` | This agent's OAuth client ID (also used as expected token audience) |
| `AGENT_CLIENT_SECRET` | This agent's OAuth client secret (for token exchange) |
| `AGENT_REQUIRED_SCOPES` | Space-separated scopes required on the subject token (e.g. `stripe_mcp:invoke email`) |
| `OPENAI_MODEL` | OpenAI model ID (e.g. `gpt-5.4`) |
| `OPENAI_API_KEY` | OpenAI API key for the LLM |
