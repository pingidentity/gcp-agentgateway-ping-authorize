# Gemini Enterprise Agent Platform + PingAuthorize

Reference implementations demonstrating how **PingAuthorize** enforces fine-grained policy on AI agent tool calls within the **Gemini Enterprise Agent Platform** — intercepting every MCP `tools/call` before it reaches a backend service.

Two use cases are provided: a consumer-facing ingress pattern using the GCP Regional Load Balancer, and a machine-to-machine egress pattern using the GCP Agent Gateway.

---

## Use Case 1 — Ingress: AI Shopping Agent

**Directory:** [`ingress-public-lb-mcp/`](./ingress-public-lb-mcp/)

A consumer-facing storefront where an authenticated user converses with an AI shopping assistant. The agent acts on behalf of the user via RFC 8693 token exchange, and every Stripe tool call is authorized by PingAuthorize before execution.

**Flow:**
```
Browser (React UI)
  → ping-store-agent (Strands AI, external — runs outside GCP)
    → GCP Regional Load Balancer (Traffic Extension / ext_proc)
      → ping-authz-shim → PingAuthorize → PERMIT / DENY
        → stripe-mcp (Cloud Run)
```

**Key characteristics:**
- User authenticates via PingOne AIC (Authorization Code + PKCE)
- Agent performs RFC 8693 token exchange to produce a delegated token carrying both user and agent identity
- PingAuthorize receives the delegated token, tool name, and payment arguments on every call
- `stripe-mcp` is internal-only — unreachable without passing through the load balancer and policy check

---

## Use Case 2 — Egress: Identity Provisioning Agent

**Directory:** [`egress-registry-gw-mcp/`](./egress-registry-gw-mcp/)

A machine-to-machine provisioning system where an AI agent manages user accounts across two identity systems. The agent and both MCP servers are registered in **GCP Agent Registry**. The Agent Gateway (egress / Agent-to-Anywhere mode) sits in the call path, routing all agent-to-MCP traffic through PingAuthorize.

**Flow:**
```
ping-provisioner-agent (ADK Python / Gemini, Cloud Run)
  → GCP Agent Gateway — ping-authz-agent-gateway (egress, AGENT_TO_ANYWHERE)
    → ping-authz-shim (CONTENT_AUTHZ ext_proc) → PingAuthorize → PERMIT / DENY
      → pingone-aic-mcp (ForgeRock AIC REST API)
      → entra-mcp (Microsoft Graph API)
```

**Key characteristics:**
- No human in the loop — agent is invoked via `POST /provision` with a natural language instruction
- Agent and MCP servers are visible in GCP Agent Registry (Vertex AI console)
- PingAuthorize can enforce policies such as "deny `deprovision_user` unless agent is in approved-deprovisioners" or "block provisioning to domains not on an allowlist"
- Both identity backends expose the same four MCP tools (`provision_user`, `deprovision_user`, `update_user_status`, `list_users`), routed by the agent based on the instruction

---

## Common Pattern

Both use cases enforce policy via the same mechanism — a `ping-authz-shim` gRPC service attached to the network control plane at different layers:

| Use Case | Control Plane | Shim Attachment |
|---|---|---|
| UC1 — Ingress | GCP Regional Load Balancer | Traffic Extension (ext_proc callout on the URL map) |
| UC2 — Egress | GCP Agent Gateway | CONTENT_AUTHZ authz extension + authz policy |

In both cases, `ping-authz-shim` parses the MCP JSON-RPC body, extracts the token and tool arguments, and calls PingAuthorize for a `PERMIT` or `DENY` decision on every `tools/call`.
