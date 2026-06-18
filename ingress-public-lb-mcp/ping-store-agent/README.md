# ping-store-agent

TypeScript/Express backend that acts as a **delegated agent**: receives a
user's AIC access token, performs RFC 8693 token exchange to obtain a
delegated token, then runs a Strands AI agent that invokes Stripe MCP tools
through the regional load balancer on the user's behalf.

## Token Flow

```
ping-chat-ui-storefront
  → POST /chat  +  Authorization: Bearer <user_token>

ping-store-agent
  1. Validate subject token (JWT, AIC JWKS, may_act claim)
  2. RFC 8693 token exchange → delegated token (sub=user, act.sub=agent)
  3. Strands agent invokes MCP tools via load balancer with delegated token
```

## Project Structure

```
src/
├── main.ts     # Express server, POST /chat and GET /health routes
├── auth.ts     # JWT validation (jose), OIDC discovery, RFC 8693 token exchange
├── agent.ts    # Strands agent + MCP client creation
├── config.ts   # Environment config and system prompt
└── util.ts     # Shared types and helpers
```

## Environment Variables

```
LB_URL=                          # Regional load balancer URL
CORS_ORIGIN_CHAT_UI_STOREFRONT=  # Allowed CORS origin for the chat UI
PINGONE_AIC_ISSUER=              # AIC issuer URL (OIDC discovery, JWKS, token endpoint)
AGENT_PORT=3000
AGENT_CLIENT_ID=                 # This agent's OAuth client ID
AGENT_CLIENT_SECRET=             # This agent's OAuth client secret
AGENT_REQUIRED_SCOPES=           # Scopes required on the subject token
OPENAI_MODEL=gpt-4o
OPENAI_API_KEY=
```

## Local Development

```bash
cp .env.sample .env
npm install
npm run dev
```

## Docker

```bash
docker build -t ping-store-agent .
docker run -p 3000:3000 --env-file .env ping-store-agent
```
