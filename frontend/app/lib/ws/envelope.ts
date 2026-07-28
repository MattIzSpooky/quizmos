// Hand-written wire envelope shared with the backend (see
// backend/internal/ws/envelope.go): {"type": "...", "payload": {...}}.
// Only the payload shapes are generated from api/asyncapi.yaml.
import type {
  PresencePlayerJoined,
  PresencePlayerLeft,
  PlayerKicked,
  GameStarted,
  QuestionStarted,
  QuestionEnded,
  QuestionReviewed,
  QuestionAnswersReset,
  AnswerResult,
  LeaderboardUpdated,
  GameEnded,
  ErrorPayload,
  AnswerSubmit,
  FreeTextAnswerUpdated,
} from "./types.gen";

export type ServerMessage =
  | { type: "presence.playerJoined"; payload: PresencePlayerJoined }
  | { type: "presence.playerLeft"; payload: PresencePlayerLeft }
  | { type: "player.kicked"; payload: PlayerKicked }
  | { type: "game.started"; payload: GameStarted }
  | { type: "question.started"; payload: QuestionStarted }
  | { type: "question.ended"; payload: QuestionEnded }
  | { type: "question.reviewed"; payload: QuestionReviewed }
  | { type: "question.answersReset"; payload: QuestionAnswersReset }
  | { type: "answer.result"; payload: AnswerResult }
  | { type: "leaderboard.updated"; payload: LeaderboardUpdated }
  | { type: "game.ended"; payload: GameEnded }
  | { type: "error"; payload: ErrorPayload }
  // Admin-only — never sent on the player channel.
  | { type: "freeTextAnswer.updated"; payload: FreeTextAnswerUpdated };

export type ClientMessage = { type: "answer.submit"; payload: AnswerSubmit };

export function parseServerMessage(raw: string): ServerMessage {
  const envelope = JSON.parse(raw) as { type: string; payload: unknown };
  return envelope as ServerMessage;
}

export function encodeClientMessage(msg: ClientMessage): string {
  return JSON.stringify(msg);
}
