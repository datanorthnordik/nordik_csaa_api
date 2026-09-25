-- Standalone pgAdmin migration for the homepage latest-content feed.
--
-- Run this entire file against the CSAA PostgreSQL database. No substitutions
-- or manual edits are required. It is safe to run more than once.
--
-- Prerequisites (all in the same non-system schema):
--   events
--   press_entries
--   newsletter_entries

-- Recover cleanly if a previous execution failed after BEGIN and left this
-- pgAdmin Query Tool session in PostgreSQL's aborted-transaction state.
ROLLBACK;

BEGIN;

-- Use the schema that owns the application's source tables. This avoids
-- assuming they live in public and matches the API's unqualified table names.
DO $$
DECLARE
    target_schema TEXT;
BEGIN
    SELECT n.nspname
    INTO target_schema
    FROM pg_catalog.pg_namespace AS n
    WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
      AND n.nspname NOT LIKE 'pg_toast%'
      AND EXISTS (
          SELECT 1 FROM pg_catalog.pg_class AS c
          WHERE c.relnamespace = n.oid
            AND c.relname = 'events'
            AND c.relkind IN ('r', 'p')
      )
      AND EXISTS (
          SELECT 1 FROM pg_catalog.pg_class AS c
          WHERE c.relnamespace = n.oid
            AND c.relname = 'press_entries'
            AND c.relkind IN ('r', 'p')
      )
      AND EXISTS (
          SELECT 1 FROM pg_catalog.pg_class AS c
          WHERE c.relnamespace = n.oid
            AND c.relname = 'newsletter_entries'
            AND c.relkind IN ('r', 'p')
      )
    ORDER BY
        (n.nspname = current_schema()) DESC,
        (n.nspname = 'public') DESC,
        n.nspname
    LIMIT 1;

    IF target_schema IS NULL THEN
        RAISE EXCEPTION
            'Required tables events, press_entries, and newsletter_entries were not found together in database %',
            current_database()
            USING HINT = 'Select the CSAA application database in pgAdmin, ensure its base migrations are installed, and run this file again.';
    END IF;

    PERFORM pg_catalog.set_config(
        'search_path',
        pg_catalog.quote_ident(target_schema) || ', pg_catalog',
        TRUE
    );
    RAISE NOTICE 'Installing latest-content objects in schema "%"', target_schema;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE IF NOT EXISTS latest_content (
    id SERIAL PRIMARY KEY,
    source_type VARCHAR(20) NOT NULL,
    source_id INT NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    display_date TIMESTAMP NOT NULL,
    published_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uq_latest_content_source UNIQUE (source_type, source_id),

    CONSTRAINT chk_latest_content_source_type
        CHECK (source_type IN ('event', 'press', 'newsletter')),

    CONSTRAINT chk_latest_content_source_id
        CHECK (source_id > 0),

    CONSTRAINT chk_latest_content_title_not_blank
        CHECK (BTRIM(title) <> '')
);

CREATE INDEX IF NOT EXISTS idx_latest_content_publication_order
    ON latest_content(published_at DESC, id DESC);

CREATE OR REPLACE FUNCTION latest_content_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql
SET search_path FROM CURRENT;

DROP TRIGGER IF EXISTS trg_latest_content_set_updated_at
    ON latest_content;
CREATE TRIGGER trg_latest_content_set_updated_at
BEFORE UPDATE ON latest_content
FOR EACH ROW
EXECUTE FUNCTION latest_content_set_updated_at();

CREATE OR REPLACE FUNCTION latest_content_plain_text(value TEXT)
RETURNS TEXT AS $$
    SELECT LEFT(
        BTRIM(
            REGEXP_REPLACE(
                REGEXP_REPLACE(COALESCE(value, ''), '<[^>]+>', ' ', 'g'),
                '[[:space:]]+',
                ' ',
                'g'
            )
        ),
        1000
    );
$$ LANGUAGE SQL IMMUTABLE
SET search_path FROM CURRENT;

CREATE OR REPLACE FUNCTION prune_latest_content()
RETURNS VOID AS $$
BEGIN
    -- Serialize pruning so concurrent publications cannot leave more than 10 rows.
    PERFORM pg_advisory_xact_lock(hashtext('latest_content_top_10'));

    DELETE FROM latest_content
    WHERE id IN (
        SELECT id
        FROM latest_content
        ORDER BY published_at DESC, id DESC
        OFFSET 10
    );
END;
$$ LANGUAGE plpgsql
SET search_path FROM CURRENT;

CREATE OR REPLACE FUNCTION sync_event_latest_content()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        DELETE FROM latest_content
        WHERE source_type = 'event' AND source_id = OLD.id;
        RETURN OLD;
    END IF;

    IF NOT (NEW.published AND NEW.privacy_type = 'public') THEN
        DELETE FROM latest_content
        WHERE source_type = 'event' AND source_id = NEW.id;
        RETURN NEW;
    END IF;

    IF TG_OP = 'UPDATE' AND OLD.published AND OLD.privacy_type = 'public' THEN
        UPDATE latest_content
        SET title = NEW.title,
            description = latest_content_plain_text(NEW.teaser),
            display_date = NEW.start_at
        WHERE source_type = 'event' AND source_id = NEW.id;
        RETURN NEW;
    END IF;

    INSERT INTO latest_content (
        source_type,
        source_id,
        title,
        description,
        display_date,
        published_at
    ) VALUES (
        'event',
        NEW.id,
        NEW.title,
        latest_content_plain_text(NEW.teaser),
        NEW.start_at,
        CURRENT_TIMESTAMP
    )
    ON CONFLICT (source_type, source_id) DO UPDATE
    SET title = EXCLUDED.title,
        description = EXCLUDED.description,
        display_date = EXCLUDED.display_date;

    PERFORM prune_latest_content();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql
SET search_path FROM CURRENT;

CREATE OR REPLACE FUNCTION sync_press_latest_content()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        DELETE FROM latest_content
        WHERE source_type = 'press' AND source_id = OLD.id;
        RETURN OLD;
    END IF;

    IF NOT (NEW.status = 'published' AND NEW.visibility = 'public') THEN
        DELETE FROM latest_content
        WHERE source_type = 'press' AND source_id = NEW.id;
        RETURN NEW;
    END IF;

    IF TG_OP = 'UPDATE'
       AND OLD.status = 'published'
       AND OLD.visibility = 'public' THEN
        UPDATE latest_content
        SET title = NEW.title,
            description = latest_content_plain_text(NEW.content_html),
            display_date = NEW.release_date
        WHERE source_type = 'press' AND source_id = NEW.id;
        RETURN NEW;
    END IF;

    INSERT INTO latest_content (
        source_type,
        source_id,
        title,
        description,
        display_date,
        published_at
    ) VALUES (
        'press',
        NEW.id,
        NEW.title,
        latest_content_plain_text(NEW.content_html),
        NEW.release_date,
        CURRENT_TIMESTAMP
    )
    ON CONFLICT (source_type, source_id) DO UPDATE
    SET title = EXCLUDED.title,
        description = EXCLUDED.description,
        display_date = EXCLUDED.display_date;

    PERFORM prune_latest_content();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql
SET search_path FROM CURRENT;

CREATE OR REPLACE FUNCTION sync_newsletter_latest_content()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        DELETE FROM latest_content
        WHERE source_type = 'newsletter' AND source_id = OLD.id;
        RETURN OLD;
    END IF;

    IF NOT (NEW.status = 'published' AND NEW.visibility = 'public') THEN
        DELETE FROM latest_content
        WHERE source_type = 'newsletter' AND source_id = NEW.id;
        RETURN NEW;
    END IF;

    IF TG_OP = 'UPDATE'
       AND OLD.status = 'published'
       AND OLD.visibility = 'public' THEN
        UPDATE latest_content
        SET title = NEW.title,
            description = latest_content_plain_text(NEW.content_html),
            display_date = NEW.send_date
        WHERE source_type = 'newsletter' AND source_id = NEW.id;
        RETURN NEW;
    END IF;

    INSERT INTO latest_content (
        source_type,
        source_id,
        title,
        description,
        display_date,
        published_at
    ) VALUES (
        'newsletter',
        NEW.id,
        NEW.title,
        latest_content_plain_text(NEW.content_html),
        NEW.send_date,
        CURRENT_TIMESTAMP
    )
    ON CONFLICT (source_type, source_id) DO UPDATE
    SET title = EXCLUDED.title,
        description = EXCLUDED.description,
        display_date = EXCLUDED.display_date;

    PERFORM prune_latest_content();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql
SET search_path FROM CURRENT;

DROP TRIGGER IF EXISTS trg_events_sync_latest_content ON events;
CREATE TRIGGER trg_events_sync_latest_content
AFTER INSERT OR UPDATE OR DELETE ON events
FOR EACH ROW
EXECUTE FUNCTION sync_event_latest_content();

DROP TRIGGER IF EXISTS trg_press_entries_sync_latest_content
    ON press_entries;
CREATE TRIGGER trg_press_entries_sync_latest_content
AFTER INSERT OR UPDATE OR DELETE ON press_entries
FOR EACH ROW
EXECUTE FUNCTION sync_press_latest_content();

DROP TRIGGER IF EXISTS trg_newsletter_entries_sync_latest_content
    ON newsletter_entries;
CREATE TRIGGER trg_newsletter_entries_sync_latest_content
AFTER INSERT OR UPDATE OR DELETE ON newsletter_entries
FOR EACH ROW
EXECUTE FUNCTION sync_newsletter_latest_content();

-- Backfill the newest currently published public records across all sources.
WITH candidates AS (
    SELECT
        'event'::VARCHAR(20) AS source_type,
        id AS source_id,
        title,
        latest_content_plain_text(teaser) AS description,
        start_at AS display_date,
        COALESCE(updated_at, created_at) AS published_at
    FROM events
    WHERE published = TRUE AND privacy_type = 'public'

    UNION ALL

    SELECT
        'press'::VARCHAR(20),
        id,
        title,
        latest_content_plain_text(content_html),
        release_date::TIMESTAMP,
        COALESCE(publish_at, updated_at, created_at)
    FROM press_entries
    WHERE status = 'published' AND visibility = 'public'

    UNION ALL

    SELECT
        'newsletter'::VARCHAR(20),
        id,
        title,
        latest_content_plain_text(content_html),
        send_date::TIMESTAMP,
        COALESCE(publish_at, updated_at, created_at)
    FROM newsletter_entries
    WHERE status = 'published' AND visibility = 'public'
), newest AS (
    SELECT *
    FROM candidates
    ORDER BY published_at DESC, source_type ASC, source_id DESC
    LIMIT 10
)
INSERT INTO latest_content (
    source_type,
    source_id,
    title,
    description,
    display_date,
    published_at
)
SELECT
    source_type,
    source_id,
    title,
    description,
    display_date,
    published_at
FROM newest
ON CONFLICT (source_type, source_id) DO UPDATE
SET title = EXCLUDED.title,
    description = EXCLUDED.description,
    display_date = EXCLUDED.display_date;

SELECT prune_latest_content();

-- Verification result shown in pgAdmin after successful execution. Keep this
-- inside the transaction because the auto-detected search path is transaction-local.
SELECT
    id,
    source_type,
    source_id,
    title,
    description,
    display_date,
    published_at
FROM latest_content
ORDER BY published_at DESC, id DESC;

COMMIT;
