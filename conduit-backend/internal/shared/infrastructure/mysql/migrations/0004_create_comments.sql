CREATE TABLE IF NOT EXISTS comments (
  id         BIGINT NOT NULL AUTO_INCREMENT,
  article_id CHAR(36) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
  author_id  CHAR(36) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
  body       VARCHAR(1000) NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT (UTC_TIMESTAMP(6)),
  updated_at DATETIME(6) NOT NULL DEFAULT (UTC_TIMESTAMP(6)),
  PRIMARY KEY (id),
  KEY comments_article (article_id, created_at DESC, id DESC),
  KEY comments_author (author_id),
  CONSTRAINT comments_article_fk FOREIGN KEY (article_id) REFERENCES articles (id) ON DELETE CASCADE,
  CONSTRAINT comments_author_fk FOREIGN KEY (author_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
