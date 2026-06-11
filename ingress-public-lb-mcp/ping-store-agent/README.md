# Ping Store Agent

Express backend that acts as a **delegated agent** — receives a user's access token, performs [RFC 8693 token exchange](https://datatracker.ietf.org/doc/html/rfc8693), then uses a Strands AI agent to invoke Stripe MCP tools through the regional load balancer on the user's behalf.

## Token Flow

**Inbound Auth**: `ping-chat-ui-storefront` → subject token (from auth code grant) → `ping-store-agent`  
**Outbound Auth**: `ping-store-agent` → delegated token (from token exchange grant) → Regional Load Balancer → `stripe-mcp`

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

Copy `.env.sample` to `.env` and fill in values.

| Variable | Description |
|---|---|
| `LB_URL` | Regional load balancer URL (e.g. `https://your-mcp-lb.com`) |
| `CORS_ORIGIN_CHAT_UI_STOREFRONT` | Allowed CORS origin for the chat UI (e.g. `https://ping-store-chat-app.com`) |
| `PINGONE_AIC_ISSUER` | PingOne AIC issuer URL (used for OIDC discovery, JWKS, and token endpoint) |
| `AGENT_PORT` | Server port (e.g. `3000`) |
| `AGENT_CLIENT_ID` | This agent's OAuth client ID (also used as expected token audience) |
| `AGENT_CLIENT_SECRET` | This agent's OAuth client secret (for token exchange) |
| `AGENT_REQUIRED_SCOPES` | Space-separated scopes required on the subject token (e.g. `stripe_mcp:invoke email`) |
| `OPENAI_MODEL` | OpenAI model ID (e.g. `gpt-4o`) |
| `OPENAI_API_KEY` | OpenAI API key for the LLM |

## Deployment

```bash
cp .env.sample .env   # fill in values
docker build -t ping-store-agent .
```

Deploy the built image to Cloud Run or any container runtime that can reach the load balancer and PingOne AIC. The service must be publicly reachable so `ping-chat-ui-storefront` can call `/chat`.
