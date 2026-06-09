import { createRemoteJWKSet, jwtVerify, type JWTVerifyGetKey } from "jose";
import type { TokenExchangeResult } from "./util.js";
import { basicAuthHeader, HttpError } from "./util.js";
import { PINGONE_AIC_ISSUER, AGENT_CLIENT_ID, AGENT_CLIENT_SECRET, AGENT_REQUIRED_SCOPES, AGENT_GATEWAY_URL } from "./config.js";

/** Strips the default :443 port from HTTPS URLs for consistent comparison. */
const normalizeUrl = (url: string) => url.replace(/:443(?=\/)/, "");

/** Discovers the JWKS URI from the OIDC discovery endpoint and creates a remote key set. */
const discoverJwks = async (): Promise<JWTVerifyGetKey> => {
  const res = await fetch(`${PINGONE_AIC_ISSUER}/.well-known/openid-configuration`);
  if (!res.ok) throw new Error(`OIDC discovery failed (${res.status})`);
  const { jwks_uri } = await res.json() as { jwks_uri: string };
  return createRemoteJWKSet(new URL(jwks_uri));
};

const jwksPromise = discoverJwks();
const tokenEndpoint = `${PINGONE_AIC_ISSUER}/access_token`;
const authHeader = basicAuthHeader(AGENT_CLIENT_ID, AGENT_CLIENT_SECRET);
const requiredScopes = AGENT_REQUIRED_SCOPES.split(" ");

/** Verifies the subject token's signature, issuer, audience, scopes, and may_act claim. Returns the user's sub. */
export const validateSubjectToken = async (token: string): Promise<string> => {
  const jwks = await jwksPromise;
  const { payload } = await jwtVerify(token, jwks, {
    audience: AGENT_CLIENT_ID,
  }).catch(() => { throw new HttpError(401, "Invalid or expired token"); });

  if (normalizeUrl(String(payload.iss)) !== normalizeUrl(PINGONE_AIC_ISSUER))
    throw new HttpError(401, `Unexpected issuer: ${payload.iss}`);
  if (!payload.sub)
    throw new HttpError(401, "Token missing sub claim");

  const tokenScopes = String(payload.scope ?? "").split(" ");
  const missing = requiredScopes.filter((s) => !tokenScopes.includes(s));
  if (missing.length)
    throw new HttpError(403, `Missing required scope(s): ${missing.join(", ")}`);

  const mayAct = payload.may_act as Record<string, string> | undefined;
  if (!mayAct || mayAct.client_id !== AGENT_CLIENT_ID)
    throw new HttpError(403, `Agent ${AGENT_CLIENT_ID} is not authorized to act on behalf of this user`);

  return payload.sub;
};

/** Posts a grant request to the PingOne AIC token endpoint. */
const postTokenRequest = async (params: URLSearchParams, label: string): Promise<TokenExchangeResult> => {
  const res = await fetch(tokenEndpoint, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded", "Authorization": authHeader },
    body: params.toString(),
  });
  if (!res.ok) throw new Error(`${label} failed (${res.status}): ${await res.text()}`);
  return res.json() as Promise<TokenExchangeResult>;
};

/** Obtains an actor token identifying this agent via client_credentials grant. */
const fetchActorToken = async (): Promise<string> => {
  const result = await postTokenRequest(
    new URLSearchParams({ grant_type: "client_credentials", scope: AGENT_REQUIRED_SCOPES }),
    "Actor token grant",
  );
  return result.access_token;
};

/** Exchanges the user's access token + agent actor token for a delegated token (RFC 8693). */
export const exchangeDelegatedToken = async (userToken: string): Promise<TokenExchangeResult> => {
  const actorToken = await fetchActorToken();
  return postTokenRequest(
    new URLSearchParams({
      grant_type: "urn:ietf:params:oauth:grant-type:token-exchange",
      subject_token: userToken,
      subject_token_type: "urn:ietf:params:oauth:token-type:access_token",
      actor_token: actorToken,
      actor_token_type: "urn:ietf:params:oauth:token-type:access_token",
      requested_token_type: "urn:ietf:params:oauth:token-type:access_token",
      audience: AGENT_GATEWAY_URL,
      scope: AGENT_REQUIRED_SCOPES,
    }),
    "Token exchange",
  );
};
