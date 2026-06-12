# ping-provisioner-agent

ADK Python service that provisions user accounts across PingOne AIC and
Microsoft Entra through the Agent Gateway.

## Overview

The agent:
1. Receives a provisioning instruction via `POST /provision`.
2. Fetches a bearer token using OAuth 2.0 client credentials from AIC.
3. Creates a Gemini `LlmAgent` (Google ADK) with two `MCPToolset` connections:
   - `/mcp/pingone` — PingOne AIC provisioning tools
   - `/mcp/entra` — Microsoft Entra provisioning tools
4. Runs the instruction; the Agent Gateway intercepts each MCP call and
   consults PingAuthorize before forwarding to the backend.

## Endpoints

| Method | Path | Description |
|---|---|---|
| `POST` | `/provision` | Run a provisioning instruction |
| `GET` | `/health` | Liveness probe |

### POST /provision

```json
// Request
{ "instruction": "Provision alice@example.com with username alice.smith in both AIC and Entra" }

// Response
{ "result": "Provisioned alice@example.com:\n  AIC: user_id=abc123\n  Entra: user_id=def456-..." }
```

## MCP Tools Available

Both MCPToolsets expose the same tool interface:

| Tool | Description |
|---|---|
| `provision_user` | Create a new user account |
| `deprovision_user` | Permanently delete a user by email |
| `update_user_status` | Enable or disable an account |
| `list_users` | List or search accounts |

## Environment Variables

Copy `.env.sample` and fill in the values:

```
AGENT_GATEWAY_URL=      # Internal LB URL (e.g. https://agent-gateway.internal)
PINGONE_AIC_ISSUER=     # AIC OAuth2 token endpoint base (e.g. https://openam-tenant.forgeblocks.com/am/oauth2/alpha)
AGENT_CLIENT_ID=        # OAuth client ID for this agent
AGENT_CLIENT_SECRET=    # OAuth client secret
AGENT_PORT=3000
GEMINI_MODEL=gemini-2.0-flash
GOOGLE_CLOUD_PROJECT=   # GCP project ID for Vertex AI / Gemini API
```

## Local Development

```bash
pip install -r requirements.txt
cp .env.sample .env
# Edit .env with real values
python main.py
```

## Docker

```bash
docker build -t ping-provisioner-agent .
docker run -p 3000:3000 --env-file .env ping-provisioner-agent
```
