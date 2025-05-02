-- +goose Up
CREATE TABLE feed_follows (
	id UUID PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	user_id UUID NOT NULL,
    feeds_id UUID NOT NULL,
    UNIQUE(user_id, feeds_id),
    CONSTRAINT fk_user
        FOREIGN KEY(user_id)
        REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_feeds
        FOREIGN KEY(feeds_id)
        REFERENCES feeds(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE feed_follows;
