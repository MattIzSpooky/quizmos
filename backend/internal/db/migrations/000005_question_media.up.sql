-- media_key is the object's key within the media bucket (see
-- internal/storage); NULL means the question has no media attached.
-- media_type is only meaningful alongside a non-NULL media_key.
ALTER TABLE questions ADD COLUMN media_key text;
ALTER TABLE questions ADD COLUMN media_type text CHECK (media_type IN ('image', 'audio'));
ALTER TABLE questions ADD CONSTRAINT questions_media_key_type CHECK (
    (media_key IS NULL AND media_type IS NULL) OR
    (media_key IS NOT NULL AND media_type IS NOT NULL)
);
