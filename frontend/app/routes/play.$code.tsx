import { useCallback, useReducer, useState } from "react";
import type { Route } from "./+types/play.$code";
import { useGameSocket } from "../lib/ws/useGameSocket";
import type { ServerMessage } from "../lib/ws/envelope";
import type { AnswerResult, LeaderboardEntry, QuestionEnded, QuestionStarted } from "../lib/ws/types.gen";

export function meta({}: Route.MetaArgs) {
  return [{ title: "Quizmos — Playing" }];
}

type PlayState =
  | { phase: "lobby"; playerCount: number }
  | { phase: "question"; question: QuestionStarted; result: AnswerResult | null }
  | { phase: "reveal"; ended: QuestionEnded; leaderboard: LeaderboardEntry[] | null }
  | { phase: "ended"; leaderboard: LeaderboardEntry[] };

function reducer(state: PlayState, action: ServerMessage): PlayState {
  switch (action.type) {
    case "presence.playerJoined":
    case "presence.playerLeft":
      return state.phase === "lobby" ? { ...state, playerCount: action.payload.playerCount } : state;
    case "game.started":
      return state;
    case "question.started":
      return { phase: "question", question: action.payload, result: null };
    case "answer.result":
      return state.phase === "question" ? { ...state, result: action.payload } : state;
    case "question.ended":
      return { phase: "reveal", ended: action.payload, leaderboard: null };
    case "leaderboard.updated":
      return state.phase === "reveal" ? { ...state, leaderboard: action.payload.entries } : state;
    case "game.ended":
      return { phase: "ended", leaderboard: action.payload.finalLeaderboard };
    case "error":
      return state;
    default:
      return state;
  }
}

export default function Play({ params }: Route.ComponentProps) {
  const code = params.code;
  const [state, dispatch] = useReducer(reducer, { phase: "lobby", playerCount: 1 });
  const [answeredOptionId, setAnsweredOptionId] = useState<string | null>(null);

  const onMessage = useCallback((msg: ServerMessage) => {
    if (msg.type === "question.started") setAnsweredOptionId(null);
    dispatch(msg);
  }, []);
  const { status, send } = useGameSocket(code, onMessage);

  function submitAnswer(optionId: string) {
    if (state.phase !== "question" || answeredOptionId) return;
    setAnsweredOptionId(optionId);
    send({ type: "answer.submit", payload: { questionId: state.question.questionId, optionId } });
  }

  return (
    <main className="min-h-screen flex flex-col items-center justify-center gap-6 p-4 text-center">
      <p className="text-sm text-gray-400">
        Game {code} — {status === "open" ? "connected" : status}
      </p>

      {state.phase === "lobby" && (
        <>
          <h1 className="text-3xl font-bold">Waiting for the host to start…</h1>
          <p>{state.playerCount} player(s) in the lobby</p>
        </>
      )}

      {state.phase === "question" && (
        <div className="w-full max-w-md flex flex-col gap-4">
          <h1 className="text-2xl font-semibold">{state.question.prompt}</h1>
          <p className="text-sm text-gray-500">
            Question {state.question.questionIndex + 1} of {state.question.totalQuestions} ·{" "}
            {state.question.timeLimitSeconds}s
          </p>
          <div className="grid grid-cols-1 gap-3">
            {state.question.options.map((opt) => (
              <button
                key={opt.id}
                onClick={() => submitAnswer(opt.id)}
                disabled={!!answeredOptionId}
                className="border rounded px-4 py-3 text-left disabled:opacity-50"
              >
                {opt.text}
              </button>
            ))}
          </div>
          {state.result && (
            <p className={state.result.correct ? "text-green-600" : "text-red-600"}>
              {state.result.correct ? `Correct! +${state.result.pointsAwarded}` : "Not quite."}
            </p>
          )}
        </div>
      )}

      {state.phase === "reveal" && (
        <div className="w-full max-w-md flex flex-col gap-4">
          <h1 className="text-2xl font-semibold">Time's up!</h1>
          {state.leaderboard && <Leaderboard entries={state.leaderboard} />}
        </div>
      )}

      {state.phase === "ended" && (
        <div className="w-full max-w-md flex flex-col gap-4">
          <h1 className="text-3xl font-bold">Final results</h1>
          <Leaderboard entries={state.leaderboard} />
        </div>
      )}
    </main>
  );
}

function Leaderboard({ entries }: { entries: LeaderboardEntry[] }) {
  return (
    <ol className="flex flex-col gap-2">
      {entries.map((e) => (
        <li key={e.clientId} className="flex justify-between border rounded px-3 py-2">
          <span>
            #{e.rank} {e.nickname}
          </span>
          <span>{e.score}</span>
        </li>
      ))}
    </ol>
  );
}
