-- name: CreateGame :one
INSERT INTO games (quiz_id, code, created_by)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetGame :one
SELECT * FROM games WHERE id = $1;

-- name: GetGameByCode :one
SELECT * FROM games WHERE code = $1;

-- name: GameCodeExists :one
SELECT EXISTS (SELECT 1 FROM games WHERE code = $1);

-- name: ListGames :many
SELECT * FROM games
WHERE sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')
ORDER BY created_at DESC;

-- name: StartGame :one
UPDATE games
SET status = 'in_progress', current_question_index = 0, started_at = now()
WHERE id = $1 AND status = 'lobby'
RETURNING *;

-- name: SetCurrentQuestionIndex :one
UPDATE games
SET current_question_index = $2
WHERE id = $1 AND status = 'in_progress'
RETURNING *;

-- name: EndGame :one
UPDATE games
SET status = 'ended', ended_at = now()
WHERE id = $1 AND status != 'ended'
RETURNING *;

-- name: CountPlayers :one
SELECT count(*) FROM players WHERE game_id = $1;
