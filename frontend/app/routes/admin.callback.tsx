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
    <main className="min-h-screen flex items-center justify-center">
      {error ? <p className="text-red-600">Sign-in failed: {error}</p> : <p>Signing in…</p>}
    </main>
  );
}
