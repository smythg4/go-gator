-- name: CreateFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url, user_id)
VALUES (
	$1,
	$2,
	$3,
	$4,
    $5,
    $6
)
RETURNING *;

-- name: GetFeeds :many
SELECT * FROM feeds;

-- name: GetFeedFollowsForUser :many
SELECT feed_follows.*, 
        feeds.name AS feed_name, 
        users.name AS user_name
FROM feed_follows
    INNER JOIN users ON feed_follows.user_id = users.id 
    INNER JOIN feeds ON feed_follows.feeds_id = feeds.id
WHERE feed_follows.user_id = $1;

-- name: CreateFeedFollow :one
WITH inserted_feed_follow AS (INSERT INTO feed_follows (
    id, 
    created_at,
    updated_at, 
    user_id, 
    feeds_id)
VALUES ($1,$2,$3,$4,$5)
RETURNING *)
SELECT inserted_feed_follow.*, 
    feeds.name AS feed_name, 
    users.name AS user_name
FROM inserted_feed_follow 
INNER JOIN users ON inserted_feed_follow.user_id = users.id
INNER JOIN feeds ON inserted_feed_follow.feeds_id = feeds.id;

-- name: DeleteFeedFollow :exec
DELETE FROM feed_follows WHERE user_id = $1 AND feeds_id = $2;

-- name: GetFeedByURL :one
SELECT * FROM feeds WHERE url = $1;

-- name: GetFeedByID :one
SELECT * FROM feeds WHERE id = $1;

-- name: MarkFeedFetched :exec
UPDATE feeds SET last_fetched_at = NOW(), updated_at = NOW() WHERE id = $1;

-- name: GetNextFeedToFetch :one
SELECT * FROM feeds ORDER BY last_fetched_at NULLS FIRST, id LIMIT 1;