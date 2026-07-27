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

-- name: GetGameSummary :one
SELECT * FROM game_summaries WHERE id = $1;

-- name: GetGameSummaryByCode :one
SELECT * FROM game_summaries WHERE code = $1;

-- name: ListGames :many
SELECT * FROM game_summaries
WHERE sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')
ORDER BY created_at DESC;

-- name: ListGamesByQuiz :many
SELECT * FROM games WHERE quiz_id = $1;

-- name: StartGame :one
WITH updated AS (
    UPDATE games
    SET status = 'in_progress', current_question_index = 0, started_at = now()
    WHERE games.id = $1 AND games.status = 'lobby'
    RETURNING *
)
SELECT updated.*,
       qz.title AS quiz_title,
       qz.timed AS quiz_timed,
       (SELECT count(*) FROM players  p WHERE p.game_id  = updated.id)::int      AS player_count,
       (SELECT count(*) FROM questions q WHERE q.quiz_id = updated.quiz_id)::int AS total_questions
FROM updated
JOIN quizzes qz ON qz.id = updated.quiz_id;

-- name: SetCurrentQuestionIndex :one
WITH updated AS (
    UPDATE games
    SET current_question_index = $2
    WHERE games.id = $1 AND games.status = 'in_progress'
    RETURNING *
)
SELECT updated.*,
       qz.title AS quiz_title,
       qz.timed AS quiz_timed,
       (SELECT count(*) FROM players  p WHERE p.game_id  = updated.id)::int      AS player_count,
       (SELECT count(*) FROM questions q WHERE q.quiz_id = updated.quiz_id)::int AS total_questions
FROM updated
JOIN quizzes qz ON qz.id = updated.quiz_id;

-- name: EndGame :one
WITH updated AS (
    UPDATE games
    SET status = 'ended', ended_at = now()
    WHERE games.id = $1 AND games.status != 'ended'
    RETURNING *
)
SELECT updated.*,
       qz.title AS quiz_title,
       qz.timed AS quiz_timed,
       (SELECT count(*) FROM players  p WHERE p.game_id  = updated.id)::int      AS player_count,
       (SELECT count(*) FROM questions q WHERE q.quiz_id = updated.quiz_id)::int AS total_questions
FROM updated
JOIN quizzes qz ON qz.id = updated.quiz_id;

-- name: CountPlayers :one
SELECT count(*) FROM players WHERE game_id = $1;
