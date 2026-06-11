# Ping Chat UI Storefront

React SPA that authenticates users via PingOne AIC (PKCE) and provides a chat interface to the [ping-store-agent](../ping-store-agent) backend.

## Token Flow

```
User → AIC login (PKCE) → access token (with may_act claim)
     → sent to ping-store-agent → token exchange (RFC 8693) → delegated MCP calls
```

The access token authorizes the agent to act on behalf of the user. The agent performs token exchange and calls Stripe MCP tools through the regional load balancer.

## Project Structure

```
src/
├── App.tsx                      # Auth routing (LoginScreen ↔ ChatScreen)
├── auth/oidc.ts                 # PKCE auth code flow against PingOne AIC
├── api/agent.ts                 # Agent backend API client
├── components/
│   ├── LoginScreen.tsx          # Sign-in page
│   └── ChatScreen.tsx           # Chat interface
├── index.css                    # Tailwind config + Ping theme tokens
└── main.tsx                     # React entry point
```

## Environment Variables

Copy `.env.sample` to `.env` and fill in values. These are baked in at build time by Vite.

| Variable | Description |
|---|---|
| `VITE_AIC_ISSUER` | PingOne AIC OAuth2 issuer URL |
| `VITE_CLIENT_ID` | OIDC client ID (public client, PKCE) |
| `VITE_REDIRECT_URI` | OAuth callback URL (e.g. `https://ping-store-chat-app.com/callback`) |
| `VITE_SCOPES` | OAuth scopes (default: `openid profile email stripe_mcp:invoke`) |
| `VITE_PING_STORE_AGENT_URL` | Agent backend URL (e.g. `https://ping-store-agent.com`) |

## Deployment

```bash
cp .env.sample .env   # fill in values
./deploy.sh
```

`deploy.sh` runs `npm ci`, `npm run build`, then rsyncs the built `dist/` folder to the configured EC2 host over SSH. Update the `EC2_HOST`, `EC2_USER`, and `REMOTE_DIR` variables at the top of the script for your environment.
