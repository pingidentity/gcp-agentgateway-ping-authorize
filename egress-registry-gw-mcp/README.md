# egress-registry-gw-mcp — Identity Provisioning via Agent Gateway

An ADK Gemini agent that provisions user accounts across **PingOne AIC
(ForgeRock Identity Cloud)** and **Microsoft Entra (Azure AD)** using the
**GCP Agent Gateway** in egress (Agent-to-Anywhere) mode. **PingAuthorize**
enforces fine-grained policy on every MCP `tools/call` before it reaches
a backend.

---

## How It Works

The agent runs inside **Vertex AI Agent Runtime** (deployed via
`deploy/gcp/deploy_agent.py`). A React UI authenticates users with AIC via
OIDC, exchanges the user token for a Google federated credential using
**Workload Identity Federation**, then calls the Agent Runtime directly from
the browser.

When the agent executes a tool, the call flows through the Agent Gateway:

```
Browser (React UI)
  ↓  OIDC token (AIC) → WIF token exchange (Google STS)
  ↓  POST /sessions  +  streamQuery (Vertex AI Agent Runtime v1beta1)

Vertex AI Agent Runtime  ←→  ping-provisioner-agent (ADK LlmAgent, Gemini)
  ↓  McpToolset.header_provider: RFC 8693 token exchange (raw AIC → delegated)
  ↓  MCP tools/call  →  Agent Gateway (ping-authz-agent-gateway, AGENT_TO_ANYWHERE)

Agent Gateway
  ↓  CONTENT_AUTHZ ext_proc callout  →  gw-ping-authz-shim (gRPC, Cloud Run)

gw-ping-authz-shim
  Extracts: access_token, :path, mcp_tool_name, mcp_email, mcp_username
  →  PingAuthorize REST API  →  PERMIT / DENY

  ↓  PERMIT

gw-pingone-aic-mcp (Cloud Run, Go)    gw-entra-mcp (Cloud Run, Go)
  PingOne AIC Management REST API       Microsoft Graph API
```

### Key Components

| Component | Role |
|---|---|
| **Vertex AI Agent Runtime** | Managed ADK hosting — runs `ping-provisioner-agent`, manages sessions, streams responses |
| **Workload Identity Federation** | Browser exchanges an AIC OIDC token for a Google federated token to call Vertex AI directly — no Cloud Run proxy |
| **RFC 8693 Token Exchange** | Agent exchanges raw UI token for a delegated token (sub=user, act.sub=gcp_ping_provision_agent) before each MCP call |
| **GCP Agent Registry** | Registers the agent and both MCP servers; `AgentRegistry.get_mcp_server()` resolves endpoint URLs at runtime |
| **Agent Gateway (egress)** | Routes all agent → MCP traffic through the authz shim before reaching backends |
| **gw-ping-authz-shim (ext_proc)** | Inspects every `tools/call` body and enforces PingAuthorize policy |
| **PingAuthorize** | Policy engine — enforce rules like "deprovision requires elevated scope" or "only provision to approved domains" |

---

## Cloud Run Services

| Service | Language | Ingress | Purpose |
|---|---|---|---|
| `gw-ping-authz-shim` | Go / gRPC | internal | CONTENT_AUTHZ ext_proc → PingAuthorize |
| `gw-pingone-aic-mcp` | Go / MCP | internal | MCP server wrapping PingOne AIC REST API |
| `gw-entra-mcp` | Go / MCP | internal | MCP server wrapping Microsoft Graph API |

The agent itself runs in **Vertex AI Agent Runtime** (not Cloud Run).
The UI is a static React app deployed separately.

---

## Prerequisites

```bash
# gcloud alpha required for Agent Registry and Agent Gateway
gcloud components install alpha && gcloud components update

# Enable APIs
gcloud services enable \
  aiplatform.googleapis.com \
  agentregistry.googleapis.com \
  networkservices.googleapis.com \
  networksecurity.googleapis.com \
  run.googleapis.com \
  artifactregistry.googleapis.com \
  secretmanager.googleapis.com \
  cloudbuild.googleapis.com \
  --project=YOUR_PROJECT
```

Required IAM roles on the service account used for deploys:
- `roles/run.admin`
- `roles/artifactregistry.writer`
- `roles/secretmanager.secretAccessor`
- `roles/agentregistry.editor`
- `roles/networkservices.serviceExtensionsAdmin`
- `roles/networksecurity.securityPolicyAdmin`

---

## Secret Manager Setup

Create secrets before deploying:

```bash
# PingOne AIC admin credentials (gw-pingone-aic-mcp)
printf "value" | gcloud secrets create aic-admin-client-id --data-file=- --project=YOUR_PROJECT
printf "value" | gcloud secrets create aic-admin-client-secret --data-file=- --project=YOUR_PROJECT

# Microsoft Entra credentials (gw-entra-mcp)
printf "value" | gcloud secrets create azure-tenant-id --data-file=- --project=YOUR_PROJECT
printf "value" | gcloud secrets create azure-client-id --data-file=- --project=YOUR_PROJECT
printf "value" | gcloud secrets create azure-client-secret --data-file=- --project=YOUR_PROJECT

# RFC 8693 exchange client (agent token delegation)
printf "value" | gcloud secrets create exchange-client-secret --data-file=- --project=YOUR_PROJECT
```

---

## Deployment

### Step 1 — Deploy Cloud Run services

```bash
# From repo root:
gcloud builds submit --config egress-registry-gw-mcp/deploy/gcp/cloudbuild.ping-authz-shim.yaml .
gcloud builds submit --config egress-registry-gw-mcp/deploy/gcp/cloudbuild.pingone-aic-mcp.yaml .
gcloud builds submit --config egress-registry-gw-mcp/deploy/gcp/cloudbuild.entra-mcp.yaml .
```

### Step 2 — Set up Agent Registry and Agent Gateway

```bash
PROJECT_ID=YOUR_PROJECT REGION=us-central1 \
  bash egress-registry-gw-mcp/deploy/gcp/setup-agent-registry.sh
```

This script resolves Cloud Run URLs, registers services in Agent Registry,
creates the `ping-authz-agent-gateway` (egress mode), creates the authz
extension pointing at `gw-ping-authz-shim`, and attaches the authz policy.

### Step 3 — Deploy agent to Vertex AI Agent Runtime

```bash
cd egress-registry-gw-mcp
python deploy/gcp/deploy_agent.py \
  --project YOUR_PROJECT \
  --region us-central1 \
  --network-attachment projects/YOUR_PROJECT/regions/us-central1/networkAttachments/agent-gateway-attachment \
  --pingone-mcp-resource projects/YOUR_PROJECT/locations/us-central1/mcpServers/pingone-aic-mcp-server \
  --entra-mcp-resource   projects/YOUR_PROJECT/locations/us-central1/mcpServers/entra-mcp-server \
  --exchange-client-secret $EXCHANGE_CLIENT_SECRET
```

After deploy, grant the agent's AGENT_IDENTITY egress IAM:

```bash
python deploy/gcp/grant_egressor_iam.py \
  --project YOUR_PROJECT \
  --region us-central1 \
  --agent-resource projects/YOUR_PROJECT/locations/us-central1/reasoningEngines/RESOURCE_ID
```

### Step 4 — Deploy the UI

```bash
cd egress-registry-gw-mcp/ping-provisioner-ui
npm install && npm run build
bash deploy.sh   # rsync to EC2 / nginx
```

Update `ping-provisioner-ui/.env` with your project's Reasoning Engine ID and
WIF provider path before building.

---

## Policy Attributes Sent to PingAuthorize

Every `tools/call` through the gateway triggers a PingAuthorize check with:

```json
{
  "attributes": {
    "access_token": "<delegated bearer token>",
    ":path": "/mcp/entra",
    ":method": "POST",
    "mcp_method": "tools/call",
    "mcp_tool_name": "provision_user",
    "mcp_email": "alice@example.com",
    "mcp_username": "alice.smith"
  }
}
```

Example policies:
- Allow `list_users` for any agent; require `pingone:provisioning` scope for `provision_user`
- Deny `deprovision_user` unless the token's `act.sub` is in an approved-deprovisioners group
- Block provisioning to domains not on an allowlist

---

## Folder Structure

```
egress-registry-gw-mcp/
├── README.md
├── ping-provisioner-agent/     # ADK Python + FastAPI (Cloud Run version)
├── ping-provisioner-ui/        # React UI — WIF + Vertex AI Agent Runtime
├── pingone-aic-mcp/            # Go MCP → PingOne AIC REST API
├── entra-mcp/                  # Go MCP → Microsoft Graph API
├── ping-authz-shim/            # Go ext_proc shim → PingAuthorize
└── deploy/gcp/
    ├── deploy_agent.py                       # Deploy agent to Vertex AI Agent Runtime
    ├── grant_egressor_iam.py                 # Grant AGENT_IDENTITY egress IAM
    ├── setup-agent-registry.sh              # Agent Registry + Agent Gateway setup
    ├── cloudbuild.ping-authz-shim.yaml
    ├── cloudbuild.pingone-aic-mcp.yaml
    ├── cloudbuild.entra-mcp.yaml
    ├── cloudbuild.ping-provisioner-agent.yaml
    ├── toolspec.pingone-aic-mcp.json         # Agent Registry tool definitions
    ├── toolspec.entra-mcp.json
    ├── agent-gateway-egress.yaml             # Agent Gateway (egress) config
    ├── authz-extension.yaml                  # CONTENT_AUTHZ ext_proc extension
    └── authz-policy.yaml                     # Authz policy → attaches shim to gateway
```
