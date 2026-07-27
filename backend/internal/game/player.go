package game

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mattizspooky/quizmos/backend/internal/core"
	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
)

// Join only admits new players while the game is in the lobby. Once it's
// in_progress, a late join would drop someone into a quiz they never saw
// the earlier questions of with a clean scoreboard entry anyway, so
// there's no useful "late join" here — and once ended there's nothing to
// join at all. Reconnecting mid-game (e.g. a dropped connection) doesn't
// go through this path: it's a websocket concern against a player row
// that already exists, not a fresh join.
func (s *Service) Join(ctx context.Context, code string, clientID uuid.UUID, nickname, color string) (JoinResult, error) {
	ctx, span := tracer.Start(ctx, "game.Join")
	defer span.End()

	g, err := s.q.GetGameByCode(ctx, code)
	if err != nil {
		if err == pgx.ErrNoRows {
			return JoinResult{}, core.ErrNotFound
		}
		return JoinResult{}, err
	}
	if g.Status != "lobby" {
		return JoinResult{}, core.ErrConflict
	}
	player, err := s.q.UpsertPlayer(ctx, db.UpsertPlayerParams{
		GameID: g.ID, ClientID: clientID, Nickname: nickname, Color: NormalizePlayerColor(color),
	})
	if err != nil {
		return JoinResult{}, err
	}
	playersJoined.Add(ctx, 1)
	return JoinResult{Game: g, Player: player}, nil
}

// Kick removes a player from a game's lobby. It's lobby-only — removing
// someone mid-round would orphan any in-flight answer and skew scoring —
// and it's not a ban: nothing stops the same client_id from joining again
// afterward, same as anyone else.
func (s *Service) Kick(ctx context.Context, gameID, clientID uuid.UUID) error {
	ctx, span := tracer.Start(ctx, "game.Kick")
	defer span.End()

	g, err := s.q.GetGame(ctx, gameID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return core.ErrNotFound
		}
		return err
	}
	if g.Status != "lobby" {
		return core.ErrConflict
	}
	n, err := s.q.DeletePlayer(ctx, db.DeletePlayerParams{GameID: gameID, ClientID: clientID})
	if err != nil {
		return err
	}
	if n == 0 {
		return core.ErrNotFound
	}
	playersKicked.Add(ctx, 1)
	return nil
}

// GetPlayerByCode looks up an existing player by (code, client_id) without
// creating one — used to authorize a websocket connection, which must
// never create player identity itself.
func (s *Service) GetPlayerByCode(ctx context.Context, code string, clientID uuid.UUID) (db.Game, db.Player, error) {
	ctx, span := tracer.Start(ctx, "game.GetPlayerByCode")
	defer span.End()

	g, err := s.q.GetGameByCode(ctx, code)
	if err != nil {
		if err == pgx.ErrNoRows {
			return db.Game{}, db.Player{}, core.ErrNotFound
		}
		return db.Game{}, db.Player{}, err
	}
	player, err := s.q.GetPlayer(ctx, db.GetPlayerParams{GameID: g.ID, ClientID: clientID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return db.Game{}, db.Player{}, core.ErrNotFound
		}
		return db.Game{}, db.Player{}, err
	}
	return g, player, nil
}
