-- name: CreateQuestion :one
INSERT INTO questions (quiz_id, type, prompt, position, time_limit_seconds, points)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetQuestion :one
SELECT * FROM questions WHERE id = $1 AND quiz_id = $2;

-- name: GetQuestionByID :one
SELECT * FROM questions WHERE id = $1;

-- name: ListQuestionsByQuiz :many
SELECT * FROM questions WHERE quiz_id = $1 ORDER BY position ASC;

-- name: NextQuestionPosition :one
SELECT COALESCE(MAX(position) + 1, 0)::int FROM questions WHERE quiz_id = $1;

-- name: UpdateQuestion :one
UPDATE questions
SET prompt             = COALESCE(sqlc.narg('prompt'), prompt),
    time_limit_seconds = COALESCE(sqlc.narg('time_limit_seconds'), time_limit_seconds),
    points              = COALESCE(sqlc.narg('points'), points),
    updated_at          = now()
WHERE id = sqlc.arg('id') AND quiz_id = sqlc.arg('quiz_id')
RETURNING *;

-- name: SetQuestionPosition :exec
UPDATE questions SET position = $3 WHERE id = $1 AND quiz_id = $2;

-- name: SetQuestionMedia :one
UPDATE questions
SET media_key = $3, media_type = $4, updated_at = now()
WHERE id = $1 AND quiz_id = $2
RETURNING *;

-- name: ClearQuestionMedia :one
UPDATE questions
SET media_key = NULL, media_type = NULL, updated_at = now()
WHERE id = $1 AND quiz_id = $2
RETURNING *;

-- name: DeleteQuestion :execrows
DELETE FROM questions WHERE id = $1 AND quiz_id = $2;

-- name: CreateQuestionOption :one
INSERT INTO question_options (question_id, text, is_correct, position)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListOptionsByQuestion :many
SELECT * FROM question_options WHERE question_id = $1 ORDER BY position ASC;

-- name: ListOptionsByQuestionIDs :many
SELECT * FROM question_options WHERE question_id = ANY(sqlc.arg('question_ids')::uuid[]) ORDER BY question_id, position ASC;

-- name: DeleteOptionsByQuestion :exec
DELETE FROM question_options WHERE question_id = $1;

-- name: GetOption :one
SELECT * FROM question_options WHERE id = $1;
