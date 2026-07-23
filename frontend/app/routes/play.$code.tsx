import { useCallback, useEffect, useReducer, useRef, useState } from "react";
import { useNavigate } from "react-router";
import type { Route } from "./+types/play.$code";
import { useGameSocket } from "../lib/ws/useGameSocket";
import type { ServerMessage } from "../lib/ws/envelope";
import type {
  AnswerResult,
  LeaderboardEntry,
  QuestionEnded,
  QuestionReviewed,
  QuestionStarted,
} from "../lib/ws/types.gen";
import { AudioPlayer } from "../components/AudioPlayer";
import { Constellation } from "../components/Constellation";
import { OrbitTimer } from "../components/OrbitTimer";
import { Button, Panel } from "../components/ui";

export function meta({}: Route.MetaArgs) {
  return [{ title: "Quizmos — Playing" }];
}

const MAX_FREE_TEXT_ANSWER_LENGTH = 500;

function QuestionMedia({ mediaUrl, mediaType }: { mediaUrl?: string; mediaType?: string }) {
  if (!mediaUrl) return null;
  if (mediaType === "image") {
    return (
      <img
        src={mediaUrl}
        alt=""
        className="max-h-64 w-full rounded-xl border border-void-3 object-contain"
      />
    );
  }
  return <AudioPlayer src={mediaUrl} className="w-full" />;
}

type PlayState =
  | { phase: "lobby"; playerCount: number }
  | { phase: "question"; question: QuestionStarted }
  | { phase: "reveal"; ended: QuestionEnded; leaderboard: LeaderboardEntry[] | null }
  | { phase: "review"; review: QuestionReviewed }
  | { phase: "ended"; leaderboard: LeaderboardEntry[] }
  | { phase: "kicked" };

function reducer(state: PlayState, action: ServerMessage): PlayState {
  switch (action.type) {
    case "presence.playerJoined":
    case "presence.playerLeft":
      return state.phase === "lobby" ? { ...state, playerCount: action.payload.playerCount } : state;
    case "player.kicked":
      return { phase: "kicked" };
    case "game.started":
      return state;
    case "question.started":
      return { phase: "question", question: action.payload };
    case "question.ended":
      return { phase: "reveal", ended: action.payload, leaderboard: null };
    case "question.reviewed":
      return { phase: "review", review: action.payload };
    case "leaderboard.updated":
      return state.phase === "reveal" ? { ...state, leaderboard: action.payload.entries } : state;
    case "game.ended":
      return { phase: "ended", leaderboard: action.payload.finalLeaderboard };
    default:
      return state;
  }
}

export default function Play({ params }: Route.ComponentProps) {
  const code = params.code;
  const navigate = useNavigate();
  const [state, dispatch] = useReducer(reducer, { phase: "lobby", playerCount: 1 });
  const [hasAnswered, setHasAnswered] = useState(false);
  const [selectedOptionId, setSelectedOptionId] = useState<string | null>(null);
  const [freeTextValue, setFreeTextValue] = useState("");
  // answer.result can arrive well after the "question" phase ends — a
  // free-text grade might land during "reveal" or even later — so it's
  // tracked here rather than nested in the reducer's per-phase state.
  const [latestAnswerResult, setLatestAnswerResult] = useState<AnswerResult | null>(null);
  // Options carry no text past question.started, so we keep the last seen
  // question around to render option labels during the reveal phase.
  const [lastQuestion, setLastQuestion] = useState<QuestionStarted | null>(null);
  // onMessage's useCallback deps are intentionally empty (so the socket
  // never has to reconnect), so it can't close over fresh `state` — this
  // ref mirrors it for the one place that needs to read current phase.
  const stateRef = useRef(state);
  useEffect(() => {
    stateRef.current = state;
  }, [state]);

  const onMessage = useCallback((msg: ServerMessage) => {
    if (msg.type === "question.started") {
      // question.started isn't always a fresh question: resuming live
      // play after a review, or reconnecting mid-question, redelivers it
      // for a question the recipient may have already answered. The
      // server folds that into yourAnswer (rather than us guessing from
      // local state, which a reconnect/remount would lose anyway) — its
      // absence means genuinely not answered yet.
      const yours = msg.payload.yourAnswer;
      if (yours) {
        setHasAnswered(true);
        setSelectedOptionId(yours.optionId ?? null);
        setFreeTextValue(yours.text ?? "");
        setLatestAnswerResult({
          questionId: msg.payload.questionId,
          correct: yours.correct ?? false,
          pointsAwarded: yours.pointsAwarded ?? 0,
          totalScore: yours.pointsAwarded ?? 0,
          pending: yours.pending,
        });
      } else {
        setHasAnswered(false);
        setSelectedOptionId(null);
        setFreeTextValue("");
        setLatestAnswerResult(null);
      }
      setLastQuestion(msg.payload);
    }
    if (msg.type === "answer.result") {
      setLatestAnswerResult(msg.payload);
    }
    if (msg.type === "question.answersReset") {
      const current = stateRef.current;
      if (current.phase === "question" && current.question.questionId === msg.payload.questionId) {
        setHasAnswered(false);
        setSelectedOptionId(null);
        setFreeTextValue("");
        setLatestAnswerResult(null);
      }
    }
    if (msg.type === "player.kicked" || msg.type === "game.ended") {
      // The server closes this connection shortly after either message;
      // stop the socket from auto-reconnecting into a game that's over.
      disconnect();
    }
    dispatch(msg);
  }, []);
  const { status, send, disconnect } = useGameSocket(code, onMessage);

  function submitOption(optionId: string) {
    if (state.phase !== "question" || hasAnswered) return;
    setHasAnswered(true);
    setSelectedOptionId(optionId);
    send({ type: "answer.submit", payload: { questionId: state.question.questionId, optionId } });
  }

  function submitFreeText() {
    if (state.phase !== "question" || hasAnswered) return;
    const trimmed = freeTextValue.trim();
    if (!trimmed) return;
    setHasAnswered(true);
    send({ type: "answer.submit", payload: { questionId: state.question.questionId, text: trimmed } });
  }

  const statusColor =
    status === "open" ? "bg-aurora" : status === "connecting" ? "bg-starlight" : "bg-flare";

  return (
    <main className="relative z-0 flex min-h-screen flex-col items-center px-4 py-8">
      <div className="mb-8 flex items-center gap-2 font-mono text-xs text-dim">
        <span className={`h-1.5 w-1.5 rounded-full ${statusColor}`} aria-hidden="true" />
        <span className="tracking-widest">{code}</span>
      </div>

      <div className="flex w-full max-w-md flex-1 flex-col items-center justify-center">
        {state.phase === "lobby" && (
          <div className="text-center motion-safe:animate-[rise-in_0.5s_ease-out_both]">
            <h1 className="font-display text-2xl font-semibold text-paper sm:text-3xl">
              Waiting for the host to start…
            </h1>
            <p className="mt-3 font-mono text-sm text-dim">
              {state.playerCount} {state.playerCount === 1 ? "player" : "players"} in orbit
            </p>
          </div>
        )}

        {state.phase === "question" && (
          <div className="flex w-full flex-col gap-6 motion-safe:animate-[rise-in_0.4s_ease-out_both]">
            <div className="flex items-center gap-4">
              {state.question.timed && (
                <OrbitTimer key={state.question.questionId} totalSeconds={state.question.timeLimitSeconds} />
              )}
              <div>
                <p className="font-mono text-xs uppercase tracking-[0.2em] text-dim">
                  Question {state.question.questionIndex + 1} of {state.question.totalQuestions}
                </p>
                <h1 className="mt-1 font-display text-xl font-semibold text-paper sm:text-2xl">
                  {state.question.prompt}
                </h1>
              </div>
            </div>

            <QuestionMedia mediaUrl={state.question.mediaUrl} mediaType={state.question.mediaType} />

            {state.question.type === "free_text" ? (
              <div className="flex flex-col gap-2">
                <textarea
                  value={freeTextValue}
                  onChange={(e) => setFreeTextValue(e.target.value.slice(0, MAX_FREE_TEXT_ANSWER_LENGTH))}
                  disabled={hasAnswered}
                  maxLength={MAX_FREE_TEXT_ANSWER_LENGTH}
                  rows={4}
                  placeholder="Type your answer…"
                  className="w-full resize-none rounded-xl border border-void-3 bg-void-2/80 px-4 py-3 text-paper placeholder-dim/60 outline-none transition focus:border-aurora disabled:opacity-50"
                />
                <div className="flex items-center justify-between">
                  <span className="font-mono text-xs text-dim">
                    {freeTextValue.length}/{MAX_FREE_TEXT_ANSWER_LENGTH}
                  </span>
                  <Button onClick={submitFreeText} disabled={hasAnswered || !freeTextValue.trim()}>
                    Submit answer
                  </Button>
                </div>
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                {state.question.options.map((opt) => {
                  const selected = selectedOptionId === opt.id;
                  return (
                    <button
                      key={opt.id}
                      onClick={() => submitOption(opt.id)}
                      disabled={hasAnswered}
                      className={`min-h-16 rounded-xl border px-4 py-3 text-left font-medium transition disabled:cursor-not-allowed ${
                        selected
                          ? "border-aurora bg-aurora/10 text-paper"
                          : "border-void-3 bg-void-2/80 text-paper hover:border-starlight-dim disabled:opacity-50"
                      }`}
                    >
                      {opt.text}
                    </button>
                  );
                })}
              </div>
            )}

            {latestAnswerResult?.questionId === state.question.questionId && !latestAnswerResult.pending && (
              <p
                className={`text-center font-display text-lg font-semibold ${
                  latestAnswerResult.correct ? "text-aurora" : "text-flare"
                }`}
                role="status"
              >
                {latestAnswerResult.correct ? `Correct — +${latestAnswerResult.pointsAwarded}` : "Not quite"}
              </p>
            )}
            {latestAnswerResult?.questionId === state.question.questionId && latestAnswerResult.pending && (
              <p className="text-center font-mono text-xs text-dim" role="status">
                Submitted — awaiting the host's grade…
              </p>
            )}
            {hasAnswered && latestAnswerResult?.questionId !== state.question.questionId && (
              <p className="text-center font-mono text-xs text-dim" role="status">
                Answer locked in…
              </p>
            )}
          </div>
        )}

        {state.phase === "reveal" && lastQuestion && (
          <div className="flex w-full flex-col gap-6 motion-safe:animate-[rise-in_0.4s_ease-out_both]">
            <h1 className="text-center font-display text-xl font-semibold text-paper">
              {lastQuestion.prompt}
            </h1>
            <QuestionMedia mediaUrl={lastQuestion.mediaUrl} mediaType={lastQuestion.mediaType} />
            {lastQuestion.type === "free_text" ? (
              <div className="rounded-2xl border border-void-3 bg-void-2/60 p-5">
                {latestAnswerResult?.questionId === lastQuestion.questionId &&
                !latestAnswerResult.pending ? (
                  <p
                    className={`text-center font-display text-lg font-semibold ${
                      latestAnswerResult.correct ? "text-aurora" : "text-flare"
                    }`}
                  >
                    {latestAnswerResult.correct
                      ? `Correct — +${latestAnswerResult.pointsAwarded}`
                      : "Not quite"}
                  </p>
                ) : (
                  <p className="text-center text-sm text-dim">
                    The host is reviewing free-text answers by hand — your result will appear once
                    they've graded yours.
                  </p>
                )}
              </div>
            ) : (
              <ul className="flex flex-col gap-2">
                {lastQuestion.options.map((opt) => {
                  const isCorrect = opt.id === state.ended.correctOptionId;
                  const count =
                    state.ended.answerCounts.find((c) => c.optionId === opt.id)?.count ?? 0;
                  return (
                    <li
                      key={opt.id}
                      className={`flex items-center justify-between rounded-xl border px-4 py-3 ${
                        isCorrect ? "border-starlight bg-starlight/10" : "border-void-3 bg-void-2/60"
                      }`}
                    >
                      <span className={isCorrect ? "text-starlight" : "text-paper"}>
                        {isCorrect ? "★ " : ""}
                        {opt.text}
                      </span>
                      <span className="font-mono text-xs text-dim">{count}</span>
                    </li>
                  );
                })}
              </ul>
            )}
            <div>
              <p className="mb-3 font-mono text-xs uppercase tracking-[0.2em] text-dim">
                Standings
              </p>
              {state.leaderboard ? (
                <Constellation entries={state.leaderboard} />
              ) : (
                <p className="text-sm text-dim">Charting the next move…</p>
              )}
            </div>
          </div>
        )}

        {state.phase === "review" && (
          <div className="flex w-full flex-col gap-5 motion-safe:animate-[rise-in_0.4s_ease-out_both]">
            <div
              role="status"
              className="flex items-center justify-center gap-2 rounded-full border border-ember/40 bg-ember/10 px-4 py-2"
            >
              <span
                aria-hidden="true"
                className="h-2 w-2 shrink-0 rounded-full bg-ember motion-safe:animate-[twinkle_1.6s_ease-in-out_infinite]"
              />
              <span className="font-display text-sm font-semibold uppercase tracking-[0.2em] text-ember">
                Recap — not live
              </span>
            </div>
            <p className="text-center text-sm text-dim">
              The host is revisiting question {state.review.questionIndex + 1} of{" "}
              {state.review.totalQuestions}. Your answer can't change.
            </p>
            <div className="rounded-2xl border border-ember/30 bg-void-2/80 p-5 backdrop-blur-sm">
              <h1 className="text-center font-display text-xl font-semibold text-paper">
                {state.review.prompt}
              </h1>
              <QuestionMedia mediaUrl={state.review.mediaUrl} mediaType={state.review.mediaType} />
              {state.review.options.length > 0 && (
                <ul className="mt-4 flex flex-col gap-2">
                  {state.review.options.map((opt) => {
                    const isCorrect = opt.id === state.review.correctOptionId;
                    const count =
                      state.review.answerCounts.find((c) => c.optionId === opt.id)?.count ?? 0;
                    return (
                      <li
                        key={opt.id}
                        className={`flex items-center justify-between rounded-xl border px-4 py-3 ${
                          isCorrect ? "border-starlight bg-starlight/10" : "border-void-3 bg-void-2/60"
                        }`}
                      >
                        <span className={isCorrect ? "text-starlight" : "text-paper"}>
                          {isCorrect ? "★ " : ""}
                          {opt.text}
                        </span>
                        <span className="font-mono text-xs text-dim">{count}</span>
                      </li>
                    );
                  })}
                </ul>
              )}
            </div>
          </div>
        )}

        {state.phase === "ended" && (
          <div className="flex w-full flex-col items-center gap-6 motion-safe:animate-[rise-in_0.5s_ease-out_both]">
            <h1 className="text-center font-display text-2xl font-semibold text-paper sm:text-3xl">
              Final results
            </h1>
            <Panel className="w-full p-6">
              <Constellation entries={state.leaderboard} />
            </Panel>
            <Button variant="ghost" onClick={() => navigate("/")}>
              Return to base
            </Button>
          </div>
        )}

        {state.phase === "kicked" && (
          <div className="flex flex-col items-center text-center motion-safe:animate-[rise-in_0.5s_ease-out_both]">
            <h1 className="font-display text-2xl font-semibold text-flare sm:text-3xl">
              You've been removed from this game
            </h1>
            <p className="mt-3 text-sm text-dim">The host ended your spot in the lobby.</p>
            <Button variant="ghost" className="mt-6" onClick={() => navigate("/")}>
              Return to base
            </Button>
          </div>
        )}
      </div>
    </main>
  );
}
