-- name: CreateQuiz :one
INSERT INTO quizzes (title, description, created_by, timed)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetQuiz :one
SELECT * FROM quizzes WHERE id = $1;

-- name: GetQuizSummary :one
SELECT * FROM quiz_summaries WHERE id = $1;

-- name: ListQuizSummaries :many
SELECT * FROM quiz_summaries ORDER BY created_at DESC;

-- name: UpdateQuiz :one
-- Not folded into a single-round-trip CTE like the games.sql mutations
-- below: sqlc's analyzer (v1.31.1) can't resolve sqlc.narg()'d columns
-- once the statement also joins in another relation for the question
-- count, even fully qualified. Stays two round trips (this, then
-- GetQuizSummary) rather than fight the tool further.
UPDATE quizzes
SET title       = COALESCE(sqlc.narg('title'), title),
    description = COALESCE(sqlc.narg('description'), description),
    timed       = COALESCE(sqlc.narg('timed'), timed),
    updated_at  = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteQuiz :execrows
DELETE FROM quizzes WHERE id = $1;

-- name: CountQuizQuestions :one
SELECT count(*) FROM questions WHERE quiz_id = $1;
