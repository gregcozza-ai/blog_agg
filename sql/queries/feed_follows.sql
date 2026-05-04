-- name: CreateFeedFollow :one
INSERT INTO feed_follows (id, created_at, updated_at, feed_id, user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, created_at, updated_at, feed_id, user_id, (SELECT name FROM feeds WHERE id = $4) AS feed_name, (SELECT name FROM users WHERE id = $5) AS user_name;

-- name: GetFeedFollowsByUser :many
SELECT f.name, f.url
FROM feeds f
JOIN feed_follows ff ON ff.feed_id = f.id
WHERE ff.user_id = $1;

-- name: DeleteFeedFollows :exec
DELETE FROM feed_follows;

-- name: DeleteFeedFollowsUser :exec
DELETE FROM feed_follows WHERE feed_id = $1 AND user_id = $2;

