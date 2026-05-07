CREATE TABLE IF NOT EXISTS roles (
    id SERIAL PRIMARY KEY,
    role VARCHAR(100) NOT NULL UNIQUE,
    priority INT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    firstname VARCHAR(100) NOT NULL,
    lastname VARCHAR(100) NOT NULL,
    email VARCHAR(100) NOT NULL UNIQUE,
    password TEXT NOT NULL,
    role VARCHAR(100) NOT NULL DEFAULT 'User'
        REFERENCES roles(role) ON UPDATE CASCADE ON DELETE RESTRICT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE IF NOT EXISTS addresses (
    id SERIAL PRIMARY KEY,
    name VARCHAR(150),
    address_line_1 VARCHAR(255),
    address_line_2 VARCHAR(255),
    city VARCHAR(100),
    province_state VARCHAR(100),
    postal_code VARCHAR(20),
    country VARCHAR(100),
    is_saved BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS galleries (
    id SERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS events (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    show_title BOOLEAN NOT NULL DEFAULT TRUE,
    categories TEXT[] NOT NULL DEFAULT '{}',
    event_type VARCHAR(30) NOT NULL,
    start_at TIMESTAMP NOT NULL,
    end_at TIMESTAMP NULL,
    privacy_type VARCHAR(20) NOT NULL DEFAULT 'public',
    private_audiences TEXT[] NOT NULL DEFAULT '{}',
    published BOOLEAN NOT NULL DEFAULT FALSE,
    request_review BOOLEAN NOT NULL DEFAULT FALSE,
    review_email_list TEXT[] NOT NULL DEFAULT '{}',
    teaser TEXT NOT NULL,
    description_html TEXT,
    contact_name VARCHAR(150),
    contact_email VARCHAR(255),
    contact_phone VARCHAR(30),
    contact_ext VARCHAR(20),
    contact_fax VARCHAR(30),
    location_mode VARCHAR(30) NOT NULL DEFAULT 'none',
    address_id INT NULL,
    show_display_image_when_viewing BOOLEAN NOT NULL DEFAULT TRUE,
    gallery_id INT NULL,
    registration_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    registration_start_at TIMESTAMP NULL,
    registration_end_at TIMESTAMP NULL,
    registration_url TEXT NULL,
    repeat_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    recurrence_type VARCHAR(20) NULL,
    recurrence_frequency VARCHAR(20) NULL,
    recurrence_interval INT NOT NULL DEFAULT 1,
    recurrence_until TIMESTAMP NULL,
    recurrence_rule JSONB NULL,
    created_by INT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_events_address
        FOREIGN KEY (address_id) REFERENCES addresses(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_events_gallery
        FOREIGN KEY (gallery_id) REFERENCES galleries(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_events_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT chk_events_title_not_blank
        CHECK (btrim(title) <> ''),

    CONSTRAINT chk_events_teaser_not_blank
        CHECK (btrim(teaser) <> ''),

    CONSTRAINT chk_events_categories_required
        CHECK (cardinality(categories) > 0),

    CONSTRAINT chk_events_event_type
        CHECK (event_type IN (
            'single_day_all_day',
            'single_day_partial',
            'multi_day_all_day',
            'multi_day_partial'
        )),

    CONSTRAINT chk_events_end_after_start
        CHECK (end_at IS NULL OR end_at >= start_at),

    CONSTRAINT chk_events_privacy_type
        CHECK (privacy_type IN ('public', 'private')),

    CONSTRAINT chk_events_private_audiences
        CHECK (
            (privacy_type = 'public' AND cardinality(private_audiences) = 0)
            OR
            (privacy_type = 'private' AND cardinality(private_audiences) > 0)
        ),

    CONSTRAINT chk_events_location_mode
        CHECK (location_mode IN ('none', 'to_be_determined', 'address')),

    CONSTRAINT chk_events_location_address
        CHECK (
            (location_mode = 'address' AND address_id IS NOT NULL)
            OR
            (location_mode IN ('none', 'to_be_determined') AND address_id IS NULL)
        ),

    CONSTRAINT chk_events_review_request_emails
        CHECK (
            (request_review = FALSE AND cardinality(review_email_list) = 0)
            OR
            (request_review = TRUE AND cardinality(review_email_list) > 0)
        ),

    CONSTRAINT chk_events_registration
        CHECK (
            registration_enabled = FALSE
            OR (
                registration_start_at IS NOT NULL
                AND registration_end_at IS NOT NULL
                AND registration_end_at >= registration_start_at
                AND registration_url IS NOT NULL
                AND btrim(registration_url) <> ''
            )
        ),

    CONSTRAINT chk_events_recurrence_type
        CHECK (
            recurrence_type IS NULL
            OR recurrence_type IN ('scheduled', 'recurring')
        ),

    CONSTRAINT chk_events_recurrence_frequency
        CHECK (
            recurrence_frequency IS NULL
            OR recurrence_frequency IN ('daily', 'weekly', 'monthly', 'yearly')
        ),

    CONSTRAINT chk_events_recurrence_interval
        CHECK (recurrence_interval > 0),

    CONSTRAINT chk_events_repeat_definition
        CHECK (
            repeat_enabled = FALSE
            OR recurrence_type IS NOT NULL
        ),

    CONSTRAINT chk_events_recurring_requires_frequency
        CHECK (
            recurrence_type IS DISTINCT FROM 'recurring'
            OR recurrence_frequency IS NOT NULL
        ),

    CONSTRAINT chk_events_scheduled_has_no_frequency
        CHECK (
            recurrence_type IS DISTINCT FROM 'scheduled'
            OR recurrence_frequency IS NULL
        ),

    CONSTRAINT chk_events_recurrence_rule_json
        CHECK (
            recurrence_rule IS NULL
            OR jsonb_typeof(recurrence_rule) IN ('object', 'array')
        )
);

CREATE TABLE IF NOT EXISTS event_media (
    id SERIAL PRIMARY KEY,
    event_id INT NOT NULL,
    media_role VARCHAR(30) NOT NULL,
    display_name VARCHAR(255),
    gcp_object_key TEXT,
    file_url TEXT NOT NULL,
    mime_type VARCHAR(255),
    file_size BIGINT,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_event_media_event
        FOREIGN KEY (event_id) REFERENCES events(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT chk_event_media_role
        CHECK (media_role IN ('display_image', 'attachment')),

    CONSTRAINT chk_event_media_url_not_blank
        CHECK (btrim(file_url) <> ''),

    CONSTRAINT chk_event_media_file_size
        CHECK (file_size IS NULL OR file_size >= 0),

    CONSTRAINT chk_event_media_sort_order
        CHECK (sort_order >= 0)
);

CREATE TABLE IF NOT EXISTS event_occurrences (
    id SERIAL PRIMARY KEY,
    event_id INT NOT NULL,
    occurrence_start_at TIMESTAMP NOT NULL,
    occurrence_end_at TIMESTAMP NULL,
    occurrence_kind VARCHAR(20) NOT NULL DEFAULT 'scheduled',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_event_occurrences_event
        FOREIGN KEY (event_id) REFERENCES events(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT chk_event_occurrence_end_after_start
        CHECK (occurrence_end_at IS NULL OR occurrence_end_at >= occurrence_start_at),

    CONSTRAINT chk_event_occurrence_kind
        CHECK (occurrence_kind IN ('scheduled', 'generated', 'exception')),

    CONSTRAINT uq_event_occurrence_start
        UNIQUE (event_id, occurrence_start_at)
);

CREATE INDEX IF NOT EXISTS idx_addresses_name ON addresses(name);
CREATE INDEX IF NOT EXISTS idx_addresses_city ON addresses(city);
CREATE INDEX IF NOT EXISTS idx_addresses_is_saved ON addresses(is_saved);

CREATE INDEX IF NOT EXISTS idx_galleries_name ON galleries(name);

CREATE INDEX IF NOT EXISTS idx_events_created_by ON events(created_by);
CREATE INDEX IF NOT EXISTS idx_events_address_id ON events(address_id);
CREATE INDEX IF NOT EXISTS idx_events_gallery_id ON events(gallery_id);
CREATE INDEX IF NOT EXISTS idx_events_start_at ON events(start_at);
CREATE INDEX IF NOT EXISTS idx_events_published ON events(published);
CREATE INDEX IF NOT EXISTS idx_events_event_type ON events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_privacy_type ON events(privacy_type);
CREATE INDEX IF NOT EXISTS idx_events_categories_gin ON events USING GIN (categories);
CREATE INDEX IF NOT EXISTS idx_events_private_audiences_gin ON events USING GIN (private_audiences);
CREATE INDEX IF NOT EXISTS idx_events_review_email_list_gin ON events USING GIN (review_email_list);

CREATE INDEX IF NOT EXISTS idx_event_media_event_id ON event_media(event_id);
CREATE INDEX IF NOT EXISTS idx_event_media_role ON event_media(media_role);
CREATE INDEX IF NOT EXISTS idx_event_media_event_role_sort ON event_media(event_id, media_role, sort_order);
CREATE UNIQUE INDEX IF NOT EXISTS uq_event_media_one_display_image_per_event
    ON event_media(event_id)
    WHERE media_role = 'display_image';

CREATE INDEX IF NOT EXISTS idx_event_occurrences_event_id ON event_occurrences(event_id);
CREATE INDEX IF NOT EXISTS idx_event_occurrences_start_at ON event_occurrences(occurrence_start_at);
CREATE INDEX IF NOT EXISTS idx_event_occurrences_event_start ON event_occurrences(event_id, occurrence_start_at);

DROP TRIGGER IF EXISTS trg_addresses_set_updated_at ON addresses;
CREATE TRIGGER trg_addresses_set_updated_at
BEFORE UPDATE ON addresses
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_galleries_set_updated_at ON galleries;
CREATE TRIGGER trg_galleries_set_updated_at
BEFORE UPDATE ON galleries
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_events_set_updated_at ON events;
CREATE TRIGGER trg_events_set_updated_at
BEFORE UPDATE ON events
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_event_media_set_updated_at ON event_media;
CREATE TRIGGER trg_event_media_set_updated_at
BEFORE UPDATE ON event_media
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_event_occurrences_set_updated_at ON event_occurrences;
CREATE TRIGGER trg_event_occurrences_set_updated_at
BEFORE UPDATE ON event_occurrences
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

INSERT INTO roles (role, priority)
VALUES
    ('Admin', 1),
    ('User', 2)
ON CONFLICT (role) DO UPDATE
SET priority = EXCLUDED.priority,
    updated_at = CURRENT_TIMESTAMP;
