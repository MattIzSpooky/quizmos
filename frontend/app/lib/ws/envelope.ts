// Hand-written wire envelope shared with the backend (see
// backend/internal/ws/envelope.go): {"type": "...", "payload": {...}}.
// Only the payload shapes are generated from api/asyncapi.yaml.
import type {
  PresencePlayerJoined,
  PresencePlayerLeft,
  GameStarted,
  QuestionStarted,
  QuestionEnded,
  AnswerResult,
  LeaderboardUpdated,
  GameEnded,
  ErrorPayload,
  AnswerSubmit,
} from "./types.gen";

export type ServerMessage =
  | { type: "presence.playerJoined"; payload: PresencePlayerJoined }
  | { type: "presence.playerLeft"; payload: PresencePlayerLeft }
  | { type: "game.started"; payload: GameStarted }
  | { type: "question.started"; payload: QuestionStarted }
  | { type: "question.ended"; payload: QuestionEnded }
  | { type: "answer.result"; payload: AnswerResult }
  | { type: "leaderboard.updated"; payload: LeaderboardUpdated }
  | { type: "game.ended"; payload: GameEnded }
  | { type: "error"; payload: ErrorPayload };

export type ClientMessage = { type: "answer.submit"; payload: AnswerSubmit };

export function parseServerMessage(raw: string): ServerMessage {
  const envelope = JSON.parse(raw) as { type: string; payload: unknown };
  return envelope as ServerMessage;
}

export function encodeClientMessage(msg: ClientMessage): string {
  return JSON.stringify(msg);
}
