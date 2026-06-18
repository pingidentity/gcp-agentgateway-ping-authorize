# ping-chat-ui-storefront

React SPA that authenticates users via PingOne AIC (PKCE) and provides a
chat interface to `ping-store-agent`.

## Token Flow

```
User → AIC login (PKCE) → access token (with may_act claim)
     → POST /chat to ping-store-agent
     → agent performs RFC 8693 token exchange → delegated MCP calls
```

## Project Structure

```
src/
├── App.tsx                  # Auth routing (LoginScreen ↔ ChatScreen)
├── auth/oidc.ts             # PKCE auth code flow against PingOne AIC
├── api/agent.ts             # Agent backend API client
├── components/
│   ├── LoginScreen.tsx      # Sign-in page
│   └── ChatScreen.tsx       # Chat interface
├── index.css                # Tailwind + Ping theme tokens
└── main.tsx                 # React entry point
```

## Environment Variables

Baked in at build time by Vite.

```
VITE_AIC_ISSUER=             # PingOne AIC OAuth2 issuer URL
VITE_CLIENT_ID=              # OIDC client ID (public client, PKCE)
VITE_REDIRECT_URI=           # OAuth callback URL
VITE_SCOPES=                 # OAuth scopes (e.g. openid profile email stripe_mcp:invoke)
VITE_PING_STORE_AGENT_URL=   # Agent backend URL
```

## Deploy

```bash
cp .env.sample .env   # fill in values
npm install && npm run build
bash deploy.sh
```

`deploy.sh` builds and rsyncs `dist/` to the configured EC2 host over SSH.
Update `EC2_HOST`, `EC2_USER`, and `REMOTE_DIR` at the top of the script for
your environment.
