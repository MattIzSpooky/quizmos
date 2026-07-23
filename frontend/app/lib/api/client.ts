import createClient, { type Middleware } from "openapi-fetch";
import type { paths } from "./schema.gen";
import { getClientId } from "../client-id";
import { getAccessToken } from "../auth/keycloak";

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080/api";

const clientIdMiddleware: Middleware = {
  onRequest({ request }) {
    const id = getClientId();
    if (id) request.headers.set("X-Client-Id", id);
    return request;
  },
};

const authMiddleware: Middleware = {
  async onRequest({ request }) {
    const token = await getAccessToken();
    if (token) request.headers.set("Authorization", `Bearer ${token}`);
    return request;
  },
};

/** For public, unauthenticated endpoints (join, public game/leaderboard lookups). */
export const publicApi = createClient<paths>({ baseUrl: BASE_URL });
publicApi.use(clientIdMiddleware);

/** For /admin/* endpoints; attaches the Keycloak bearer token. */
export const adminApi = createClient<paths>({ baseUrl: BASE_URL });
adminApi.use(authMiddleware);
