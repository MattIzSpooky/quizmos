import { useEffect, useRef, useState } from "react";
import type { Route } from "./+types/admin.games.$gameId";
import { adminApi } from "../lib/api/client";
import { useRequireAdmin } from "../lib/auth/useRequireAdmin";
import type { components } from "../lib/api/schema.gen";
import { useAdminGameSocket } from "../lib/ws/useAdminGameSocket";
import type { ServerMessage } from "../lib/ws/envelope";
import { AdminHeader } from "../components/AdminHeader";
import { AudioPlayer } from "../components/AudioPlayer";
import { Constellation } from "../components/Constellation";
import { QrCode } from "../components/QrCode";
import { Button, Panel } from "../components/ui";
import { playerColorHex } from "../lib/playerColors";

type AdminGameDetail = components["schemas"]["AdminGameDetail"];
type Question = components["schemas"]["Question"];
type FreeTextAnswer = components["schemas"]["FreeTextAnswer"];

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
  const [freeTextAnswers, setFreeTextAnswers] = useState<FreeTextAnswer[] | null>(null);
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

  async function loadFreeTextAnswers(questionId: string) {
    const { data } = await adminApi.GET("/admin/games/{gameId}/questions/{questionId}/answers", {
      params: { path: { gameId, questionId } },
    });
    if (data) setFreeTextAnswers(data);
  }

  // Fetches the current free_text question's submissions once; new answers
  // and grading updates arrive afterward over the websocket (see
  // handleSocketMessage's "freeTextAnswer.updated" case) instead of a poll.
  useEffect(() => {
    if (game?.status !== "in_progress" || displayedQuestion?.type !== "free_text") {
      setFreeTextAnswers(null);
      return;
    }
    loadFreeTextAnswers(displayedQuestion.id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game?.status, displayedQuestion?.id, displayedQuestion?.type]);

  function handleSocketMessage(msg: ServerMessage) {
    if (msg.type === "freeTextAnswer.updated") {
      if (msg.payload.questionId !== displayedQuestion?.id) return;
      setFreeTextAnswers((prev) => {
        const answer = msg.payload;
        const next = { ...answer };
        if (!prev) return [next];
        const index = prev.findIndex((a) => a.id === next.id);
        if (index === -1) return [...prev, next];
        return prev.map((a, i) => (i === index ? next : a));
      });
      return;
    }
    // Every other event (presence, question/leaderboard/game lifecycle,
    // player kicks) means the game-state snapshot is stale — refetch it
    // rather than re-deriving the same assembly logic client-side.
    load();
    if (msg.type === "game.ended") {
      // The server closes this connection shortly after game.ended anyway
      // (see ws.Hub.CloseRoom); stop it from auto-reconnecting into a room
      // that's now permanently idle.
      disconnectSocket();
    }
  }

  const { disconnect: disconnectSocket } = useAdminGameSocket(ready ? gameId : null, handleSocketMessage);

  async function gradeAnswer(answerId: string, correct: boolean) {
    await adminApi.POST("/admin/games/{gameId}/answers/{answerId}/grade", {
      params: { path: { gameId, answerId } },
      body: { correct },
    });
    if (displayedQuestion) await loadFreeTextAnswers(displayedQuestion.id);
  }

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
      color: p.color,
    }));

  return (
    <main className="relative z-0 mx-auto max-w-6xl px-4 py-8 sm:py-12">
      <AdminHeader back={{ to: "/admin/quizzes", label: "All quizzes" }} />

      <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-[minmax(0,1fr)_22rem] lg:gap-8">
        <div className="flex flex-col gap-6 lg:gap-8">
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
              <div className="mt-6 flex flex-col gap-6">
                <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                  <p className="font-mono text-xs uppercase tracking-[0.2em] text-dim">
                    Question {displayedIndex + 1} of {game.totalQuestions}
                    {isReviewing && <span className="text-ember"> · reviewing</span>}
                  </p>
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

                <div
                  key={displayedQuestion?.id ?? "loading"}
                  className="motion-safe:animate-[rise-in_0.35s_ease-out_both]"
                >
                  {displayedQuestion ? (
                    <div className="rounded-2xl border border-void-3 bg-void/40 p-5 sm:p-6">
                      <p className="font-mono text-[0.65rem] uppercase tracking-[0.2em] text-dim">
                        What players see
                      </p>
                      <h2 className="mt-2 font-display text-xl font-semibold text-paper sm:text-2xl">
                        {displayedQuestion.prompt}
                      </h2>
                      {displayedQuestion.mediaUrl &&
                        (displayedQuestion.mediaType === "image" ? (
                          <img
                            src={displayedQuestion.mediaUrl}
                            alt=""
                            className="mt-4 max-h-72 w-full rounded-xl border border-void-3 object-contain sm:max-h-80"
                          />
                        ) : (
                          <AudioPlayer src={displayedQuestion.mediaUrl} className="mt-4" />
                        ))}
                      {displayedQuestion.type === "free_text" ? (
                        <p className="mt-4 text-sm text-dim">
                          Players get a text field for this one — grade what comes in below.
                        </p>
                      ) : (
                        <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
                          {displayedQuestion.options.map((opt) => (
                            <div
                              key={opt.id}
                              className={`min-h-16 rounded-xl border px-4 py-3 text-left font-medium ${
                                opt.isCorrect
                                  ? "border-starlight bg-starlight/10 text-starlight"
                                  : "border-void-3 bg-void-2/80 text-paper"
                              }`}
                            >
                              {opt.isCorrect && <span aria-hidden="true">★ </span>}
                              {opt.text}
                            </div>
                          ))}
                        </div>
                      )}

                      {/* Only free_text questions need grading — multiple_choice
                          scores itself, so this whole block simply doesn't
                          render for those, rather than showing an empty
                          "nothing to grade" state. */}
                      {displayedQuestion.type === "free_text" && (
                        <div className="mt-6 border-t border-void-3 pt-5">
                          <p className="mb-3 font-mono text-[0.65rem] uppercase tracking-[0.2em] text-dim">
                            Grade answers
                          </p>
                          <div className="flex flex-col divide-y divide-void-3 rounded-xl border border-void-3 bg-void-2/60">
                            {!freeTextAnswers || freeTextAnswers.length === 0 ? (
                              <p className="px-4 py-6 text-center text-sm text-dim">
                                No answers submitted yet.
                              </p>
                            ) : (
                              freeTextAnswers.map((a) => (
                                <div
                                  key={a.id}
                                  className="flex flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
                                >
                                  <div className="min-w-0">
                                    <p className="font-mono text-xs uppercase tracking-[0.2em] text-dim">
                                      {a.nickname}
                                    </p>
                                    <p className="mt-1 break-words text-paper">{a.text}</p>
                                  </div>
                                  <div className="flex shrink-0 items-center gap-2">
                                    {a.graded && (
                                      <span
                                        className={`font-mono text-xs ${a.correct ? "text-aurora" : "text-flare"}`}
                                      >
                                        {a.correct ? `Correct · +${a.pointsAwarded}` : "Incorrect"}
                                      </span>
                                    )}
                                    <button
                                      type="button"
                                      onClick={() => gradeAnswer(a.id, true)}
                                      aria-label="Mark correct — full points"
                                      title="Mark correct — full points"
                                      className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-full border text-lg font-semibold transition ${
                                        a.graded && a.correct
                                          ? "border-aurora bg-aurora/20 text-aurora"
                                          : "border-void-3 text-dim hover:border-aurora hover:text-aurora"
                                      }`}
                                    >
                                      ✓
                                    </button>
                                    <button
                                      type="button"
                                      onClick={() => gradeAnswer(a.id, false)}
                                      aria-label="Mark incorrect — no points"
                                      title="Mark incorrect — no points"
                                      className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-full border text-lg font-semibold transition ${
                                        a.graded && !a.correct
                                          ? "border-flare bg-flare/20 text-flare"
                                          : "border-void-3 text-dim hover:border-flare hover:text-flare"
                                      }`}
                                    >
                                      ✕
                                    </button>
                                  </div>
                                </div>
                              ))
                            )}
                          </div>
                        </div>
                      )}
                    </div>
                  ) : (
                    <p className="text-sm text-dim">Loading question…</p>
                  )}
                </div>
              </div>
            )}

            {game.status === "ended" && <p className="mt-6 text-sm text-dim">This game has ended.</p>}
          </Panel>

          {game.status === "lobby" && (
            <Panel className="p-6 sm:p-8">
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
            <section>
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
                        aria-label="Reset answers for this question"
                        className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-void-3 text-flare/80 transition hover:border-flare hover:text-flare disabled:cursor-not-allowed disabled:opacity-40"
                      >
                        <svg
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="2"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          className="h-5 w-5"
                          aria-hidden="true"
                        >
                          <path d="m7 21-4.3-4.3c-1-1-1-2.5 0-3.4l9.6-9.6c1-1 2.5-1 3.4 0l5.6 5.6c1 1 1 2.5 0 3.4L13 21" />
                          <path d="M22 21H7" />
                          <path d="m5 11 9 9" />
                        </svg>
                      </button>
                    </div>
                  );
                })}
              </Panel>
            </section>
          )}
        </div>

        <aside className="lg:sticky lg:top-8">
          <h2 className="mb-3 font-mono text-xs uppercase tracking-[0.2em] text-dim">
            {game.status === "lobby" ? `Players (${game.playerCount})` : "Standings"}
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
                    <span className="flex min-w-0 items-center gap-2 truncate text-paper">
                      <span
                        aria-hidden="true"
                        className="h-2.5 w-2.5 shrink-0 rounded-full"
                        style={{ backgroundColor: playerColorHex(p.color) }}
                      />
                      <span className="truncate">
                        {p.nickname} {p.connected ? "🟢" : "⚪"}
                      </span>
                    </span>
                    <button
                      type="button"
                      disabled={busy}
                      onClick={() => kickPlayer(p.clientId)}
                      title="Kick from the lobby"
                      aria-label="Kick from the lobby"
                      className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-void-3 text-lg font-semibold text-flare/80 transition hover:border-flare hover:text-flare disabled:cursor-not-allowed disabled:opacity-40"
                    >
                      ✕
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
        </aside>
      </div>
    </main>
  );
}
