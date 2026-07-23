// The anonymous player identity: generated once in the browser and
// persisted in localStorage. No backend round-trip, no registration.
const STORAGE_KEY = "quizmos.client_id";

export function getClientId(): string {
  if (typeof window === "undefined") {
    // SSR: callers on the server should not need a client id; routes that
    // do (join, play) are client-rendered interactions.
    return "";
  }
  let id = window.localStorage.getItem(STORAGE_KEY);
  if (!id) {
    id = crypto.randomUUID();
    window.localStorage.setItem(STORAGE_KEY, id);
  }
  return id;
}
