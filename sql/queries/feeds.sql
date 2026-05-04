-- name: CreateFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, created_at, updated_at, name, url;

-- name: GetFeedByURL :one
SELECT id, created_at, updated_at, name, url
FROM feeds
WHERE url = $1;

-- name: GetFeeds :many
SELECT f.id, f.created_at, f.updated_at, f.name, f.url, u.name AS user_name
FROM feeds f
JOIN users u ON f.user_id = u.id;

-- name: DeleteFeeds :exec
DELETE FROM feeds;

-- name: MarkFeedFetched :exec
UPDATE feeds
SET last_fetched_at = NOW(), updated_at = NOW()
WHERE id = $1;

-- name: GetNextFeedToFetch :one
SELECT id, name, url
FROM feeds
WHERE last_fetched_at IS NULL OR last_fetched_at < NOW() - INTERVAL '1 day'
ORDER BY last_fetched_at NULLS FIRST
LIMIT 1;

