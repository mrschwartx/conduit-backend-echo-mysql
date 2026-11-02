CREATE TABLE IF NOT EXISTS follows (
  follower_id CHAR(36) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
  followed_id CHAR(36) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
  created_at  DATETIME(6) NOT NULL DEFAULT (UTC_TIMESTAMP(6)),
  PRIMARY KEY (follower_id, followed_id),
  KEY follows_followed (followed_id),
  CONSTRAINT follows_follower_fk FOREIGN KEY (follower_id) REFERENCES users (id) ON DELETE CASCADE,
  CONSTRAINT follows_followed_fk FOREIGN KEY (followed_id) REFERENCES users (id) ON DELETE CASCADE,
  CONSTRAINT follows_not_self CHECK (follower_id <> followed_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
