CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE quizzes (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_by  text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE questions (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    quiz_id             uuid NOT NULL REFERENCES quizzes(id) ON DELETE CASCADE,
    type                text NOT NULL CHECK (type IN ('multiple_choice')),
    prompt              text NOT NULL,
    position            integer NOT NULL,
    time_limit_seconds  integer NOT NULL DEFAULT 30,
    points              integer NOT NULL DEFAULT 1000,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (quiz_id, position)
);

CREATE TABLE question_options (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id uuid NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    text        text NOT NULL,
    is_correct  boolean NOT NULL DEFAULT false,
    position    integer NOT NULL,
    UNIQUE (question_id, position)
);

CREATE TABLE games (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    quiz_id                 uuid NOT NULL REFERENCES quizzes(id) ON DELETE RESTRICT,
    code                    text NOT NULL UNIQUE,
    status                  text NOT NULL DEFAULT 'lobby' CHECK (status IN ('lobby', 'in_progress', 'ended')),
    current_question_index  integer,
    created_by              text NOT NULL,
    created_at              timestamptz NOT NULL DEFAULT now(),
    started_at              timestamptz,
    ended_at                timestamptz
);

CREATE INDEX games_status_idx ON games (status);

CREATE TABLE players (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id     uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    client_id   uuid NOT NULL,
    nickname    text NOT NULL,
    score       integer NOT NULL DEFAULT 0,
    joined_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (game_id, client_id)
);

CREATE TABLE answers (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id             uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    question_id         uuid NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    player_id           uuid NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    selected_option_id  uuid NOT NULL REFERENCES question_options(id) ON DELETE CASCADE,
    is_correct          boolean NOT NULL,
    points_awarded      integer NOT NULL DEFAULT 0,
    answered_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (question_id, player_id)
);
