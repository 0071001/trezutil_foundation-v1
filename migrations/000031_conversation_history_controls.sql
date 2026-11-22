-- A person's control over their own conversations, kept on the server so it follows them on every device:
--   * title: the name they gave the conversation (NULL: the list shows the first message they wrote, as before).
--   * archived_at: when they archived it (NULL: not archived). An archived conversation is still listed, marked.
-- Deleting a conversation is a real DELETE: its messages, context, result items and confirmations go with it
-- (ON DELETE CASCADE, migrations 000008 and 000014). Orders and reservations it created are not tied to it and stay.

-- +goose Up
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS title TEXT;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS archived_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE conversations DROP CONSTRAINT IF EXISTS conversations_title_length;
ALTER TABLE conversations ADD CONSTRAINT conversations_title_length CHECK (title IS NULL OR char_length(title) <= 120);

-- +goose Down
ALTER TABLE conversations DROP CONSTRAINT IF EXISTS conversations_title_length;
ALTER TABLE conversations DROP COLUMN IF EXISTS archived_at;
ALTER TABLE conversations DROP COLUMN IF EXISTS title;
