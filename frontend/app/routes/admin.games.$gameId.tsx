import { useEffect, useRef, useState } from "react";
import type { Route } from "./+types/admin.games.$gameId";
import { adminApi } from "../lib/api/client";
import { useRequireAdmin } from "../lib/auth/useRequireAdmin";
import type { components } from "../lib/api/schema.gen";
import { AdminHeader } from "../components/AdminHeader";
import { Constellation } from "../components/Constellation";
import { Button, Panel } from "../components/ui";

type AdminGameDetail = components["schemas"]["AdminGameDetail"];
type Question = components["schemas"]["Question"];

export function meta({}: Route.MetaArgs) {
  return [{ title: "Quizmos Admin — Live game" }];
}

const STATUS_LABEL: Record<string, string> = {
  lobby: "In the lobby",
  in_progress: "Live",
  ended: "Ended",
};

export default function AdminGameControl({ params }: Route.ComponentProps) {
  const ready = useRequireAdmin();
  const gameId = params.gameId;
  const [game, setGame] = useState<AdminGameDetail | null>(null);
  const [questions, setQuestions] = useState<Question[] | null>(null);
  const [busy, setBusy] = useState(false);
  // A ref (not just the `busy` state) guards against a double-click
  // landing before React has re-rendered the disabled button: the ref is
  // checked synchronously, so the second click is dropped even within
  // the same event-loop tick as the first.
  const actionInFlight = useRef(false);

  async function withLock(action: () => Promise<void>) {
    if (actionInFlight.current) return;
    actionInFlight.current = true;
    setBusy(true);
    try {
      await action();
    } finally {
      actionInFlight.current = false;
      setBusy(false);
    }
  }

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

  // The quiz's questions don't change once a game is running, so fetch
  // them once — just to show the admin what's currently on screen for
  // players, next to the "next question" control.
  useEffect(() => {
    if (!game?.quizId || questions) return;
    adminApi
      .GET("/admin/quizzes/{quizId}", { params: { path: { quizId: game.quizId } } })
      .then(({ data }) => data && setQuestions(data.questions));
  }, [game?.quizId, questions]);

  const currentQuestion =
    game?.currentQuestionIndex != null ? questions?.[game.currentQuestionIndex] : undefined;

  async function start() {
    await withLock(async () => {
      await adminApi.POST("/admin/games/{gameId}/start", { params: { path: { gameId } } });
      await load();
    });
  }

  async function nextQuestion() {
    await withLock(async () => {
      await adminApi.POST("/admin/games/{gameId}/next-question", { params: { path: { gameId } } });
      await load();
    });
  }

  async function reviewQuestion(index: number) {
    await withLock(async () => {
      await adminApi.POST("/admin/games/{gameId}/review-question", {
        params: { path: { gameId } },
        body: { questionIndex: index },
      });
    });
  }

  async function endGame() {
    await withLock(async () => {
      await adminApi.POST("/admin/games/{gameId}/end", { params: { path: { gameId } } });
      await load();
    });
  }

  async function kickPlayer(clientId: string) {
    await withLock(async () => {
      await adminApi.DELETE("/admin/games/{gameId}/players/{clientId}", {
        params: { path: { gameId, clientId } },
      });
      await load();
    });
  }

  if (!ready || !game) return null;

  const standings = [...game.players]
    .sort((a, b) => b.score - a.score)
    .map((p, i) => ({
      clientId: p.clientId,
      nickname: p.nickname,
      score: p.score,
      rank: i + 1,
      connected: p.connected,
    }));

  return (
    <main className="relative z-0 mx-auto max-w-2xl px-4 py-8 sm:py-12">
      <AdminHeader back={{ to: "/admin/quizzes", label: "All quizzes" }} />

      <Panel className="p-5 sm:p-6">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 className="truncate font-display text-xl font-semibold text-paper">
              {game.quizTitle}
            </h1>
            <p className="mt-1 font-mono text-xs uppercase tracking-[0.2em] text-dim">
              {STATUS_LABEL[game.status] ?? game.status}
            </p>
          </div>
          <div className="text-center sm:text-right">
            <p className="font-mono text-[0.65rem] uppercase tracking-[0.2em] text-dim">
              Join code
            </p>
            <p className="font-mono text-3xl tracking-[0.3em] text-starlight">{game.code}</p>
          </div>
        </div>

        {game.status === "in_progress" && (
          <div className="mt-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0">
              <p className="font-mono text-xs uppercase tracking-[0.2em] text-dim">
                Question {(game.currentQuestionIndex ?? 0) + 1} of {game.totalQuestions}
              </p>
              {currentQuestion ? (
                <>
                  <p className="mt-1 truncate font-display text-lg text-paper">
                    {currentQuestion.prompt}
                  </p>
                  <ul className="mt-1 flex flex-wrap gap-x-4 gap-y-1">
                    {currentQuestion.options.map((opt) => (
                      <li
                        key={opt.id}
                        className={`text-sm ${opt.isCorrect ? "text-starlight" : "text-dim"}`}
                      >
                        {opt.isCorrect ? "★ " : ""}
                        {opt.text}
                      </li>
                    ))}
                  </ul>
                </>
              ) : (
                <p className="mt-1 text-sm text-dim">Loading question…</p>
              )}
            </div>
            <div className="flex shrink-0 flex-wrap gap-3">
              <Button
                variant="ghost"
                disabled={busy || (game.currentQuestionIndex ?? 0) === 0}
                onClick={() => reviewQuestion((game.currentQuestionIndex ?? 0) - 1)}
                title="Show this question's predecessor again to players, read-only"
              >
                ← Previous
              </Button>
              <Button disabled={busy} onClick={nextQuestion}>
                Next question
              </Button>
              <Button variant="danger" disabled={busy} onClick={endGame}>
                End game
              </Button>
            </div>
          </div>
        )}

        {game.status === "lobby" && (
          <div className="mt-6">
            <Button disabled={busy} onClick={start}>
              Start game
            </Button>
          </div>
        )}
        {game.status === "ended" && <p className="mt-6 text-sm text-dim">This game has ended.</p>}
      </Panel>

      {game.status === "in_progress" && questions && (
        <section className="mt-8">
          <h2 className="mb-3 font-mono text-xs uppercase tracking-[0.2em] text-dim">
            Questions — jump back to review one
          </h2>
          <Panel className="flex flex-col divide-y divide-void-3">
            {questions.map((q, i) => {
              const isCurrent = i === (game.currentQuestionIndex ?? -1);
              const isReachable = i <= (game.currentQuestionIndex ?? -1);
              return (
                <button
                  key={q.id}
                  type="button"
                  disabled={busy || !isReachable}
                  onClick={() => reviewQuestion(i)}
                  className={`flex items-center gap-3 px-4 py-3 text-left transition first:rounded-t-2xl last:rounded-b-2xl disabled:cursor-not-allowed ${
                    isCurrent ? "bg-starlight/10" : isReachable ? "hover:bg-void-3/50" : "opacity-40"
                  }`}
                >
                  <span className="w-6 shrink-0 font-mono text-xs text-dim">{i + 1}</span>
                  <span className={`truncate ${isCurrent ? "text-starlight" : "text-paper"}`}>
                    {q.prompt}
                  </span>
                </button>
              );
            })}
          </Panel>
        </section>
      )}

      <section className="mt-8">
        <h2 className="mb-3 font-mono text-xs uppercase tracking-[0.2em] text-dim">
          Players ({game.playerCount})
        </h2>
        {game.status === "lobby" ? (
          <Panel className="flex flex-col divide-y divide-void-3">
            {game.players.length === 0 ? (
              <p className="px-4 py-6 text-center text-sm text-dim">
                No signal yet — share the join code to get people in.
              </p>
            ) : (
              game.players.map((p) => (
                <div key={p.clientId} className="flex items-center justify-between gap-3 px-4 py-3">
                  <span className="truncate text-paper">
                    {p.nickname} {p.connected ? "🟢" : "⚪"}
                  </span>
                  <button
                    disabled={busy}
                    onClick={() => kickPlayer(p.clientId)}
                    className="shrink-0 font-mono text-xs text-flare/80 underline decoration-flare/30 underline-offset-4 hover:text-flare disabled:opacity-40"
                  >
                    Kick
                  </button>
                </div>
              ))
            )}
          </Panel>
        ) : (
          <Panel className="p-5">
            <Constellation entries={standings} />
          </Panel>
        )}
      </section>
    </main>
  );
}
