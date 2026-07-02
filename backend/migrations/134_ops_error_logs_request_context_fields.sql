-- Backfill ops_error_logs request context columns for databases that applied
-- older versions of the vNext ops migration before these fields existed.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE ops_error_logs
    ADD COLUMN IF NOT EXISTS request_body JSONB,
    ADD COLUMN IF NOT EXISTS request_headers JSONB,
    ADD COLUMN IF NOT EXISTS request_body_truncated BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS request_body_bytes INT,
    ADD COLUMN IF NOT EXISTS is_retryable BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS retry_count INT NOT NULL DEFAULT 0;

COMMENT ON COLUMN ops_error_logs.request_body IS 'Sanitized request body stored for error diagnosis and safe retries.';
COMMENT ON COLUMN ops_error_logs.request_headers IS 'Sanitized request headers stored for error diagnosis and safe retries.';
COMMENT ON COLUMN ops_error_logs.request_body_truncated IS 'Whether the stored sanitized request body was truncated.';
COMMENT ON COLUMN ops_error_logs.request_body_bytes IS 'Original request body size in bytes before sanitization/truncation.';
COMMENT ON COLUMN ops_error_logs.is_retryable IS 'Best-effort retryability classification for the error request.';
COMMENT ON COLUMN ops_error_logs.retry_count IS 'Number of retry attempts issued from this error log.';
