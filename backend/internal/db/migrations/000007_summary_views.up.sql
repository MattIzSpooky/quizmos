-- Views for the two "row plus a couple of aggregates" shapes the app reads
-- constantly (a game summarized with its quiz's title/timed and live
-- player/question counts; a quiz summarized with its question count), so a
-- single SELECT replaces what used to be 2-3 sequential round trips.
CREATE VIEW game_summaries AS
SELECT g.*,
       qz.title AS quiz_title,
       qz.timed AS quiz_timed,
       (SELECT count(*) FROM players  p WHERE p.game_id  = g.id)::int      AS player_count,
       (SELECT count(*) FROM questions q WHERE q.quiz_id = g.quiz_id)::int AS total_questions
FROM games g
JOIN quizzes qz ON qz.id = g.quiz_id;

CREATE VIEW quiz_summaries AS
SELECT z.*,
       (SELECT count(*) FROM questions q WHERE q.quiz_id = z.id)::int AS question_count
FROM quizzes z;
