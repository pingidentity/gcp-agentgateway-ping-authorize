# egress-registry-gw-mcp — Use Case 2: Identity Provisioning Agent Gateway

An ADK Python agent provisions user accounts across two identity systems —
**PingOne AIC (ForgeRock Identity Cloud)** and **Microsoft Entra** — through the
**GCP Agent Gateway** (egress / Agent-to-Anywhere mode) with **PingAuthorize**
enforcing fine-grained policy on every MCP `tools/call` before it reaches a backend.

Both the agent and the MCP servers are registered in **GCP Agent Registry**, making
them visible in the GCP console and enabling runtime service discovery without
hardcoded URLs.

---

## Architecture

```
ping-provisioner-agent (Cloud Run, ADK Python / Gemini)
  AgentRegistry.get_mcp_toolset("mcpServers/pingone-aic-mcp-server")
  AgentRegistry.get_mcp_toolset("mcpServers/entra-mcp-server")
  ↓ uses Application Default Credentials (Cloud Run service account)

       ↓  GCP Agent Gateway — ping-authz-agent-gateway (egress / AGENT_TO_ANYWHERE)
          Routes to registered MCP server endpoints from Agent Registry

       ↓  CONTENT_AUTHZ authz-extension → ping-authz-shim (Cloud Run, gRPC)
          ext_proc: inspects full MCP JSON-RPC body

ping-authz-shim
  Extracts: mcp_tool_name, mcp_email, mcp_username from tools/call body
  Extracts: access_token from Authorization header
  → PingAuthorize REST API → PERMIT / DENY

       ↓  PERMIT

pingone-aic-mcp (Cloud Run, Go)       entra-mcp (Cloud Run, Go)
  PingOne AIC Management REST API        Microsoft Graph API
  Registered as: pingone-aic-mcp-server  Registered as: entra-mcp-server
  Tools: provision_user,                 Tools: provision_user,
         deprovision_user,                      deprovision_user,
         update_user_status,                    update_user_status,
         list_users                             list_users
```

### Why Agent Registry + Agent Gateway?

| Component | Role |
|---|---|
| **Agent Registry** | Registers the agent and both MCP servers; resolves endpoint URLs at runtime — no hardcoded URLs in agent code |
| **Agent Gateway (egress)** | Managed GCP network control plane — routes all agent → MCP calls through policy enforcement |
| **ping-authz-shim (ext_proc)** | CONTENT_AUTHZ extension — gives PingAuthorize visibility into every tools/call, including provisioning arguments |
| **PingAuthorize** | Policy engine — enforces rules like "agent can provision in AIC but not Entra" or "deprovision requires elevated scope" |

---

## Services

| Service | Language | Cloud Run ingress | Purpose |
|---|---|---|---|
| `ping-provisioner-agent` | Python / ADK | `all` | LlmAgent using AgentRegistry for MCP discovery |
| `ping-authz-shim` | Go / gRPC | `internal` | CONTENT_AUTHZ ext_proc callout → PingAuthorize |
| `pingone-aic-mcp` | Go / MCP | `internal` | MCP server wrapping PingOne AIC (ForgeRock) REST API |
| `entra-mcp` | Go / MCP | `internal` | MCP server wrapping Microsoft Graph API |

---

## Prerequisites

```bash
# Install gcloud alpha component (required for Agent Registry and Agent Gateway)
gcloud components install alpha
gcloud components update

# Enable required APIs
gcloud services enable \
  agentregistry.googleapis.com \
  networkservices.googleapis.com \
  networksecurity.googleapis.com \
  run.googleapis.com \
  artifactregistry.googleapis.com \
  secretmanager.googleapis.com \
  cloudbuild.googleapis.com \
  --project=tech-partner-ping
```

Required IAM roles on the Cloud Build / deploy service account:
- `roles/agentregistry.editor`
- `roles/run.admin`
- `roles/artifactregistry.writer`
- `roles/secretmanager.secretAccessor`
- `roles/networkservices.serviceExtensionsAdmin`
- `roles/networksecurity.securityPolicyAdmin`

---

## Secret Manager — Required Secrets

Create these before running Cloud Build:

```bash
# PingOne AIC admin credentials (used by pingone-aic-mcp)
printf "your-aic-admin-client-id" | \
  gcloud secrets create aic-admin-client-id --data-file=- --project=tech-partner-ping
printf "your-aic-admin-client-secret" | \
  gcloud secrets create aic-admin-client-secret --data-file=- --project=tech-partner-ping

# Microsoft Entra credentials (used by entra-mcp)
printf "your-azure-tenant-id" | \
  gcloud secrets create azure-tenant-id --data-file=- --project=tech-partner-ping
printf "your-azure-client-id" | \
  gcloud secrets create azure-client-id --data-file=- --project=tech-partner-ping
printf "your-azure-client-secret" | \
  gcloud secrets create azure-client-secret --data-file=- --project=tech-partner-ping
```

The `ping-provisioner-agent` uses Application Default Credentials (ADC) via its
Cloud Run service account — no credentials needed in Secret Manager for the agent.

---

## Quick Start

### Step 1 — Deploy all services via Cloud Build

```bash
# From the repo root.

# 1. Authorization shim (deploy before Agent Gateway is configured)
gcloud builds submit \
  --config egress-registry-gw-mcp/deploy/gcp/cloudbuild.ping-authz-shim.yaml .

# 2. MCP backends (each build deploys + registers in Agent Registry)
gcloud builds submit \
  --config egress-registry-gw-mcp/deploy/gcp/cloudbuild.pingone-aic-mcp.yaml .
gcloud builds submit \
  --config egress-registry-gw-mcp/deploy/gcp/cloudbuild.entra-mcp.yaml .

# 3. Provisioner agent (deploys + registers in Agent Registry)
gcloud builds submit \
  --config egress-registry-gw-mcp/deploy/gcp/cloudbuild.ping-provisioner-agent.yaml .
```

Each Cloud Build pipeline: build Docker image → push to Artifact Registry →
deploy to Cloud Run → register service in GCP Agent Registry.

### Step 2 — Create the Agent Gateway and authz policy

```bash
PROJECT_ID=tech-partner-ping REGION=us-central1 \
  bash egress-registry-gw-mcp/deploy/gcp/setup-agent-registry.sh
```

This script:
1. Resolves the Cloud Run URL for each service
2. Registers agent + MCP servers in Agent Registry (idempotent)
3. Creates the Agent Gateway (`ping-authz-agent-gateway`, egress mode)
4. Creates the authz extension (`ping-authz-ext`) pointing at `ping-authz-shim`
5. Creates the authz policy (`ping-authz-policy`) attaching the extension to the gateway

### Step 3 — Verify in the GCP console

```
https://console.cloud.google.com/agent-registry?project=tech-partner-ping
```

You should see:
- **Agents**: `ping-provisioner-agent`
- **MCP Servers**: `pingone-aic-mcp-server`, `entra-mcp-server`

### Step 4 — Test provisioning

```bash
PROVISIONER_URL=$(gcloud run services describe ping-provisioner-agent \
  --region=us-central1 --project=tech-partner-ping --format='value(status.url)')

curl -X POST "${PROVISIONER_URL}/provision" \
  -H "Content-Type: application/json" \
  -d '{"instruction": "Provision alice@example.com in both PingOne AIC and Entra"}'
```

---

## Policy Attributes Sent to PingAuthorize

The `ping-authz-shim` CONTENT_AUTHZ ext_proc sends these attributes on every
`tools/call`:

```json
{
  "attributes": {
    "access_token": "<agent identity token>",
    ":path": "/mcp",
    "mcp_method": "tools/call",
    "mcp_tool_name": "provision_user",
    "mcp_email": "alice@example.com",
    "mcp_username": "alice.smith"
  }
}
```

Example PingAuthorize policies:
- Allow `list_users` freely; require specific scope for `provision_user`
- Deny `deprovision_user` unless the agent service account is in `approved-deprovisioners`
- Block provisioning to email domains not on an allowlist

---

## Folder Structure

```
egress-registry-gw-mcp/
├── README.md
├── ping-provisioner-agent/   # ADK Python + FastAPI; AgentRegistry-based discovery
├── pingone-aic-mcp/          # Go MCP → ForgeRock AIC REST API
├── entra-mcp/                # Go MCP → Microsoft Graph API
├── ping-authz-shim/          # Go ext_proc shim → PingAuthorize (CONTENT_AUTHZ)
└── deploy/gcp/
    ├── cloudbuild.ping-authz-shim.yaml
    ├── cloudbuild.pingone-aic-mcp.yaml
    ├── cloudbuild.entra-mcp.yaml
    ├── cloudbuild.ping-provisioner-agent.yaml
    ├── toolspec.pingone-aic-mcp.json    # Agent Registry MCP tool definitions
    ├── toolspec.entra-mcp.json          # Agent Registry MCP tool definitions
    ├── agent-gateway-egress.yaml        # Agent Gateway (egress) config
    ├── authz-extension.yaml             # CONTENT_AUTHZ ext_proc extension
    ├── authz-policy.yaml                # Authz policy → attaches shim to gateway
    └── setup-agent-registry.sh          # One-shot setup script
```
