// Admin authentication: Keycloak via authorization-code + PKCE. The
// backend never issues tokens itself — it only validates the bearer token
// this flow produces (see backend/internal/auth/keycloak.go).
import { UserManager, WebStorageStateStore, type User } from "oidc-client-ts";

const KEYCLOAK_ISSUER =
  import.meta.env.VITE_KEYCLOAK_ISSUER ?? "http://localhost:8081/realms/quizmos";
const KEYCLOAK_CLIENT_ID = import.meta.env.VITE_KEYCLOAK_CLIENT_ID ?? "quizmos-frontend";

let manager: UserManager | null = null;

function getManager(): UserManager {
  if (typeof window === "undefined") {
    throw new Error("Keycloak auth is only available in the browser");
  }
  if (!manager) {
    manager = new UserManager({
      authority: KEYCLOAK_ISSUER,
      client_id: KEYCLOAK_CLIENT_ID,
      redirect_uri: `${window.location.origin}/admin/callback`,
      post_logout_redirect_uri: `${window.location.origin}/`,
      response_type: "code",
      scope: "openid profile roles",
      userStore: new WebStorageStateStore({ store: window.localStorage }),
    });
  }
  return manager;
}

export function login(): Promise<void> {
  return getManager().signinRedirect();
}

export function logout(): Promise<void> {
  return getManager().signoutRedirect();
}

export function handleLoginCallback(): Promise<User> {
  return getManager().signinRedirectCallback();
}

export async function getUser(): Promise<User | null> {
  return getManager().getUser();
}

export async function getAccessToken(): Promise<string | null> {
  const user = await getUser();
  if (!user || user.expired) return null;
  return user.access_token;
}

/**
 * Decodes realm_access.roles from the *access token*, not `user.profile`
 * (the ID token's claims): Keycloak's built-in "realm roles" mapper only
 * puts realm_access on the access token by default, so reading it from
 * the ID token silently sees no roles at all and every login looks
 * unauthorized — checking the access token is what actually matches what
 * the backend validates on each API call anyway.
 */
export function isAdmin(user: User | null): boolean {
  if (!user?.access_token) return false;
  const roles = decodeAccessTokenRoles(user.access_token);
  return roles.includes("quiz-admin");
}

function decodeAccessTokenRoles(accessToken: string): string[] {
  try {
    const payload = accessToken.split(".")[1];
    const base64 = payload.replace(/-/g, "+").replace(/_/g, "/");
    const padded = base64 + "=".repeat((4 - (base64.length % 4)) % 4);
    const json = atob(padded);
    const claims = JSON.parse(json) as { realm_access?: { roles?: string[] } };
    return claims.realm_access?.roles ?? [];
  } catch {
    return [];
  }
}
