-- name: UpsertPlayer :one
INSERT INTO players (game_id, client_id, nickname)
VALUES ($1, $2, $3)
ON CONFLICT (game_id, client_id) DO UPDATE SET nickname = EXCLUDED.nickname
RETURNING *;

-- name: GetPlayer :one
SELECT * FROM players WHERE game_id = $1 AND client_id = $2;

-- name: ListPlayersByGame :many
SELECT * FROM players WHERE game_id = $1 ORDER BY joined_at ASC;

-- name: AddPlayerScore :exec
UPDATE players SET score = score + $2 WHERE id = $1;

-- name: LeaderboardByGame :many
SELECT id, client_id, nickname, score,
       RANK() OVER (ORDER BY score DESC) AS rank
FROM players
WHERE game_id = $1
ORDER BY score DESC, joined_at ASC;
