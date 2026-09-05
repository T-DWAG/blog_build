CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS admin_users (
  id              BIGSERIAL PRIMARY KEY,
  username        VARCHAR(32)  NOT NULL UNIQUE,
  password_hash   VARCHAR(100) NOT NULL,
  failed_attempts INT          NOT NULL DEFAULT 0,
  locked_until    TIMESTAMPTZ,
  last_login_at   TIMESTAMPTZ,
  created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS articles (
  id           BIGSERIAL PRIMARY KEY,
  slug         VARCHAR(200) NOT NULL UNIQUE,
  title        VARCHAR(200) NOT NULL,
  summary      VARCHAR(300) NOT NULL DEFAULT '',
  content_md   TEXT         NOT NULL,
  status       VARCHAR(20)  NOT NULL DEFAULT 'draft'
               CHECK (status IN ('draft','published')),
  is_pinned    BOOLEAN      NOT NULL DEFAULT FALSE,
  tags         TEXT[]       NOT NULL DEFAULT '{}',
  cover_url    VARCHAR(500) NOT NULL DEFAULT '',
  published_at TIMESTAMPTZ,
  created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_articles_list ON articles (status, is_pinned DESC, published_at DESC);
CREATE INDEX IF NOT EXISTS idx_articles_tags ON articles USING GIN (tags);

CREATE TABLE IF NOT EXISTS projects (
  id         BIGSERIAL PRIMARY KEY,
  name       VARCHAR(100) NOT NULL,
  cover_url  VARCHAR(500) NOT NULL DEFAULT '',
  summary    VARCHAR(300) NOT NULL DEFAULT '',
  repo_url   VARCHAR(500) NOT NULL DEFAULT '',
  home_url   VARCHAR(500) NOT NULL DEFAULT '',
  demo_url   VARCHAR(500) NOT NULL DEFAULT '',
  detail_md  TEXT         NOT NULL DEFAULT '',
  tags       TEXT[]       NOT NULL DEFAULT '{}',
  sort_order INT          NOT NULL DEFAULT 0,
  published  BOOLEAN      NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_projects_list ON projects (published, sort_order DESC);

CREATE TABLE IF NOT EXISTS messages (
  id              BIGSERIAL PRIMARY KEY,
  nickname        VARCHAR(24)  NOT NULL,
  content         VARCHAR(500) NOT NULL,
  status          VARCHAR(20)  NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','approved','rejected')),
  ip              INET,
  user_agent      TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  reviewed_at     TIMESTAMPTZ,
  reviewed_action VARCHAR(10)
);
CREATE INDEX IF NOT EXISTS idx_messages_list ON messages (status, created_at DESC);

CREATE TABLE IF NOT EXISTS knowledge_docs (
  id          BIGSERIAL PRIMARY KEY,
  title       VARCHAR(200) NOT NULL,
  source_type VARCHAR(20)  NOT NULL
              CHECK (source_type IN ('github','zip','markdown','text')),
  source_ref  VARCHAR(500) NOT NULL DEFAULT '',
  status      VARCHAR(20)  NOT NULL DEFAULT 'indexing'
              CHECK (status IN ('indexing','ready','failed')),
  error_msg   TEXT,
  chunk_count INT          NOT NULL DEFAULT 0,
  meta        JSONB        NOT NULL DEFAULT '{}',
  created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS knowledge_chunks (
  id          BIGSERIAL PRIMARY KEY,
  doc_id      BIGINT       NOT NULL REFERENCES knowledge_docs(id) ON DELETE CASCADE,
  chunk_index INT          NOT NULL DEFAULT 0,
  content     TEXT         NOT NULL,
  source_path VARCHAR(500) NOT NULL DEFAULT '',
  token_count INT          NOT NULL DEFAULT 0,
  embedding   VECTOR(1024) NOT NULL,
  created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_chunks_doc ON knowledge_chunks (doc_id);
CREATE INDEX IF NOT EXISTS idx_chunks_vec ON knowledge_chunks USING HNSW (embedding vector_cosine_ops);

CREATE TABLE IF NOT EXISTS settings (
  key         VARCHAR(64) PRIMARY KEY,
  value       JSONB       NOT NULL,
  description VARCHAR(200) NOT NULL DEFAULT '',
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
