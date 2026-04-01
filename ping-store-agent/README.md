# Ping Store Agent

Express backend that acts as a **delegated agent** — receives a user's access token, performs [RFC 8693 token exchange](https://datatracker.ietf.org/doc/html/rfc8693), then uses a Strands AI agent to invoke Stripe MCP tools through the GCP Agent Gateway on the user's behalf.

## Token Flow

```
Chat UI → subject token (with may_act) → this agent
       → token exchange (RFC 8693) → delegated token (sub=user, act=agent)
       → Agent Gateway → PingOne Authorize → stripe-mcp
```

## Project Structure

```
src/
├── main.ts              # Entry point — starts the Express server
├── server.ts            # Express routes (POST /chat, GET /health)
├── auth.ts              # Subject token validation (issuer, audience, scope, may_act)
├── session.ts           # Per-user agent session management
├── agent.ts             # Strands agent + MCP client creation
└── token-exchange.ts    # RFC 8693 token exchange with PingOne AIC
```

## Setup

```bash
cp .env.sample .env   # Fill in AIC credentials, OpenAI key, gateway URL
npm install
npm run build
npm start
```

## Environment Variables

| Variable | Description |
|---|---|
| `AIC_TOKEN_ENDPOINT` | PingOne AIC token endpoint URL |
| `AIC_ISSUER` | Expected token issuer for validation |
| `AGENT_CLIENT_ID` | This agent's OAuth client ID (also used as expected audience) |
| `AGENT_CLIENT_SECRET` | This agent's OAuth client secret (for token exchange) |
| `AGENT_GATEWAY_URL` | GCP Agent Gateway MCP endpoint (e.g. `https://...run.app/mcp`) |
| `OPENAI_API_KEY` | OpenAI API key for the LLM |
| `PORT` | Server port (default: `8080`) |

## Deployment

```bash
./deploy.sh   # Builds and rsyncs to EC2 → runs via systemd
```
