ALTER TABLE questions DROP CONSTRAINT questions_type_check;
ALTER TABLE questions ADD CONSTRAINT questions_type_check CHECK (type IN ('multiple_choice', 'free_text'));

-- Free-text answers have no option to select and no automatic correctness
-- verdict — is_correct stays NULL until the admin grades it by hand.
ALTER TABLE answers ALTER COLUMN selected_option_id DROP NOT NULL;
ALTER TABLE answers ALTER COLUMN is_correct DROP NOT NULL;
ALTER TABLE answers ADD COLUMN answer_text text;

ALTER TABLE answers ADD CONSTRAINT answers_option_xor_text CHECK (
    (selected_option_id IS NOT NULL AND answer_text IS NULL) OR
    (selected_option_id IS NULL AND answer_text IS NOT NULL)
);
