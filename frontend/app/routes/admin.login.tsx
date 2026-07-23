import type { Route } from "./+types/admin.login";
import { login } from "../lib/auth/keycloak";
import { Button, Panel } from "../components/ui";

export function meta({}: Route.MetaArgs) {
  return [{ title: "Quizmos Admin — Login" }];
}

export default function AdminLogin() {
  return (
    <main className="relative z-0 flex min-h-screen flex-col items-center justify-center px-4">
      <Panel className="w-full max-w-sm p-8 text-center motion-safe:animate-[rise-in_0.5s_ease-out_both]">
        <h1 className="font-display text-2xl font-semibold text-paper">Quiz admin</h1>
        <p className="mt-2 text-sm text-dim">Sign in to author quizzes and run live games.</p>
        <Button onClick={() => login()} className="mt-6 w-full">
          Sign in with Keycloak
        </Button>
      </Panel>
    </main>
  );
}
