import { useState } from "react";
import { useNavigate } from "react-router";
import type { Route } from "./+types/home";
import { publicApi } from "../lib/api/client";
import { getClientId } from "../lib/client-id";

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
      setError("Couldn't join that game. Check the code and try again.");
      return;
    }
    navigate(`/play/${data.code}`);
  }

  return (
    <main className="min-h-screen flex flex-col items-center justify-center gap-8 p-4">
      <h1 className="text-4xl font-bold">Quizmos</h1>
      <form onSubmit={handleJoin} className="flex flex-col gap-4 w-full max-w-xs">
        <input
          className="border rounded px-3 py-2 text-center text-xl tracking-widest uppercase"
          placeholder="Game code"
          value={code}
          maxLength={6}
          onChange={(e) => setCode(e.target.value)}
          required
        />
        <input
          className="border rounded px-3 py-2"
          placeholder="Nickname"
          value={nickname}
          maxLength={32}
          onChange={(e) => setNickname(e.target.value)}
          required
        />
        {error && <p className="text-red-600 text-sm">{error}</p>}
        <button
          type="submit"
          disabled={submitting}
          className="bg-black text-white rounded px-3 py-2 disabled:opacity-50"
        >
          {submitting ? "Joining…" : "Join game"}
        </button>
      </form>
      <a href="/admin/login" className="text-sm text-gray-500 underline">
        Quiz admin login
      </a>
    </main>
  );
}
