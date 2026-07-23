ALTER TABLE answers DROP CONSTRAINT answers_option_xor_text;
ALTER TABLE answers DROP COLUMN answer_text;
ALTER TABLE answers ALTER COLUMN is_correct SET NOT NULL;
ALTER TABLE answers ALTER COLUMN selected_option_id SET NOT NULL;

ALTER TABLE questions DROP CONSTRAINT questions_type_check;
ALTER TABLE questions ADD CONSTRAINT questions_type_check CHECK (type IN ('multiple_choice'));
