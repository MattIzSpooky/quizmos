-- name: CreateAnswer :one
INSERT INTO answers (game_id, question_id, player_id, selected_option_id, is_correct, points_awarded)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAnswer :one
SELECT * FROM answers WHERE question_id = $1 AND player_id = $2;

-- name: CountAnswersByOption :many
SELECT selected_option_id, count(*) AS answer_count
FROM answers
WHERE question_id = $1
GROUP BY selected_option_id;
