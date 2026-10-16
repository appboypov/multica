CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_comment_release_keyset ON comment (release_id, created_at DESC, id DESC) WHERE release_id IS NOT NULL;
