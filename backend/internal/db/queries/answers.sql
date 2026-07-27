-- name: CreateAnswer :one
INSERT INTO answers (game_id, question_id, player_id, selected_option_id, answer_text, is_correct, points_awarded)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAnswer :one
SELECT * FROM answers WHERE question_id = $1 AND player_id = $2;

-- name: GetAnswerByID :one
SELECT * FROM answers WHERE id = $1;

-- name: CountAnswersByOption :many
SELECT selected_option_id, count(*) AS answer_count
FROM answers
WHERE question_id = $1
GROUP BY selected_option_id;

-- name: GetAnswersForQuestion :many
SELECT * FROM answers WHERE game_id = $1 AND question_id = $2;

-- name: ListAnswerStatusesForQuestion :many
-- Every player's answer (if any) to one question in one game, keyed by
-- client_id — used to personalize a question.started broadcast to every
-- connected client in a single query instead of one GetPlayer+GetAnswer
-- pair per client.
SELECT p.client_id, a.*
FROM answers a
JOIN players p ON p.id = a.player_id
WHERE a.game_id = $1 AND a.question_id = $2;

-- name: ListFreeTextAnswersForQuestion :many
SELECT a.*, p.nickname, p.client_id
FROM answers a
JOIN players p ON p.id = a.player_id
WHERE a.game_id = $1 AND a.question_id = $2
ORDER BY a.answered_at ASC;

-- name: DeleteAnswersForQuestion :exec
DELETE FROM answers WHERE game_id = $1 AND question_id = $2;

-- name: GradeAnswer :one
UPDATE answers
SET is_correct = $2, points_awarded = $3
WHERE id = $1
RETURNING *;
