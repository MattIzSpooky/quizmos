import { useState } from "react";
import { useNavigate } from "react-router";
import type { Route } from "./+types/home";
import { publicApi } from "../lib/api/client";
import { getClientId } from "../lib/client-id";
import { Button, Panel } from "../components/ui";

export function meta({}: Route.MetaArgs) {
  return [
    { title: "Quizmos" },
    { name: "description", content: "Join a live pub quiz" },
  ];
}

export default function Home() {
  const navigate = useNavigate();
  const [code, setCode] = useState("");
  const [nickname, setNickname] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleJoin(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    const { data, error: apiError } = await publicApi.POST("/games/join", {
      params: { header: { "X-Client-Id": getClientId() } },
      body: { code: code.trim().toUpperCase(), nickname: nickname.trim() },
    });
    setSubmitting(false);
    if (apiError || !data) {
      setError("No game found at that code. Check it and try again.");
      return;
    }
    navigate(`/play/${data.code}`);
  }

  return (
    <main className="relative z-0 flex min-h-screen flex-col items-center justify-center px-4 py-12">
      <div className="w-full max-w-sm motion-safe:animate-[rise-in_0.6s_ease-out_both]">
        <div className="mb-10 text-center">
          <h1 className="font-display text-4xl font-semibold tracking-tight text-paper sm:text-5xl">
            QUIZ<span className="text-starlight">MOS</span>
          </h1>
          <p className="mt-3 text-sm text-dim">
            Live trivia, beamed straight to your phone.
          </p>
        </div>

        <Panel className="p-6 sm:p-8">
          <form onSubmit={handleJoin} className="flex flex-col gap-5">
            <label className="flex flex-col gap-1.5">
              <span className="text-xs font-semibold uppercase tracking-[0.2em] text-dim">
                Game code
              </span>
              <input
                className="rounded-lg border border-void-3 bg-void px-4 py-3 text-center font-mono text-2xl uppercase tracking-[0.35em] text-starlight outline-none transition focus:border-aurora"
                placeholder="XXXXXX"
                value={code}
                maxLength={6}
                autoComplete="off"
                autoCapitalize="characters"
                onChange={(e) => setCode(e.target.value)}
                required
              />
            </label>
            <label className="flex flex-col gap-1.5">
              <span className="text-xs font-semibold uppercase tracking-[0.2em] text-dim">
                Nickname
              </span>
              <input
                className="rounded-lg border border-void-3 bg-void px-4 py-3 text-paper placeholder-dim/60 outline-none transition focus:border-aurora"
                placeholder="How should we call you?"
                value={nickname}
                maxLength={32}
                onChange={(e) => setNickname(e.target.value)}
                required
              />
            </label>

            {error && (
              <p role="alert" className="text-sm text-flare">
                {error}
              </p>
            )}

            <Button type="submit" disabled={submitting} className="w-full">
              {submitting ? "Joining…" : "Join game"}
            </Button>
          </form>
        </Panel>

        <div className="mt-8 text-center">
          <a
            href="/admin/login"
            className="font-mono text-xs text-dim underline decoration-void-3 underline-offset-4 transition hover:text-aurora"
          >
            Quiz admin login
          </a>
        </div>
      </div>
    </main>
  );
}
