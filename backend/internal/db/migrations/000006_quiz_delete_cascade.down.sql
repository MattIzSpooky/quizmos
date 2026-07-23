ALTER TABLE games DROP CONSTRAINT games_quiz_id_fkey;
ALTER TABLE games
    ADD CONSTRAINT games_quiz_id_fkey
    FOREIGN KEY (quiz_id) REFERENCES quizzes(id) ON DELETE RESTRICT;
