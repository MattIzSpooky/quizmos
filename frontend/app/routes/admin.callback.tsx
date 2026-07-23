import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import type { Route } from "./+types/admin.callback";
import { handleLoginCallback } from "../lib/auth/keycloak";

export function meta({}: Route.MetaArgs) {
  return [{ title: "Signing in…" }];
}

export default function AdminCallback() {
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    handleLoginCallback()
      .then(() => navigate("/admin/quizzes", { replace: true }))
      .catch((err) => setError(String(err)));
  }, [navigate]);

  return (
    <main className="relative z-0 flex min-h-screen flex-col items-center justify-center gap-3 px-4 text-center">
      {error ? (
        <>
          <p className="font-display text-lg font-semibold text-flare">Sign-in failed</p>
          <p className="text-sm text-dim">{error}</p>
        </>
      ) : (
        <p className="font-mono text-sm text-dim">Signing in…</p>
      )}
    </main>
  );
}
