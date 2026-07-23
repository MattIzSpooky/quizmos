-- Deleting a quiz now removes every game (and, transitively, its players
-- and answers) created from it, instead of being blocked by RESTRICT.
-- Question media in MinIO is cleaned up by the application layer
-- (Service.DeleteQuiz), since that's not something a SQL cascade can do.
ALTER TABLE games DROP CONSTRAINT games_quiz_id_fkey;
ALTER TABLE games
    ADD CONSTRAINT games_quiz_id_fkey
    FOREIGN KEY (quiz_id) REFERENCES quizzes(id) ON DELETE CASCADE;
