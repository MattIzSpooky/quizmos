import { useEffect, useRef, useState } from "react";
import type { Route } from "./+types/admin.games.$gameId";
import { adminApi } from "../lib/api/client";
import { useRequireAdmin } from "../lib/auth/useRequireAdmin";
import type { components } from "../lib/api/schema.gen";
import { AdminHeader } from "../components/AdminHeader";
import { Constellation } from "../components/Constellation";
import { QrCode } from "../components/QrCode";
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
  const [showQr, setShowQr] = useState(false);
  // Reviewing a question is a pure broadcast — it never moves the game's
  // actual current-question pointer (see backend ReviewQuestion) — so the
  // admin's own view has to track "what's on screen right now" separately
  // from `game.currentQuestionIndex`. null means "follow live play".
  const [viewedIndex, setViewedIndex] = useState<number | null>(null);
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

  const displayedIndex = viewedIndex ?? game?.currentQuestionIndex ?? 0;
  const displayedQuestion = questions?.[displayedIndex];
  const isReviewing = viewedIndex != null && viewedIndex !== game?.currentQuestionIndex;

  async function start() {
    await withLock(async () => {
      await adminApi.POST("/admin/games/{gameId}/start", { params: { path: { gameId } } });
      setViewedIndex(null);
      await load();
    });
  }

  async function nextQuestion() {
    await withLock(async () => {
      await adminApi.POST("/admin/games/{gameId}/next-question", { params: { path: { gameId } } });
      setViewedIndex(null); // follow live play again, not whatever was last reviewed
      await load();
    });
  }

  async function reviewQuestion(index: number) {
    await withLock(async () => {
      await adminApi.POST("/admin/games/{gameId}/review-question", {
        params: { path: { gameId } },
        body: { questionIndex: index },
      });
      setViewedIndex(index);
    });
  }

  async function resetAnswers(index: number, prompt: string) {
    if (!window.confirm(`Wipe every player's answer to "${prompt}"? This can't be undone.`)) return;
    await withLock(async () => {
      await adminApi.POST("/admin/games/{gameId}/reset-answers", {
        params: { path: { gameId } },
        body: { questionIndex: index },
      });
      await load();
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
          {game.status !== "lobby" && (
            <div className="text-center sm:text-right">
              <p className="font-mono text-[0.65rem] uppercase tracking-[0.2em] text-dim">
                Join code
              </p>
              <p className="font-mono text-2xl tracking-[0.3em] text-starlight">{game.code}</p>
            </div>
          )}
        </div>

        {game.status === "in_progress" && (
          <div className="mt-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0">
              <p className="font-mono text-xs uppercase tracking-[0.2em] text-dim">
                Question {displayedIndex + 1} of {game.totalQuestions}
                {isReviewing && <span className="text-ember"> · reviewing</span>}
              </p>
              {displayedQuestion ? (
                <>
                  <p className="mt-1 truncate font-display text-lg text-paper">
                    {displayedQuestion.prompt}
                  </p>
                  <ul className="mt-1 flex flex-wrap gap-x-4 gap-y-1">
                    {displayedQuestion.options.map((opt) => (
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
                disabled={busy || displayedIndex === 0}
                onClick={() => reviewQuestion(displayedIndex - 1)}
                title="Show the previous question again to players, read-only"
              >
                ← Previous
              </Button>
              {isReviewing ? (
                <Button
                  disabled={busy}
                  onClick={() => reviewQuestion(game.currentQuestionIndex ?? 0)}
                  title="Return to the live question and continue the game"
                >
                  Resume
                </Button>
              ) : (
                <Button disabled={busy} onClick={nextQuestion}>
                  Next question
                </Button>
              )}
              <Button variant="danger" disabled={busy} onClick={endGame}>
                End game
              </Button>
            </div>
          </div>
        )}

        {game.status === "ended" && <p className="mt-6 text-sm text-dim">This game has ended.</p>}
      </Panel>

      {game.status === "lobby" && (
        <Panel className="mt-6 p-6 sm:p-8">
          <div className="flex flex-col items-center gap-8 sm:flex-row sm:items-center sm:justify-between">
            <div className="text-center sm:text-left">
              <p className="font-mono text-xs uppercase tracking-[0.2em] text-dim">
                Invite players
              </p>
              <p className="mt-2 font-mono text-4xl tracking-[0.4em] text-starlight">{game.code}</p>
              <p className="mt-3 text-sm text-dim">
                Share the code, or scan the QR code to join instantly.
              </p>
              <button
                onClick={() => setShowQr((v) => !v)}
                className="mt-3 font-mono text-xs text-dim underline decoration-void-3 underline-offset-4 hover:text-aurora"
              >
                {showQr ? "Hide QR code" : "Show QR code"}
              </button>
            </div>
            {showQr && (
              <QrCode value={`${window.location.origin}/?code=${game.code}`} size={168} />
            )}
          </div>

          <div className="mt-8">
            <Button disabled={busy} onClick={start}>
              Start game
            </Button>
          </div>
        </Panel>
      )}

      {game.status === "in_progress" && questions && (
        <section className="mt-8">
          <h2 className="mb-3 font-mono text-xs uppercase tracking-[0.2em] text-dim">
            Questions — jump back to review one
          </h2>
          <Panel className="flex flex-col divide-y divide-void-3">
            {questions.map((q, i) => {
              const isCurrent = i === displayedIndex;
              const isReachable = i <= (game.currentQuestionIndex ?? -1);
              return (
                <div
                  key={q.id}
                  className={`flex items-center gap-3 px-4 py-3 transition first:rounded-t-2xl last:rounded-b-2xl ${
                    isCurrent ? "bg-starlight/10" : ""
                  }`}
                >
                  <button
                    type="button"
                    disabled={busy || !isReachable}
                    onClick={() => reviewQuestion(i)}
                    className={`flex min-w-0 flex-1 items-center gap-3 text-left transition disabled:cursor-not-allowed ${
                      isReachable ? "hover:opacity-80" : "opacity-40"
                    }`}
                  >
                    <span className="w-6 shrink-0 font-mono text-xs text-dim">{i + 1}</span>
                    <span className={`truncate ${isCurrent ? "text-starlight" : "text-paper"}`}>
                      {q.prompt}
                    </span>
                  </button>
                  <button
                    type="button"
                    disabled={busy || !isReachable}
                    onClick={() => resetAnswers(i, q.prompt)}
                    title="Wipe every player's answer to this question so it can be answered again"
                    className="shrink-0 font-mono text-xs text-flare/80 underline decoration-flare/30 underline-offset-4 hover:text-flare disabled:cursor-not-allowed disabled:opacity-40"
                  >
                    Reset answers
                  </button>
                </div>
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
