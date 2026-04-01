interface TokenExchangeResult {
  access_token: string;
  token_type: string;
  expires_in: number;
}

/**
 * RFC 8693 Token Exchange: swaps a user's access token for a delegated token.
 *
 * The resulting token carries:
 *   sub = original user (from the subject_token)
 *   act = { sub: AGENT_CLIENT_ID }  (the agent acting on behalf of the user)
 *
 * This lets the MCP server (via PingOne Authorize) know both WHO the request
 * is for and WHAT service is making the call.
 */
export async function exchangeForDelegatedToken(
  userAccessToken: string,
): Promise<TokenExchangeResult> {
  const tokenEndpoint = requiredEnv("AIC_TOKEN_ENDPOINT");
  const clientId = requiredEnv("AGENT_CLIENT_ID");
  const clientSecret = requiredEnv("AGENT_CLIENT_SECRET");
  const scope =
    process.env.TOKEN_EXCHANGE_SCOPE ??
    "openid profile email stripe_mcp:invoke";

  const params = new URLSearchParams({
    grant_type: "urn:ietf:params:oauth:grant-type:token-exchange",
    subject_token: userAccessToken,
    subject_token_type: "urn:ietf:params:oauth:token-type:access_token",
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
    throw new Error(
      `Token exchange failed (${res.status}): ${body}`,
    );
  }

  return (await res.json()) as TokenExchangeResult;
}

function requiredEnv(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`Missing required env var: ${name}`);
  return value;
}
