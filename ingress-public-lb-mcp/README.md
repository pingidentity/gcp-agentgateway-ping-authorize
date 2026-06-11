# ingress-public-lb-mcp

**Use Case**: Public internet client accessing an MCP server on Cloud Run via a GCP Regional Load Balancer with Ping Authorize as the policy enforcement point.

**Flow**: Public Internet → GCP Regional Load Balancer (ext_proc) → MCP Server (Cloud Run)

**Example**: A React storefront powered by an AI shopping assistant. The user logs in with PingOne AIC, an agent performs RFC 8693 token exchange to act on their behalf, and every MCP tool call is authorized by PingAuthorize before it reaches the Stripe MCP server.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Browser                                                    │
│  ping-chat-ui-storefront (React)                            │
│  - Authenticates user via PingOne AIC                       │
│  - Gets access token with may_act claim                     │
└────────────────────────┬────────────────────────────────────┘
                         │ POST /chat
                         │ Authorization: Bearer <subject_token>
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  ping-store-agent (Strands AI)                              │
│  - Validates subject token (JWT via AIC JWKS)               │
│  - RFC 8693 token exchange → delegated token                │
│  - Invokes Strands agent with MCP client                    │
└────────────────────────┬────────────────────────────────────┘
                         │ POST /mcp (HTTP/2)
                         │ Authorization: Bearer <delegated_token>
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  GCP Regional Load Balancer                                 │
│  Traffic Extension (ext_proc callout)                       │
└────────────────────────┬────────────────────────────────────┘
                         │ gRPC ext_proc stream
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  ping-authz-shim (Go / gRPC)                                │
│  - Parses MCP JSON-RPC body                                 │
│  - Extracts tool name + arguments                           │
│  - Calls PingAuthorize for a policy decision                │
│  - PERMIT → forward   DENY → 403                            │
└──────────────┬──────────────────────┬───────────────────────┘
               │ PERMIT               │ DENY
               ▼                      ▼
┌──────────────────────────┐   ┌──────────────────┐
│  stripe-mcp (Go / MCP)   │   │  403 Forbidden   │
│  - list_stripe_products  │   └──────────────────┘
│  - get_stripe_product    │
│  - get_stripe_customer   │
│  - create_payment_intent │
└──────────────────────────┘
```

**Security posture**: `ping-authz-shim` and `stripe-mcp` run on Cloud Run with `--ingress internal-and-cloud-load-balancing`. They are unreachable from the public internet — all traffic must pass through the load balancer and ext_proc.

---

## Services

| Service | Purpose |
|---|---|
| [`ping-chat-ui-storefront`](./ping-chat-ui-storefront/) | Chat UI — PKCE login, conversation interface |
| [`ping-store-agent`](./ping-store-agent/) | Delegated agent backend — JWT validation, RFC 8693 token exchange, Strands AI agent |
| [`ping-authz-shim`](./ping-authz-shim/) | Envoy ext_proc shim — intercepts every request, parses MCP payloads, enforces policy via PingAuthorize |
| [`stripe-mcp`](./stripe-mcp/) | MCP server exposing Stripe tools (catalog, customer lookup, payments) |
---

## End-to-End Request Flow

### 1. User authenticates (OAuth 2.0 Authorization Code + PKCE)

`ping-chat-ui-storefront` initiates a PKCE flow with PingOne AIC. After login, the user receives an access token containing:

```json
{
  "sub": "user123",
  "email": "user@example.com",
  "scope": "openid profile email stripe_mcp:invoke",
  "may_act": { "client_id": "<agent_client_id>" }
}
```

The `may_act` claim is issued by AIC to signal that this user has authorized the agent to act on their behalf.

### 2. UI sends message to agent backend

```
POST /chat
Authorization: Bearer <user_access_token>
Body: { "message": "What products do you have?" }
```

### 3. Agent validates the subject token

`ping-store-agent` verifies the JWT against AIC's JWKS endpoint and checks:
- Signature, expiry, issuer, audience
- Required scopes are present (`stripe_mcp:invoke`, `email`)
- `may_act.client_id` matches this agent's client ID

### 4. Agent performs RFC 8693 token exchange

```
Step 1 — Actor token (agent identifies itself):
  POST {AIC_ISSUER}/access_token
  grant_type=client_credentials
  → actor_token

Step 2 — Delegated token (agent + user identity combined):
  POST {AIC_ISSUER}/access_token
  grant_type=urn:ietf:params:oauth:grant-type:token-exchange
  subject_token=<user_token>
  actor_token=<actor_token>
  audience={LB_URL}
  → delegated_token
```

The resulting delegated token carries both identities, enabling PingAuthorize to write policies based on who the user is *and* which agent is acting on their behalf:

```json
{
  "sub": "user123",
  "act": { "sub": "agent-service-account" },
  "aud": "https://your-mcp-lb.com",
  "scope": "stripe_mcp:invoke"
}
```

### 5. Agent invokes MCP tools via the load balancer

The Strands AI agent connects an MCP client to the load balancer and invokes tools with the delegated token:

```
POST https://your-mcp-lb.com/mcp
Authorization: Bearer <delegated_token>
Body: {
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": { "name": "list_stripe_products", "arguments": {} }
}
```

### 6. Load balancer intercepts and calls ext_proc shim

The regional load balancer's Traffic Extension forwards the request to `ping-authz-shim` via gRPC before it reaches `stripe-mcp`.

**Phase 1 — Request headers:**
- Path not `/mcp` or `/.well-known/*` → 404
- No bearer token → 401 with `WWW-Authenticate` header
- Token present → proceed to body phase

**Phase 2 — Request body:**

The shim parses the MCP JSON-RPC body and extracts policy attributes:

```
access_token:      <delegated_token_value>
mcp_method:        tools/call
mcp_tool_name:     list_stripe_products
mcp_product_id:    (payment tools only)
mcp_quantity:      (payment tools only)
mcp_total_price:   (payment tools only)
mcp_currency:      (payment tools only)
```

These attributes are sent to PingAuthorize:

```
POST {PING_AUTHORIZE_URL}
Body: { "attributes": { ... } }
→ { "decision": "PERMIT" | "DENY" }
```

- **PERMIT** → request forwarded to `stripe-mcp` with `x-ping-authorize-decision: permit` header
- **DENY** → immediate 403 response, `stripe-mcp` never sees the request

### 7. stripe-mcp executes the tool

`stripe-mcp` validates the bearer token by calling AIC's `/userinfo` endpoint to resolve the caller's email, then executes the requested Stripe tool and returns an MCP JSON-RPC response.

### 8. Response returns to the user

The MCP result travels back through the agent, which uses the LLM to generate a natural language response. The response is returned to the UI and displayed in the chat.

---

## MCP Tools

| Tool | Description | Key Parameters |
|---|---|---|
| `list_stripe_products` | List all active products with prices | — |
| `get_stripe_product` | Get details for a specific product | `product_id` |
| `get_stripe_customer` | Look up the user's saved payment method | — |
| `create_stripe_payment_intent` | Charge the user's saved card | `product_id`, `quantity`, `total_price`, `currency` |

`stripe-mcp` also serves OAuth discovery metadata at `/.well-known/oauth-protected-resource` (RFC 9728) and `/.well-known/oauth-authorization-server` (RFC 8414) so MCP-aware clients can discover the authorization server automatically.

---

## Prerequisites

- **GCP project** with Cloud Run, Cloud Load Balancing, Service Extensions, and Artifact Registry APIs enabled
- **PingOne AIC tenant** configured with:
  - A public OIDC client for the UI (Authorization Code + PKCE, `stripe_mcp:invoke` scope)
  - A confidential client for the agent (Client Credentials + Token Exchange grants, `may_act` policy)
  - A `may_act` policy/script that adds the agent's `client_id` to the `may_act` claim
  - Dynamic Client Registration enabled
- **PingAuthorize** deployed and reachable from Cloud Run (e.g., GCE VM in the same GCP project)
- **Stripe account** with a secret API key and at least one customer whose email matches a user in PingOne AIC, with a saved payment method
- **OpenAI API key** for the agent LLM

---

## Deployment

### Services

Each service has its own README with deployment instructions and environment variables:

| Service | README |
|---|---|
| `ping-chat-ui-storefront` | [README](./ping-chat-ui-storefront/README.md) |
| `ping-store-agent` | [README](./ping-store-agent/README.md) |
| `ping-authz-shim` | [README](./ping-authz-shim/README.md) |
| `stripe-mcp` | [README](./stripe-mcp/README.md) |

Deploy the services first, then configure the load balancer below.

### Regional Load Balancer

In the GCP console (or via `gcloud`):

1. **Create serverless NEGs** for the two Cloud Run services:
   - `ping-authz-shim` NEG (region: `us-central1`)
   - `stripe-mcp` NEG (region: `us-central1`)

2. **Create backend services** pointing to each NEG:
   - `ping-authz-shim-backend` — set protocol to HTTP/2 (required for gRPC)
   - `stripe-mcp-backend`

3. **Provision a regional external Application Load Balancer** with:
   - A URL map routing `/mcp` and `/.well-known/*` to `stripe-mcp-backend`
   - An SSL certificate for your domain
   - A static external IP

4. **Create a Traffic Extension** (Service Extension) callout:
   - Point it at `ping-authz-shim-backend`
   - Enable **request header processing** and **request body processing**

5. **Attach the Traffic Extension** to the load balancer's URL map on the `/mcp` route.

After setup, all `/mcp` traffic from the internet hits the load balancer, which callouts to `ping-authz-shim` for a policy decision on every request before forwarding to `stripe-mcp`.
