-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS pgcrypto; -- for gen_random_uuid
CREATE EXTENSION IF NOT EXISTS vector;

-- Questions table
CREATE TABLE IF NOT EXISTS questions (
  id UUID PRIMARY KEY,
  title TEXT NOT NULL,
  text TEXT NOT NULL,
  topic TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  sha256 TEXT UNIQUE NOT NULL,
  embedding VECTOR(1536) NOT NULL,
  choices JSONB
);

ALTER TABLE questions
  ADD COLUMN IF NOT EXISTS choices_normalized JSONB,
  ADD COLUMN IF NOT EXISTS choices_signature TEXT;

-- ivfflat index for fast similarity search
CREATE INDEX IF NOT EXISTS questions_embedding_ivfflat ON questions USING ivfflat (embedding vector_cosine_ops) WITH (lists=50);
CREATE INDEX IF NOT EXISTS questions_choices_normalized_idx ON questions USING GIN (COALESCE(choices_normalized, '[]'::jsonb));
CREATE UNIQUE INDEX IF NOT EXISTS questions_choices_signature_idx ON questions (choices_signature) WHERE choices_signature IS NOT NULL;

WITH expanded AS (
  SELECT
    q.id,
    jsonb_agg(DISTINCT norm.norm ORDER BY norm.norm) AS normalized
  FROM questions q
  LEFT JOIN LATERAL (
    SELECT NULLIF(
      regexp_replace(
        regexp_replace(
          regexp_replace(lower(trim(elem.value)), '^(the|an|a)\s+', ''),
          '[^a-z0-9]+', '', 'g'
        ),
        '\s+', '', 'g'
      ),
      ''
    ) AS norm
    FROM jsonb_array_elements_text(COALESCE(q.choices, '[]'::jsonb)) AS elem(value)
  ) AS norm ON norm.norm IS NOT NULL
  GROUP BY q.id
)
UPDATE questions q
SET choices_normalized = expanded.normalized
FROM expanded
WHERE q.id = expanded.id AND (q.choices_normalized IS NULL OR jsonb_typeof(q.choices_normalized) IS DISTINCT FROM 'array');

UPDATE questions
SET choices_signature = encode(digest(
  COALESCE((
    SELECT string_agg(value, '|' ORDER BY value)
    FROM jsonb_array_elements_text(choices_normalized)
  ), ''), 'sha256'), 'hex')
WHERE choices_normalized IS NOT NULL AND jsonb_array_length(choices_normalized) > 0 AND choices_signature IS NULL;

-- Answers table
CREATE TABLE IF NOT EXISTS answers (
  id UUID PRIMARY KEY,
  question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
  text TEXT NOT NULL,
  score INT,
  rubric_json JSONB,
  feedback TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
