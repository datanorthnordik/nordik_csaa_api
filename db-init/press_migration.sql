-- Press Entries Migration
-- Prerequisites: tables users(id) must already exist.

BEGIN;

CREATE TABLE IF NOT EXISTS press_entries (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    release_date DATE NOT NULL,
    category_id INT,
    source_url TEXT,
    content_html TEXT NOT NULL DEFAULT '',
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    visibility VARCHAR(30) NOT NULL DEFAULT 'private',
    cover_image_url TEXT,
    cover_image_gcp_key TEXT,
    publish_at TIMESTAMP,
    created_by INT,
    updated_by INT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_press_entries_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,
    
    CONSTRAINT fk_press_entries_updated_by
        FOREIGN KEY (updated_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,
    
    CONSTRAINT chk_press_entries_title_not_blank
        CHECK (btrim(title) <> ''),
    
    CONSTRAINT chk_press_entries_status
        CHECK (status IN ('draft', 'published', 'archived')),
    
    CONSTRAINT chk_press_entries_visibility
        CHECK (visibility IN ('public', 'private', 'scheduled'))
);

CREATE TABLE IF NOT EXISTS press_media (
    id SERIAL PRIMARY KEY,
    press_entry_id INT NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    file_name VARCHAR(255),
    gcp_object_key TEXT,
    file_url TEXT NOT NULL,
    mime_type VARCHAR(255),
    file_size BIGINT,
    media_role VARCHAR(50) NOT NULL DEFAULT 'attachment',
    sort_order INT NOT NULL DEFAULT -1,
    created_by INT,
    updated_by INT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_press_media_press_entry
        FOREIGN KEY (press_entry_id) REFERENCES press_entries(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    
    CONSTRAINT fk_press_media_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,
    
    CONSTRAINT fk_press_media_updated_by
        FOREIGN KEY (updated_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,
    
    CONSTRAINT chk_press_media_display_name_not_blank
        CHECK (btrim(display_name) <> ''),
    
    CONSTRAINT chk_press_media_file_url_not_blank
        CHECK (btrim(file_url) <> ''),
    
    CONSTRAINT chk_press_media_media_role
        CHECK (media_role IN ('cover_image', 'attachment')),
    
    CONSTRAINT chk_press_media_file_size
        CHECK (file_size IS NULL OR file_size >= 0),
    
    CONSTRAINT chk_press_media_sort_order
        CHECK (sort_order >= 0)
);

CREATE INDEX IF NOT EXISTS idx_press_entries_release_date
    ON press_entries(release_date DESC);

CREATE INDEX IF NOT EXISTS idx_press_entries_status
    ON press_entries(status);

CREATE INDEX IF NOT EXISTS idx_press_entries_visibility
    ON press_entries(visibility);

CREATE INDEX IF NOT EXISTS idx_press_entries_created_by
    ON press_entries(created_by);

CREATE INDEX IF NOT EXISTS idx_press_entries_updated_by
    ON press_entries(updated_by);

CREATE INDEX IF NOT EXISTS idx_press_media_press_entry_id
    ON press_media(press_entry_id);

CREATE INDEX IF NOT EXISTS idx_press_media_media_role
    ON press_media(media_role);

CREATE INDEX IF NOT EXISTS idx_press_media_sort_order
    ON press_media(press_entry_id, sort_order, id);

-- Trigger for auto-updating updated_at
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_press_entries_set_updated_at ON press_entries;
CREATE TRIGGER trg_press_entries_set_updated_at
BEFORE UPDATE ON press_entries
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_press_media_set_updated_at ON press_media;
CREATE TRIGGER trg_press_media_set_updated_at
BEFORE UPDATE ON press_media
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- Function to assign sort order for press media
CREATE OR REPLACE FUNCTION assign_press_media_sort_order()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.sort_order >= 0 THEN
        RETURN NEW;
    END IF;

    PERFORM 1
    FROM press_entries
    WHERE id = NEW.press_entry_id
    FOR UPDATE;

    SELECT COALESCE(MAX(sort_order), -1) + 1
    INTO NEW.sort_order
    FROM press_media
    WHERE press_entry_id = NEW.press_entry_id
      AND id <> COALESCE(NEW.id, 0);

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_press_media_assign_sort_order ON press_media;
CREATE TRIGGER trg_press_media_assign_sort_order
BEFORE INSERT OR UPDATE ON press_media
FOR EACH ROW
EXECUTE FUNCTION assign_press_media_sort_order();

COMMIT;
