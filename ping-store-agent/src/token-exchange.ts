interface TokenExchangeResult {
  access_token: string;
  token_type: string;
  expires_in: number;
}

/**
 * Obtains an actor token for this agent via client_credentials grant.
 * The actor token identifies the agent (ping-store-agent) as the service
 * acting on behalf of the user in the delegation flow.
 */
async function getActorToken(): Promise<string> {
  const tokenEndpoint = requiredEnv("AIC_TOKEN_ENDPOINT");
  const clientId = requiredEnv("AGENT_CLIENT_ID");
  const clientSecret = requiredEnv("AGENT_CLIENT_SECRET");

  const params = new URLSearchParams({
    grant_type: "client_credentials",
    scope: process.env.ACTOR_SCOPE ?? "stripe_mcp:invoke email",
  });

  const basicAuth = Buffer.from(`${clientId}:${clientSecret}`).toString("base64");

  const res = await fetch(tokenEndpoint, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
      "Authorization": `Basic ${basicAuth}`,
    },
    body: params.toString(),
  });

  if (!res.ok) {
    const body = await res.text();
    throw new Error(`Actor token grant failed (${res.status}): ${body}`);
  }

  const json = await res.json() as { access_token: string };
  return json.access_token;
}

/**
 * RFC 8693 Token Exchange (Delegation): exchanges the user's access token
 * (subject_token) together with the agent's access token (actor_token) for
 * a delegated token.
 *
 * The resulting token carries:
 *   sub = original user (from the subject_token)
 *   act = { sub: AGENT_CLIENT_ID }  (the agent acting on behalf of the user)
 *
 * This lets PingOne Authorize know both WHO the request is for and WHAT
 * service is making the call.
 */
export async function exchangeForDelegatedToken(
  userAccessToken: string,
): Promise<TokenExchangeResult> {
  const tokenEndpoint = requiredEnv("AIC_TOKEN_ENDPOINT");
  const clientId = requiredEnv("AGENT_CLIENT_ID");
  const clientSecret = requiredEnv("AGENT_CLIENT_SECRET");

  const scope =
    process.env.TOKEN_EXCHANGE_SCOPE ??
    "stripe_mcp:invoke email";

  const audience =
    process.env.TOKEN_EXCHANGE_AUDIENCE ??
    "https://ping-gcp-agent-gateway.com";

  // Step 1: Get actor token for the agent (client_credentials)
  const actorToken = await getActorToken();

  // Step 2: Exchange subject token + actor token for a delegated token
  const params = new URLSearchParams({
    grant_type: "urn:ietf:params:oauth:grant-type:token-exchange",
    subject_token: userAccessToken,
    subject_token_type: "urn:ietf:params:oauth:token-type:access_token",
    actor_token: actorToken,
    actor_token_type: "urn:ietf:params:oauth:token-type:access_token",
    requested_token_type: "urn:ietf:params:oauth:token-type:access_token",
    audience,
    scope,
  });

  const basicAuth = Buffer.from(`${clientId}:${clientSecret}`).toString("base64");

  const res = await fetch(tokenEndpoint, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
      "Authorization": `Basic ${basicAuth}`,
    },
    body: params.toString(),
  });

  if (!res.ok) {
    const body = await res.text();
    throw new Error(`Token exchange (delegation) failed (${res.status}): ${body}`);
  }

  return (await res.json()) as TokenExchangeResult;
}

function requiredEnv(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`Missing required env var: ${name}`);
  return value;
}
