import type { Route } from "./+types/admin.login";
import { login } from "../lib/auth/keycloak";

export function meta({}: Route.MetaArgs) {
  return [{ title: "Quizmos Admin — Login" }];
}

export default function AdminLogin() {
  return (
    <main className="min-h-screen flex flex-col items-center justify-center gap-6">
      <h1 className="text-2xl font-bold">Quizmos Admin</h1>
      <button onClick={() => login()} className="bg-black text-white rounded px-4 py-2">
        Sign in with Keycloak
      </button>
    </main>
  );
}
