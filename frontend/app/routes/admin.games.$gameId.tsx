import { useEffect, useState } from "react";
import type { Route } from "./+types/admin.games.$gameId";
import { adminApi } from "../lib/api/client";
import { useRequireAdmin } from "../lib/auth/useRequireAdmin";
import type { components } from "../lib/api/schema.gen";

type AdminGameDetail = components["schemas"]["AdminGameDetail"];

export function meta({}: Route.MetaArgs) {
  return [{ title: "Quizmos Admin — Live game" }];
}

export default function AdminGameControl({ params }: Route.ComponentProps) {
  const ready = useRequireAdmin();
  const gameId = params.gameId;
  const [game, setGame] = useState<AdminGameDetail | null>(null);
  const [busy, setBusy] = useState(false);

  async function load() {
    const { data } = await adminApi.GET("/admin/games/{gameId}", { params: { path: { gameId } } });
    if (data) setGame(data);
  }

  useEffect(() => {
    if (!ready) return;
    load();
    const interval = setInterval(load, 2000);
    return () => clearInterval(interval);
  }, [ready]);

  async function start() {
    setBusy(true);
    await adminApi.POST("/admin/games/{gameId}/start", { params: { path: { gameId } } });
    setBusy(false);
    load();
  }

  async function nextQuestion() {
    setBusy(true);
    await adminApi.POST("/admin/games/{gameId}/next-question", { params: { path: { gameId } } });
    setBusy(false);
    load();
  }

  async function endGame() {
    setBusy(true);
    await adminApi.POST("/admin/games/{gameId}/end", { params: { path: { gameId } } });
    setBusy(false);
    load();
  }

  if (!ready || !game) return null;

  return (
    <main className="max-w-2xl mx-auto p-6 flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-bold">{game.quizTitle}</h1>
        <p className="text-gray-500">
          Join code <span className="font-mono text-lg tracking-widest">{game.code}</span> · status:{" "}
          {game.status}
        </p>
      </div>

      <div className="flex gap-2">
        {game.status === "lobby" && (
          <button disabled={busy} onClick={start} className="bg-black text-white rounded px-4 py-2">
            Start game
          </button>
        )}
        {game.status === "in_progress" && (
          <>
            <button disabled={busy} onClick={nextQuestion} className="bg-black text-white rounded px-4 py-2">
              Next question ({(game.currentQuestionIndex ?? 0) + 1}/{game.totalQuestions})
            </button>
            <button disabled={busy} onClick={endGame} className="border rounded px-4 py-2">
              End game
            </button>
          </>
        )}
        {game.status === "ended" && <p>Game over.</p>}
      </div>

      <section>
        <h2 className="font-semibold mb-2">Players ({game.playerCount})</h2>
        <ul className="flex flex-col gap-1">
          {game.players.map((p) => (
            <li key={p.clientId} className="flex justify-between border rounded px-3 py-2">
              <span>
                {p.nickname} {p.connected ? "🟢" : "⚪"}
              </span>
              <span>{p.score}</span>
            </li>
          ))}
        </ul>
      </section>
    </main>
  );
}
