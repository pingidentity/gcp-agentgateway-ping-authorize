# ingress-public-lb-mcp — AI Storefront Agent with PingAuthorize

A React storefront powered by an AI shopping assistant. Users log in with
PingOne AIC, a delegated agent performs RFC 8693 token exchange to act on
their behalf, and **PingAuthorize** enforces fine-grained policy on every
MCP `tools/call` before it reaches the Stripe backend.

---

## Architecture

```
Browser (ping-chat-ui-storefront)
  PKCE login → AIC access token (with may_act claim)
  POST /chat + Bearer <user_token>
            ↓
ping-store-agent (TypeScript / Strands AI)
  Validates subject token (AIC JWKS)
  RFC 8693 token exchange → delegated token (sub=user, act.sub=agent)
  Strands agent invokes MCP tools
            ↓  POST /mcp  +  Bearer <delegated_token>
GCP Regional Load Balancer
  Traffic Extension ext_proc callout
            ↓  gRPC
lb-ping-authz-shim (Go / Cloud Run, internal)
  Extracts: access_token, mcp_tool_name, mcp_product_id, mcp_total_price, ...
  → PingAuthorize REST API → PERMIT / DENY
            ↓  PERMIT
lb-stripe-mcp (Go / Cloud Run, internal)
  Stripe API — products, customers, payment intents
```

The `lb-ping-authz-shim` and `lb-stripe-mcp` services run with
`--ingress internal-and-cloud-load-balancing` — they are unreachable
from the public internet. All traffic must pass through the load balancer.

---

## Components

| Component | Language | Purpose |
|---|---|---|
| `ping-chat-ui-storefront` | React / TypeScript | PKCE login + chat UI |
| `ping-store-agent` | TypeScript / Strands AI | Token validation, RFC 8693 exchange, MCP agent |
| `lb-ping-authz-shim` | Go / gRPC | ext_proc shim → PingAuthorize |
| `lb-stripe-mcp` | Go / MCP | Stripe tools (products, customers, payments) |

---

## End-to-End Request Flow

### 1. User authenticates
`ping-chat-ui-storefront` completes a PKCE flow with AIC. The resulting
access token contains a `may_act` claim granting the agent permission to
act on behalf of this user.

### 2. UI sends a message
```
POST /chat
Authorization: Bearer <user_access_token>
{ "message": "What products do you have?" }
```

### 3. Agent validates and exchanges the token
`ping-store-agent` verifies the JWT (signature, expiry, issuer, `may_act`
claim), then performs RFC 8693 token exchange:

```
Step 1 — actor token (agent client credentials):
  POST {AIC_ISSUER}/access_token  grant_type=client_credentials
  → actor_token

Step 2 — delegated token:
  POST {AIC_ISSUER}/access_token  grant_type=token-exchange
  subject_token=<user_token>  actor_token=<actor_token>  audience={LB_URL}
  → delegated_token  { sub: user, act: { sub: agent }, aud: lb_url }
```

### 4. Strands agent calls MCP tools
The agent connects an MCP client to the load balancer and invokes tools
with the delegated token.

### 5. Load balancer → ext_proc shim
The Traffic Extension forwards the request to `lb-ping-authz-shim` via
gRPC before routing to `lb-stripe-mcp`.

**Headers phase:** passthrough `/.well-known/*`, 404 unknown paths, 401
missing token. Authenticated `/mcp` requests proceed to body phase.

**Body phase:** parses MCP JSON-RPC, extracts policy attributes, calls
PingAuthorize. PERMIT → forward. DENY → 403.

### 6. stripe-mcp executes the tool
Resolves caller identity via AIC `/userinfo`, executes the Stripe API
call, and returns an MCP JSON-RPC response.

---

## Policy Attributes Sent to PingAuthorize

```json
{
  "attributes": {
    "access_token": "<delegated token>",
    ":path": "/mcp",
    "mcp_method": "tools/call",
    "mcp_tool_name": "create_stripe_payment_intent",
    "mcp_product_id": "prod_abc123",
    "mcp_quantity": "1",
    "mcp_total_price": "29.99",
    "mcp_currency": "usd"
  }
}
```

Example policies:
- Allow `list_stripe_products` and `get_stripe_product` freely; require `stripe_mcp:invoke` scope for payments
- Deny `create_stripe_payment_intent` if `mcp_total_price` exceeds a per-user limit
- Restrict purchases to users whose `sub` is in an approved-customers group

---

## Prerequisites

- GCP project with Cloud Run, Cloud Load Balancing, Service Extensions,
  and Artifact Registry APIs enabled
- PingOne AIC tenant with:
  - Public OIDC client for the UI (PKCE, `stripe_mcp:invoke` scope)
  - Confidential client for the agent (client credentials + token exchange, `may_act` policy)
- PingAuthorize reachable from Cloud Run
- Stripe account with a secret key and at least one customer with a saved
  payment method whose email matches an AIC user
- OpenAI API key for the Strands agent LLM

---

## Deployment

### Step 1 — Store secrets

```bash
printf "sk_live_..." | gcloud secrets create stripe-secret-key --data-file=-
```

### Step 2 — Deploy Cloud Run services

```bash
# From repo root:
gcloud builds submit --config ingress-public-lb-mcp/ping-authz-shim/cloudbuild.yaml .
gcloud builds submit --config ingress-public-lb-mcp/stripe-mcp/cloudbuild.yaml .
```

`ping-store-agent` is a long-running Express server deployed separately
(Docker or any container runtime reachable by the UI).

### Step 3 — Configure the regional load balancer

In GCP console or via `gcloud`:

1. Create **serverless NEGs** for `lb-ping-authz-shim` and `lb-stripe-mcp`
2. Create **backend services** (set `lb-ping-authz-shim` backend to HTTP/2)
3. Create a **regional external Application Load Balancer** routing `/mcp`
   and `/.well-known/*` to the `lb-stripe-mcp` backend
4. Create a **Traffic Extension** callout pointing at `lb-ping-authz-shim`
   with request header + body processing enabled
5. Attach the Traffic Extension to the `/mcp` route

### Step 4 — Deploy the UI

```bash
cd ingress-public-lb-mcp/ping-chat-ui-storefront
cp .env.sample .env   # fill in values
npm install && npm run build
bash deploy.sh
```

---

## Folder Structure

```
ingress-public-lb-mcp/
├── README.md
├── ping-chat-ui-storefront/    # React PKCE chat UI
├── ping-store-agent/           # TypeScript delegated agent backend
├── ping-authz-shim/            # Go ext_proc shim → PingAuthorize
│   └── cloudbuild.yaml
└── stripe-mcp/                 # Go MCP → Stripe API
    └── cloudbuild.yaml
```
