-- name: CreateQuiz :one
INSERT INTO quizzes (title, description, created_by)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetQuiz :one
SELECT * FROM quizzes WHERE id = $1;

-- name: ListQuizzes :many
SELECT * FROM quizzes ORDER BY created_at DESC;

-- name: UpdateQuiz :one
UPDATE quizzes
SET title       = COALESCE(sqlc.narg('title'), title),
    description = COALESCE(sqlc.narg('description'), description),
    updated_at  = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteQuiz :execrows
DELETE FROM quizzes WHERE id = $1;

-- name: CountQuizQuestions :one
SELECT count(*) FROM questions WHERE quiz_id = $1;
