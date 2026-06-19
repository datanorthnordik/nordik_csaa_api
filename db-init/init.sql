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

ALTER TABLE galleries
    ADD COLUMN IF NOT EXISTS description TEXT,
    ADD COLUMN IF NOT EXISTS cover_image_url TEXT,
    ADD COLUMN IF NOT EXISTS cover_image_object_key TEXT,
    ADD COLUMN IF NOT EXISTS cover_image_alt_text VARCHAR(255),
    ADD COLUMN IF NOT EXISTS published BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS created_by INT NULL,
    ADD COLUMN IF NOT EXISTS updated_by INT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_galleries_created_by'
    ) THEN
        ALTER TABLE galleries
            ADD CONSTRAINT fk_galleries_created_by
            FOREIGN KEY (created_by) REFERENCES users(id)
            ON UPDATE CASCADE
            ON DELETE SET NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_galleries_updated_by'
    ) THEN
        ALTER TABLE galleries
            ADD CONSTRAINT fk_galleries_updated_by
            FOREIGN KEY (updated_by) REFERENCES users(id)
            ON UPDATE CASCADE
            ON DELETE SET NULL;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS gallery_images (
    id SERIAL PRIMARY KEY,
    gallery_id INT NOT NULL,
    title VARCHAR(255),
    alt_text VARCHAR(255),
    link_url TEXT,
    gcp_object_key TEXT,
    file_url TEXT NOT NULL,
    mime_type VARCHAR(255),
    file_size BIGINT,
    sort_order INT NOT NULL DEFAULT 0,
    uploaded_by INT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_gallery_images_gallery
        FOREIGN KEY (gallery_id) REFERENCES galleries(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_gallery_images_uploaded_by
        FOREIGN KEY (uploaded_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT chk_gallery_images_url_not_blank
        CHECK (btrim(file_url) <> ''),

    CONSTRAINT chk_gallery_images_file_size
        CHECK (file_size IS NULL OR file_size >= 0),

    CONSTRAINT chk_gallery_images_sort_order
        CHECK (sort_order >= 0)
);

ALTER TABLE gallery_images
    ADD COLUMN IF NOT EXISTS link_url TEXT,
    ADD COLUMN IF NOT EXISTS sort_order INT NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_gallery_images_sort_order'
    ) THEN
        ALTER TABLE gallery_images
            ADD CONSTRAINT chk_gallery_images_sort_order
            CHECK (sort_order >= 0);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS video_packages (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    package_type VARCHAR(20) NOT NULL DEFAULT 'single',
    created_by INT NULL,
    updated_by INT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_video_packages_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_video_packages_updated_by
        FOREIGN KEY (updated_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT chk_video_packages_title_not_blank
        CHECK (btrim(title) <> ''),

    CONSTRAINT chk_video_packages_type
        CHECK (package_type IN ('single', 'collection'))
);

CREATE TABLE IF NOT EXISTS video_package_items (
    id SERIAL PRIMARY KEY,
    video_package_id INT NOT NULL,
    title VARCHAR(255) NOT NULL,
    youtube_url TEXT NOT NULL,
    description TEXT,
    teaser_image_url TEXT NOT NULL,
    teaser_image_object_key TEXT,
    sort_order INT NOT NULL DEFAULT 0,
    created_by INT NULL,
    updated_by INT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_video_package_items_package
        FOREIGN KEY (video_package_id) REFERENCES video_packages(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_video_package_items_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_video_package_items_updated_by
        FOREIGN KEY (updated_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT chk_video_package_items_title_not_blank
        CHECK (btrim(title) <> ''),

    CONSTRAINT chk_video_package_items_youtube_url_not_blank
        CHECK (btrim(youtube_url) <> ''),

    CONSTRAINT chk_video_package_items_teaser_image_url_not_blank
        CHECK (btrim(teaser_image_url) <> ''),

    CONSTRAINT chk_video_package_items_sort_order
        CHECK (sort_order >= 0)
);

CREATE TABLE IF NOT EXISTS pages (
    id SERIAL PRIMARY KEY,
    page_title VARCHAR(255) NOT NULL,
    url_slug VARCHAR(255) NOT NULL UNIQUE,
    parent_id INT NULL,
    page_type VARCHAR(20) NOT NULL DEFAULT 'page',
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    hero_image_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    hero_image_url TEXT,
    hero_image_object_key TEXT,
    seo_page_title VARCHAR(255),
    seo_page_description TEXT,
    created_by INT NULL,
    modified_by INT NULL,
    last_modified TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_pages_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_pages_modified_by
        FOREIGN KEY (modified_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT chk_pages_title_not_blank
        CHECK (btrim(page_title) <> ''),

    CONSTRAINT chk_pages_slug_not_blank
        CHECK (btrim(url_slug) <> ''),

    CONSTRAINT chk_pages_slug_format
        CHECK (
            url_slug = '/'
            OR url_slug ~ '^/[a-z0-9]+(?:/[a-z0-9]+|-[a-z0-9]+)*$'
        ),

    CONSTRAINT chk_pages_status
        CHECK (status IN ('draft', 'published')),

    CONSTRAINT chk_pages_type
        CHECK (page_type IN ('page', 'module'))
);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'pages'
          AND column_name = 'parent_page_id'
    ) AND NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'pages'
          AND column_name = 'parent_id'
    ) THEN
        ALTER TABLE pages RENAME COLUMN parent_page_id TO parent_id;
    END IF;
END $$;

ALTER TABLE pages
    ADD COLUMN IF NOT EXISTS parent_id INT NULL;

ALTER TABLE pages
    ADD COLUMN IF NOT EXISTS page_type VARCHAR(20) NOT NULL DEFAULT 'page';

UPDATE pages
SET page_type = 'page'
WHERE page_type IS NULL OR btrim(page_type) = '';

ALTER TABLE pages
    DROP CONSTRAINT IF EXISTS fk_pages_parent_page;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_pages_parent'
    ) THEN
        ALTER TABLE pages
            ADD CONSTRAINT fk_pages_parent
            FOREIGN KEY (parent_id) REFERENCES pages(id)
            ON UPDATE CASCADE
            ON DELETE SET NULL;
    END IF;
END $$;

ALTER TABLE pages
    DROP CONSTRAINT IF EXISTS chk_pages_type;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_pages_type'
    ) THEN
        ALTER TABLE pages
            ADD CONSTRAINT chk_pages_type
            CHECK (page_type IN ('page', 'module'));
    END IF;
END $$;

ALTER TABLE pages
    DROP CONSTRAINT IF EXISTS chk_pages_parent_not_self;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_pages_parent_not_self'
    ) THEN
        ALTER TABLE pages
            ADD CONSTRAINT chk_pages_parent_not_self
            CHECK (parent_id IS NULL OR parent_id <> id);
    END IF;
END $$;

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
    teaser TEXT DEFAULT '',
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

    CONSTRAINT chk_events_event_type_dates
        CHECK (
            (event_type = 'single_day_all_day' AND end_at IS NULL)
            OR
            (event_type = 'single_day_partial' AND end_at IS NOT NULL AND DATE(end_at) = DATE(start_at))
            OR
            (event_type = 'multi_day_all_day' AND end_at IS NOT NULL AND DATE(end_at) > DATE(start_at))
            OR
            (event_type = 'multi_day_partial' AND end_at IS NOT NULL AND DATE(end_at) > DATE(start_at))
        ),

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

    CONSTRAINT chk_events_published_review
        CHECK (
            published = FALSE
            OR request_review = FALSE
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

CREATE TABLE IF NOT EXISTS menus (
    id SERIAL PRIMARY KEY,
    menu_key VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    created_by INT NULL,
    updated_by INT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_menus_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_menus_updated_by
        FOREIGN KEY (updated_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT chk_menus_key_not_blank
        CHECK (btrim(menu_key) <> ''),

    CONSTRAINT chk_menus_name_not_blank
        CHECK (btrim(name) <> '')
);

CREATE TABLE IF NOT EXISTS menu_items (
    id SERIAL PRIMARY KEY,
    menu_id INT NOT NULL,
    parent_id INT NULL,
    label VARCHAR(255) NOT NULL,
    navigation_type VARCHAR(30) NOT NULL,
    page_id INT NULL,
    external_url TEXT,
    open_in_new_tab BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_menu_items_menu
        FOREIGN KEY (menu_id) REFERENCES menus(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_menu_items_parent
        FOREIGN KEY (parent_id) REFERENCES menu_items(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_menu_items_page
        FOREIGN KEY (page_id) REFERENCES pages(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT chk_menu_items_label_not_blank
        CHECK (btrim(label) <> ''),

    CONSTRAINT chk_menu_items_navigation_type
        CHECK (navigation_type IN ('pages', 'external_link')),

    CONSTRAINT chk_menu_items_parent_not_self
        CHECK (parent_id IS NULL OR parent_id <> id),

    CONSTRAINT chk_menu_items_sort_order
        CHECK (sort_order >= 0),

    CONSTRAINT chk_menu_items_target
        CHECK (
            (navigation_type = 'pages' AND page_id IS NOT NULL AND btrim(COALESCE(external_url, '')) = '')
            OR
            (navigation_type = 'external_link' AND page_id IS NULL AND btrim(COALESCE(external_url, '')) <> '')
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
CREATE INDEX IF NOT EXISTS idx_galleries_published ON galleries(published);
CREATE INDEX IF NOT EXISTS idx_galleries_created_by ON galleries(created_by);
CREATE INDEX IF NOT EXISTS idx_galleries_updated_by ON galleries(updated_by);

CREATE INDEX IF NOT EXISTS idx_gallery_images_gallery_id ON gallery_images(gallery_id);
CREATE INDEX IF NOT EXISTS idx_gallery_images_uploaded_by ON gallery_images(uploaded_by);
CREATE INDEX IF NOT EXISTS idx_gallery_images_file_url ON gallery_images(file_url);
CREATE INDEX IF NOT EXISTS idx_gallery_images_gallery_sort ON gallery_images(gallery_id, sort_order, id);

CREATE INDEX IF NOT EXISTS idx_video_packages_title ON video_packages(title);
CREATE INDEX IF NOT EXISTS idx_video_packages_package_type ON video_packages(package_type);
CREATE INDEX IF NOT EXISTS idx_video_packages_created_by ON video_packages(created_by);
CREATE INDEX IF NOT EXISTS idx_video_packages_updated_by ON video_packages(updated_by);

CREATE INDEX IF NOT EXISTS idx_video_package_items_package_id ON video_package_items(video_package_id);
CREATE INDEX IF NOT EXISTS idx_video_package_items_created_by ON video_package_items(created_by);
CREATE INDEX IF NOT EXISTS idx_video_package_items_updated_by ON video_package_items(updated_by);
CREATE INDEX IF NOT EXISTS idx_video_package_items_package_sort ON video_package_items(video_package_id, sort_order, id);

CREATE INDEX IF NOT EXISTS idx_pages_status ON pages(status);
CREATE INDEX IF NOT EXISTS idx_pages_page_type ON pages(page_type);
CREATE INDEX IF NOT EXISTS idx_pages_slug ON pages(url_slug);
DROP INDEX IF EXISTS idx_pages_parent_page_id;
CREATE INDEX IF NOT EXISTS idx_pages_parent_id ON pages(parent_id);
CREATE INDEX IF NOT EXISTS idx_pages_created_by ON pages(created_by);
CREATE INDEX IF NOT EXISTS idx_pages_modified_by ON pages(modified_by);
CREATE INDEX IF NOT EXISTS idx_pages_last_modified ON pages(last_modified);

CREATE INDEX IF NOT EXISTS idx_menus_key ON menus(menu_key);
CREATE INDEX IF NOT EXISTS idx_menus_created_by ON menus(created_by);
CREATE INDEX IF NOT EXISTS idx_menus_updated_by ON menus(updated_by);

CREATE INDEX IF NOT EXISTS idx_menu_items_menu_id ON menu_items(menu_id);
CREATE INDEX IF NOT EXISTS idx_menu_items_parent_id ON menu_items(parent_id);
CREATE INDEX IF NOT EXISTS idx_menu_items_page_id ON menu_items(page_id);
CREATE INDEX IF NOT EXISTS idx_menu_items_menu_sort ON menu_items(menu_id, parent_id, sort_order, id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_menu_items_page_per_menu
    ON menu_items(menu_id, page_id)
    WHERE page_id IS NOT NULL;

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

DROP TRIGGER IF EXISTS trg_video_packages_set_updated_at ON video_packages;
CREATE TRIGGER trg_video_packages_set_updated_at
BEFORE UPDATE ON video_packages
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_video_package_items_set_updated_at ON video_package_items;
CREATE TRIGGER trg_video_package_items_set_updated_at
BEFORE UPDATE ON video_package_items
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE OR REPLACE FUNCTION set_pages_timestamps()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    NEW.last_modified = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_pages_set_timestamps ON pages;
CREATE TRIGGER trg_pages_set_timestamps
BEFORE UPDATE ON pages
FOR EACH ROW
EXECUTE FUNCTION set_pages_timestamps();

DROP TRIGGER IF EXISTS trg_events_set_updated_at ON events;
CREATE TRIGGER trg_events_set_updated_at
BEFORE UPDATE ON events
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_menus_set_updated_at ON menus;
CREATE TRIGGER trg_menus_set_updated_at
BEFORE UPDATE ON menus
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_menu_items_set_updated_at ON menu_items;
CREATE TRIGGER trg_menu_items_set_updated_at
BEFORE UPDATE ON menu_items
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

DROP TRIGGER IF EXISTS trg_gallery_images_set_updated_at ON gallery_images;
CREATE TRIGGER trg_gallery_images_set_updated_at
BEFORE UPDATE ON gallery_images
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

ALTER TABLE events
    ALTER COLUMN teaser SET DEFAULT '';

ALTER TABLE events
    ALTER COLUMN teaser DROP NOT NULL;

ALTER TABLE events
    DROP CONSTRAINT IF EXISTS chk_events_teaser_not_blank;

INSERT INTO roles (role, priority)
VALUES
    ('Admin', 1),
    ('User', 2)
ON CONFLICT (role) DO UPDATE
SET priority = EXCLUDED.priority,
    updated_at = CURRENT_TIMESTAMP;

INSERT INTO menus (menu_key, name)
VALUES ('main', 'Main Website Navigation')
ON CONFLICT (menu_key) DO UPDATE
SET name = EXCLUDED.name,
    updated_at = CURRENT_TIMESTAMP;

INSERT INTO pages (
    page_title,
    url_slug,
    page_type,
    status,
    hero_image_enabled,
    seo_page_title,
    seo_page_description,
    last_modified
)
VALUES (
    'Videos',
    '/videos',
    'module',
    'published',
    FALSE,
    'Videos',
    'Videos',
    CURRENT_TIMESTAMP
)
ON CONFLICT (url_slug) DO NOTHING;

-- Page content builder schema.
-- Note: the 1:1 relation is stored as page_details.page_id instead of pages.page_detail_id
-- to avoid circular foreign keys and orphaned detail rows.

CREATE TABLE IF NOT EXISTS page_details (
    id SERIAL PRIMARY KEY,
    page_id INT NOT NULL UNIQUE,
    template_key VARCHAR(100) NOT NULL DEFAULT 'default',
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    schema_version INT NOT NULL DEFAULT 1,
    created_by INT NULL,
    updated_by INT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_page_details_page
        FOREIGN KEY (page_id) REFERENCES pages(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_page_details_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_page_details_updated_by
        FOREIGN KEY (updated_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT chk_page_details_template_key_not_blank
        CHECK (btrim(template_key) <> ''),

    CONSTRAINT chk_page_details_settings_is_object
        CHECK (jsonb_typeof(settings) = 'object'),

    CONSTRAINT chk_page_details_schema_version_positive
        CHECK (schema_version > 0)
);

CREATE TABLE IF NOT EXISTS page_sections (
    id SERIAL PRIMARY KEY,
    page_detail_id INT NOT NULL,
    section_name VARCHAR(150) NOT NULL,
    section_type VARCHAR(50) NOT NULL,
    sort_order INT NOT NULL DEFAULT -1,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_page_sections_page_detail
        FOREIGN KEY (page_detail_id) REFERENCES page_details(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT chk_page_sections_name_not_blank
        CHECK (btrim(section_name) <> ''),

    CONSTRAINT chk_page_sections_type
        CHECK (section_type IN (
            'typography',
            'gallery',
            'video',
            'document',
            'quote',
            'cta_banner',
            'header'
        )),

    CONSTRAINT chk_page_sections_sort_order
        CHECK (sort_order >= 0),

    CONSTRAINT chk_page_sections_settings_is_object
        CHECK (jsonb_typeof(settings) = 'object')
);

CREATE TABLE IF NOT EXISTS documents (
    id SERIAL PRIMARY KEY,
    display_name VARCHAR(255) NOT NULL,
    description TEXT,
    original_file_name VARCHAR(255),
    gcp_object_key TEXT,
    file_url TEXT NOT NULL,
    mime_type VARCHAR(255),
    file_size BIGINT,
    checksum_sha256 CHAR(64),
    created_by INT NULL,
    updated_by INT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_documents_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_documents_updated_by
        FOREIGN KEY (updated_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT chk_documents_display_name_not_blank
        CHECK (btrim(display_name) <> ''),

    CONSTRAINT chk_documents_file_url_not_blank
        CHECK (btrim(file_url) <> ''),

    CONSTRAINT chk_documents_file_size
        CHECK (file_size IS NULL OR file_size >= 0),

    CONSTRAINT chk_documents_checksum_sha256
        CHECK (
            checksum_sha256 IS NULL
            OR checksum_sha256 ~ '^[A-Fa-f0-9]{64}$'
        )
);

CREATE TABLE IF NOT EXISTS page_section_header_modules (
    page_section_id INT PRIMARY KEY,
    main_header_text VARCHAR(255) NOT NULL,
    sub_header_text VARCHAR(255),
    description TEXT NOT NULL DEFAULT '',
    hierarchy VARCHAR(20) NOT NULL DEFAULT 'h1_hero',
    text_align VARCHAR(20) NOT NULL DEFAULT 'left',
    underline_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_page_section_header_modules_section
        FOREIGN KEY (page_section_id) REFERENCES page_sections(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT chk_page_section_header_modules_main_header_text_not_blank
        CHECK (btrim(main_header_text) <> ''),

    CONSTRAINT chk_page_section_header_modules_hierarchy
        CHECK (hierarchy IN ('h1_hero', 'h2_section')),

    CONSTRAINT chk_page_section_header_modules_text_align
        CHECK (text_align IN ('left', 'center', 'right')),

    CONSTRAINT chk_page_section_header_modules_underline_enabled
        CHECK (NOT underline_enabled OR hierarchy = 'h2_section')
);

ALTER TABLE page_section_header_modules
    ADD COLUMN IF NOT EXISTS text_align VARCHAR(20);

ALTER TABLE page_section_header_modules
    ADD COLUMN IF NOT EXISTS description TEXT;

ALTER TABLE page_section_header_modules
    ADD COLUMN IF NOT EXISTS underline_enabled BOOLEAN;

UPDATE page_section_header_modules
SET description = ''
WHERE description IS NULL;

UPDATE page_section_header_modules
SET text_align = 'left'
WHERE text_align IS NULL OR btrim(text_align) = '';

UPDATE page_section_header_modules
SET underline_enabled = FALSE
WHERE underline_enabled IS NULL;

ALTER TABLE page_section_header_modules
    ALTER COLUMN description SET DEFAULT '',
    ALTER COLUMN description SET NOT NULL,
    ALTER COLUMN text_align SET DEFAULT 'left',
    ALTER COLUMN text_align SET NOT NULL,
    ALTER COLUMN underline_enabled SET DEFAULT FALSE,
    ALTER COLUMN underline_enabled SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_page_section_header_modules_text_align'
    ) THEN
        ALTER TABLE page_section_header_modules
            ADD CONSTRAINT chk_page_section_header_modules_text_align
            CHECK (text_align IN ('left', 'center', 'right'));
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_page_section_header_modules_underline_enabled'
    ) THEN
        ALTER TABLE page_section_header_modules
            ADD CONSTRAINT chk_page_section_header_modules_underline_enabled
            CHECK (NOT underline_enabled OR hierarchy = 'h2_section');
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS page_section_typography_modules (
    page_section_id INT PRIMARY KEY,
    body_html TEXT NOT NULL,
    body_text TEXT,
    text_align VARCHAR(20) NOT NULL DEFAULT 'left',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_page_section_typography_modules_section
        FOREIGN KEY (page_section_id) REFERENCES page_sections(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT chk_page_section_typography_modules_body_html_not_blank
        CHECK (btrim(body_html) <> ''),

    CONSTRAINT chk_page_section_typography_modules_text_align
        CHECK (text_align IN ('left', 'center', 'right'))
);

CREATE TABLE IF NOT EXISTS page_section_gallery_modules (
    page_section_id INT PRIMARY KEY,
    gallery_id INT NOT NULL,
    view_mode VARCHAR(20) NOT NULL DEFAULT 'grid',
    show_title_description BOOLEAN NOT NULL DEFAULT TRUE,
    auto_scroll_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_page_section_gallery_modules_section
        FOREIGN KEY (page_section_id) REFERENCES page_sections(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_page_section_gallery_modules_gallery
        FOREIGN KEY (gallery_id) REFERENCES galleries(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,

    CONSTRAINT chk_page_section_gallery_modules_view_mode
        CHECK (view_mode IN ('grid', 'carousel', 'masonry', 'focus', 'icons'))
);

ALTER TABLE page_section_gallery_modules
    ADD COLUMN IF NOT EXISTS show_title_description BOOLEAN,
    ADD COLUMN IF NOT EXISTS auto_scroll_enabled BOOLEAN;

UPDATE page_section_gallery_modules
SET show_title_description = TRUE
WHERE show_title_description IS NULL;

UPDATE page_section_gallery_modules
SET auto_scroll_enabled = FALSE
WHERE auto_scroll_enabled IS NULL;

ALTER TABLE page_section_gallery_modules
    ALTER COLUMN show_title_description SET DEFAULT TRUE,
    ALTER COLUMN show_title_description SET NOT NULL,
    ALTER COLUMN auto_scroll_enabled SET DEFAULT FALSE,
    ALTER COLUMN auto_scroll_enabled SET NOT NULL;

CREATE TABLE IF NOT EXISTS page_section_video_modules (
    page_section_id INT PRIMARY KEY,
    video_package_id INT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_page_section_video_modules_section
        FOREIGN KEY (page_section_id) REFERENCES page_sections(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_page_section_video_modules_video_package
        FOREIGN KEY (video_package_id) REFERENCES video_packages(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS page_section_quote_modules (
    page_section_id INT PRIMARY KEY,
    quote_content TEXT NOT NULL,
    attribution VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_page_section_quote_modules_section
        FOREIGN KEY (page_section_id) REFERENCES page_sections(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT chk_page_section_quote_modules_quote_content_not_blank
        CHECK (btrim(quote_content) <> '')
);

CREATE TABLE IF NOT EXISTS page_section_cta_banner_modules (
    page_section_id INT PRIMARY KEY,
    banner_heading VARCHAR(255) NOT NULL,
    banner_message VARCHAR(255),
    button_text VARCHAR(100),
    button_url TEXT,
    open_in_new_tab BOOLEAN NOT NULL DEFAULT FALSE,
    image_url TEXT,
    image_object_key TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_page_section_cta_banner_modules_section
        FOREIGN KEY (page_section_id) REFERENCES page_sections(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT chk_page_section_cta_banner_modules_banner_heading_not_blank
        CHECK (btrim(banner_heading) <> ''),

    CONSTRAINT chk_page_section_cta_banner_modules_button_pair
        CHECK (
            (btrim(COALESCE(button_text, '')) = '' AND btrim(COALESCE(button_url, '')) = '')
            OR
            (btrim(COALESCE(button_text, '')) <> '' AND btrim(COALESCE(button_url, '')) <> '')
        )
);

ALTER TABLE page_section_cta_banner_modules
    ADD COLUMN IF NOT EXISTS image_url TEXT,
    ADD COLUMN IF NOT EXISTS image_object_key TEXT;

CREATE TABLE IF NOT EXISTS page_section_documents (
    id SERIAL PRIMARY KEY,
    page_section_id INT NOT NULL,
    document_id INT NOT NULL,
    display_name_override VARCHAR(255),
    sort_order INT NOT NULL DEFAULT -1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_page_section_documents_section
        FOREIGN KEY (page_section_id) REFERENCES page_sections(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_page_section_documents_document
        FOREIGN KEY (document_id) REFERENCES documents(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,

    CONSTRAINT chk_page_section_documents_sort_order
        CHECK (sort_order >= 0)
);

ALTER TABLE page_sections
    DROP CONSTRAINT IF EXISTS chk_page_sections_type;

ALTER TABLE page_sections
    ADD CONSTRAINT chk_page_sections_type
        CHECK (section_type IN (
            'typography',
            'gallery',
            'video',
            'document',
            'quote',
            'cta_banner',
            'header'
        ));

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'uq_page_sections_page_detail_sort'
    ) THEN
        ALTER TABLE page_sections
            ADD CONSTRAINT uq_page_sections_page_detail_sort
            UNIQUE (page_detail_id, sort_order)
            DEFERRABLE INITIALLY IMMEDIATE;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'uq_page_section_documents_section_sort'
    ) THEN
        ALTER TABLE page_section_documents
            ADD CONSTRAINT uq_page_section_documents_section_sort
            UNIQUE (page_section_id, sort_order)
            DEFERRABLE INITIALLY IMMEDIATE;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'uq_page_section_documents_section_document'
    ) THEN
        ALTER TABLE page_section_documents
            ADD CONSTRAINT uq_page_section_documents_section_document
            UNIQUE (page_section_id, document_id);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_page_details_page_id ON page_details(page_id);
CREATE INDEX IF NOT EXISTS idx_page_details_created_by ON page_details(created_by);
CREATE INDEX IF NOT EXISTS idx_page_details_updated_by ON page_details(updated_by);

CREATE INDEX IF NOT EXISTS idx_page_sections_page_detail_id ON page_sections(page_detail_id);
CREATE INDEX IF NOT EXISTS idx_page_sections_type ON page_sections(section_type);
CREATE INDEX IF NOT EXISTS idx_page_sections_page_detail_sort ON page_sections(page_detail_id, sort_order, id);

CREATE INDEX IF NOT EXISTS idx_documents_display_name ON documents(display_name);
CREATE INDEX IF NOT EXISTS idx_documents_created_by ON documents(created_by);
CREATE INDEX IF NOT EXISTS idx_documents_updated_by ON documents(updated_by);
CREATE UNIQUE INDEX IF NOT EXISTS uq_documents_gcp_object_key
    ON documents(gcp_object_key)
    WHERE gcp_object_key IS NOT NULL AND btrim(gcp_object_key) <> '';

CREATE INDEX IF NOT EXISTS idx_page_section_gallery_modules_gallery_id
    ON page_section_gallery_modules(gallery_id);

CREATE INDEX IF NOT EXISTS idx_page_section_video_modules_video_package_id
    ON page_section_video_modules(video_package_id);

CREATE INDEX IF NOT EXISTS idx_page_section_documents_section_id
    ON page_section_documents(page_section_id);
CREATE INDEX IF NOT EXISTS idx_page_section_documents_document_id
    ON page_section_documents(document_id);
CREATE INDEX IF NOT EXISTS idx_page_section_documents_section_sort
    ON page_section_documents(page_section_id, sort_order, id);

CREATE OR REPLACE FUNCTION assign_page_section_sort_order()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.sort_order >= 0 THEN
        RETURN NEW;
    END IF;

    PERFORM 1
    FROM page_details
    WHERE id = NEW.page_detail_id
    FOR UPDATE;

    SELECT COALESCE(MAX(sort_order), -1) + 1
    INTO NEW.sort_order
    FROM page_sections
    WHERE page_detail_id = NEW.page_detail_id
      AND id <> COALESCE(NEW.id, 0);

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION assign_page_section_document_sort_order()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.sort_order >= 0 THEN
        RETURN NEW;
    END IF;

    PERFORM 1
    FROM page_sections
    WHERE id = NEW.page_section_id
    FOR UPDATE;

    SELECT COALESCE(MAX(sort_order), -1) + 1
    INTO NEW.sort_order
    FROM page_section_documents
    WHERE page_section_id = NEW.page_section_id
      AND id <> COALESCE(NEW.id, 0);

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION validate_page_section_type()
RETURNS TRIGGER AS $$
DECLARE
    expected_type TEXT := TG_ARGV[0];
    actual_type TEXT;
BEGIN
    SELECT section_type
    INTO actual_type
    FROM page_sections
    WHERE id = NEW.page_section_id;

    IF actual_type IS NULL THEN
        RAISE EXCEPTION 'page_section_id % does not exist', NEW.page_section_id;
    END IF;

    IF actual_type <> expected_type THEN
        RAISE EXCEPTION 'page_section_id % must reference a % section, found %',
            NEW.page_section_id,
            expected_type,
            actual_type;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION sync_page_from_page_details()
RETURNS TRIGGER AS $$
DECLARE
    target_page_id INT;
    target_modified_by INT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_page_id := OLD.page_id;
        target_modified_by := COALESCE(OLD.updated_by, OLD.created_by);
    ELSIF TG_OP = 'INSERT' THEN
        target_page_id := NEW.page_id;
        target_modified_by := COALESCE(NEW.updated_by, NEW.created_by);
    ELSE
        target_page_id := COALESCE(NEW.page_id, OLD.page_id);
        target_modified_by := COALESCE(NEW.updated_by, NEW.created_by, OLD.updated_by, OLD.created_by);
    END IF;

    IF target_page_id IS NOT NULL THEN
        UPDATE pages
        SET updated_at = CURRENT_TIMESTAMP,
            last_modified = CURRENT_TIMESTAMP,
            modified_by = COALESCE(target_modified_by, modified_by)
        WHERE id = target_page_id;
    END IF;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION touch_page_detail_from_section()
RETURNS TRIGGER AS $$
DECLARE
    target_page_detail_id INT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_page_detail_id := OLD.page_detail_id;
    ELSIF TG_OP = 'INSERT' THEN
        target_page_detail_id := NEW.page_detail_id;
    ELSE
        target_page_detail_id := COALESCE(NEW.page_detail_id, OLD.page_detail_id);
    END IF;

    IF target_page_detail_id IS NOT NULL THEN
        UPDATE page_details
        SET updated_at = CURRENT_TIMESTAMP
        WHERE id = target_page_detail_id;
    END IF;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION touch_page_detail_from_section_child()
RETURNS TRIGGER AS $$
DECLARE
    target_page_section_id INT;
    target_page_detail_id INT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_page_section_id := OLD.page_section_id;
    ELSIF TG_OP = 'INSERT' THEN
        target_page_section_id := NEW.page_section_id;
    ELSE
        target_page_section_id := COALESCE(NEW.page_section_id, OLD.page_section_id);
    END IF;

    IF target_page_section_id IS NULL THEN
        IF TG_OP = 'DELETE' THEN
            RETURN OLD;
        END IF;
        RETURN NEW;
    END IF;

    SELECT page_detail_id
    INTO target_page_detail_id
    FROM page_sections
    WHERE id = target_page_section_id;

    IF target_page_detail_id IS NOT NULL THEN
        UPDATE page_details
        SET updated_at = CURRENT_TIMESTAMP
        WHERE id = target_page_detail_id;
    END IF;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_page_details_set_updated_at ON page_details;
CREATE TRIGGER trg_page_details_set_updated_at
BEFORE UPDATE ON page_details
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_page_sections_assign_sort_order ON page_sections;
CREATE TRIGGER trg_page_sections_assign_sort_order
BEFORE INSERT OR UPDATE ON page_sections
FOR EACH ROW
EXECUTE FUNCTION assign_page_section_sort_order();

DROP TRIGGER IF EXISTS trg_page_sections_set_updated_at ON page_sections;
CREATE TRIGGER trg_page_sections_set_updated_at
BEFORE UPDATE ON page_sections
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_documents_set_updated_at ON documents;
CREATE TRIGGER trg_documents_set_updated_at
BEFORE UPDATE ON documents
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_page_section_header_modules_set_updated_at ON page_section_header_modules;
CREATE TRIGGER trg_page_section_header_modules_set_updated_at
BEFORE UPDATE ON page_section_header_modules
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_page_section_typography_modules_set_updated_at ON page_section_typography_modules;
CREATE TRIGGER trg_page_section_typography_modules_set_updated_at
BEFORE UPDATE ON page_section_typography_modules
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_page_section_gallery_modules_set_updated_at ON page_section_gallery_modules;
CREATE TRIGGER trg_page_section_gallery_modules_set_updated_at
BEFORE UPDATE ON page_section_gallery_modules
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_page_section_video_modules_set_updated_at ON page_section_video_modules;
CREATE TRIGGER trg_page_section_video_modules_set_updated_at
BEFORE UPDATE ON page_section_video_modules
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_page_section_quote_modules_set_updated_at ON page_section_quote_modules;
CREATE TRIGGER trg_page_section_quote_modules_set_updated_at
BEFORE UPDATE ON page_section_quote_modules
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_page_section_cta_banner_modules_set_updated_at ON page_section_cta_banner_modules;
CREATE TRIGGER trg_page_section_cta_banner_modules_set_updated_at
BEFORE UPDATE ON page_section_cta_banner_modules
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_page_section_documents_assign_sort_order ON page_section_documents;
CREATE TRIGGER trg_page_section_documents_assign_sort_order
BEFORE INSERT OR UPDATE ON page_section_documents
FOR EACH ROW
EXECUTE FUNCTION assign_page_section_document_sort_order();

DROP TRIGGER IF EXISTS trg_page_section_documents_set_updated_at ON page_section_documents;
CREATE TRIGGER trg_page_section_documents_set_updated_at
BEFORE UPDATE ON page_section_documents
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_page_section_header_modules_validate_type ON page_section_header_modules;
CREATE TRIGGER trg_page_section_header_modules_validate_type
BEFORE INSERT OR UPDATE ON page_section_header_modules
FOR EACH ROW
EXECUTE FUNCTION validate_page_section_type('header');

DROP TRIGGER IF EXISTS trg_page_section_typography_modules_validate_type ON page_section_typography_modules;
CREATE TRIGGER trg_page_section_typography_modules_validate_type
BEFORE INSERT OR UPDATE ON page_section_typography_modules
FOR EACH ROW
EXECUTE FUNCTION validate_page_section_type('typography');

DROP TRIGGER IF EXISTS trg_page_section_gallery_modules_validate_type ON page_section_gallery_modules;
CREATE TRIGGER trg_page_section_gallery_modules_validate_type
BEFORE INSERT OR UPDATE ON page_section_gallery_modules
FOR EACH ROW
EXECUTE FUNCTION validate_page_section_type('gallery');

DROP TRIGGER IF EXISTS trg_page_section_video_modules_validate_type ON page_section_video_modules;
CREATE TRIGGER trg_page_section_video_modules_validate_type
BEFORE INSERT OR UPDATE ON page_section_video_modules
FOR EACH ROW
EXECUTE FUNCTION validate_page_section_type('video');

DROP TRIGGER IF EXISTS trg_page_section_quote_modules_validate_type ON page_section_quote_modules;
CREATE TRIGGER trg_page_section_quote_modules_validate_type
BEFORE INSERT OR UPDATE ON page_section_quote_modules
FOR EACH ROW
EXECUTE FUNCTION validate_page_section_type('quote');

DROP TRIGGER IF EXISTS trg_page_section_cta_banner_modules_validate_type ON page_section_cta_banner_modules;
CREATE TRIGGER trg_page_section_cta_banner_modules_validate_type
BEFORE INSERT OR UPDATE ON page_section_cta_banner_modules
FOR EACH ROW
EXECUTE FUNCTION validate_page_section_type('cta_banner');

DROP TRIGGER IF EXISTS trg_page_section_documents_validate_type ON page_section_documents;
CREATE TRIGGER trg_page_section_documents_validate_type
BEFORE INSERT OR UPDATE ON page_section_documents
FOR EACH ROW
EXECUTE FUNCTION validate_page_section_type('document');

DROP TRIGGER IF EXISTS trg_page_details_sync_page ON page_details;
CREATE TRIGGER trg_page_details_sync_page
AFTER INSERT OR UPDATE OR DELETE ON page_details
FOR EACH ROW
EXECUTE FUNCTION sync_page_from_page_details();

DROP TRIGGER IF EXISTS trg_page_sections_touch_page_detail ON page_sections;
CREATE TRIGGER trg_page_sections_touch_page_detail
AFTER INSERT OR UPDATE OR DELETE ON page_sections
FOR EACH ROW
EXECUTE FUNCTION touch_page_detail_from_section();

DROP TRIGGER IF EXISTS trg_page_section_header_modules_touch_page_detail ON page_section_header_modules;
CREATE TRIGGER trg_page_section_header_modules_touch_page_detail
AFTER INSERT OR UPDATE OR DELETE ON page_section_header_modules
FOR EACH ROW
EXECUTE FUNCTION touch_page_detail_from_section_child();

DROP TRIGGER IF EXISTS trg_page_section_typography_modules_touch_page_detail ON page_section_typography_modules;
CREATE TRIGGER trg_page_section_typography_modules_touch_page_detail
AFTER INSERT OR UPDATE OR DELETE ON page_section_typography_modules
FOR EACH ROW
EXECUTE FUNCTION touch_page_detail_from_section_child();

DROP TRIGGER IF EXISTS trg_page_section_gallery_modules_touch_page_detail ON page_section_gallery_modules;
CREATE TRIGGER trg_page_section_gallery_modules_touch_page_detail
AFTER INSERT OR UPDATE OR DELETE ON page_section_gallery_modules
FOR EACH ROW
EXECUTE FUNCTION touch_page_detail_from_section_child();

DROP TRIGGER IF EXISTS trg_page_section_video_modules_touch_page_detail ON page_section_video_modules;
CREATE TRIGGER trg_page_section_video_modules_touch_page_detail
AFTER INSERT OR UPDATE OR DELETE ON page_section_video_modules
FOR EACH ROW
EXECUTE FUNCTION touch_page_detail_from_section_child();

DROP TRIGGER IF EXISTS trg_page_section_quote_modules_touch_page_detail ON page_section_quote_modules;
CREATE TRIGGER trg_page_section_quote_modules_touch_page_detail
AFTER INSERT OR UPDATE OR DELETE ON page_section_quote_modules
FOR EACH ROW
EXECUTE FUNCTION touch_page_detail_from_section_child();

DROP TRIGGER IF EXISTS trg_page_section_cta_banner_modules_touch_page_detail ON page_section_cta_banner_modules;
CREATE TRIGGER trg_page_section_cta_banner_modules_touch_page_detail
AFTER INSERT OR UPDATE OR DELETE ON page_section_cta_banner_modules
FOR EACH ROW
EXECUTE FUNCTION touch_page_detail_from_section_child();

DROP TRIGGER IF EXISTS trg_page_section_documents_touch_page_detail ON page_section_documents;
CREATE TRIGGER trg_page_section_documents_touch_page_detail
AFTER INSERT OR UPDATE OR DELETE ON page_section_documents
FOR EACH ROW
EXECUTE FUNCTION touch_page_detail_from_section_child();

-- Blogs Migration
-- Prerequisites: tables users(id) must already exist.

BEGIN;

CREATE TABLE IF NOT EXISTS blogs (
    id SERIAL PRIMARY KEY,
    publish_date DATE NOT NULL,
    heading VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    cover_image_url TEXT,
    cover_image_object_key TEXT,
    created_by INT,
    updated_by INT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_blogs_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_blogs_updated_by
        FOREIGN KEY (updated_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT chk_blogs_heading_not_blank
        CHECK (btrim(heading) <> ''),

    CONSTRAINT chk_blogs_description_not_blank
        CHECK (btrim(description) <> '')
);

-- Remove the initial JSONB prototype now that blog content is normalized.
ALTER TABLE blogs DROP COLUMN IF EXISTS blog_detail;

CREATE TABLE IF NOT EXISTS blog_details (
    id SERIAL PRIMARY KEY,
    blog_id INT NOT NULL UNIQUE,
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    schema_version INT NOT NULL DEFAULT 1,
    created_by INT,
    updated_by INT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_blog_details_blog
        FOREIGN KEY (blog_id) REFERENCES blogs(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    CONSTRAINT fk_blog_details_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,
    CONSTRAINT fk_blog_details_updated_by
        FOREIGN KEY (updated_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,
    CONSTRAINT chk_blog_details_settings_is_object
        CHECK (jsonb_typeof(settings) = 'object'),
    CONSTRAINT chk_blog_details_schema_version
        CHECK (schema_version > 0)
);

CREATE TABLE IF NOT EXISTS blog_sections (
    id SERIAL PRIMARY KEY,
    blog_detail_id INT NOT NULL,
    section_name VARCHAR(150) NOT NULL,
    section_type VARCHAR(50) NOT NULL,
    sort_order INT NOT NULL DEFAULT -1,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_blog_sections_detail
        FOREIGN KEY (blog_detail_id) REFERENCES blog_details(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    CONSTRAINT chk_blog_sections_name_not_blank
        CHECK (btrim(section_name) <> ''),
    CONSTRAINT chk_blog_sections_type
        CHECK (section_type IN ('heading', 'image', 'typography', 'action', 'video', 'animation')),
    CONSTRAINT chk_blog_sections_sort_order
        CHECK (sort_order >= 0),
    CONSTRAINT chk_blog_sections_settings_is_object
        CHECK (jsonb_typeof(settings) = 'object'),
    CONSTRAINT uq_blog_sections_detail_sort
        UNIQUE (blog_detail_id, sort_order)
);

CREATE TABLE IF NOT EXISTS blog_section_heading_modules (
    blog_section_id INT PRIMARY KEY,
    heading_text VARCHAR(255) NOT NULL,
    underline_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_blog_heading_section
        FOREIGN KEY (blog_section_id) REFERENCES blog_sections(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    CONSTRAINT chk_blog_heading_text_not_blank
        CHECK (btrim(heading_text) <> '')
);

CREATE TABLE IF NOT EXISTS blog_section_image_modules (
    blog_section_id INT PRIMARY KEY,
    image_url TEXT,
    image_object_key TEXT,
    caption TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_blog_image_section
        FOREIGN KEY (blog_section_id) REFERENCES blog_sections(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    CONSTRAINT chk_blog_image_reference
        CHECK (btrim(image_url) <> '' OR btrim(COALESCE(image_object_key, '')) <> '')
);

CREATE TABLE IF NOT EXISTS blog_section_typography_modules (
    blog_section_id INT PRIMARY KEY,
    body_html TEXT NOT NULL,
    body_text TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_blog_typography_section
        FOREIGN KEY (blog_section_id) REFERENCES blog_sections(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    CONSTRAINT chk_blog_typography_body_not_blank
        CHECK (btrim(body_text) <> '')
);

CREATE TABLE IF NOT EXISTS blog_section_action_modules (
    blog_section_id INT PRIMARY KEY,
    action_text VARCHAR(255) NOT NULL,
    action_type VARCHAR(20) NOT NULL,
    target_url TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_blog_action_section
        FOREIGN KEY (blog_section_id) REFERENCES blog_sections(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    CONSTRAINT chk_blog_action_text_not_blank
        CHECK (btrim(action_text) <> ''),
    CONSTRAINT chk_blog_action_type
        CHECK (action_type IN ('link', 'video')),
    CONSTRAINT chk_blog_action_target_not_blank
        CHECK (btrim(target_url) <> '')
);

CREATE TABLE IF NOT EXISTS blog_section_video_modules (
    blog_section_id INT PRIMARY KEY,
    youtube_url TEXT NOT NULL,
    caption TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_blog_video_section
        FOREIGN KEY (blog_section_id) REFERENCES blog_sections(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    CONSTRAINT chk_blog_video_url_not_blank
        CHECK (btrim(youtube_url) <> '')
);

CREATE TABLE IF NOT EXISTS blog_section_animation_modules (
    blog_section_id INT PRIMARY KEY,
    navigation VARCHAR(20) NOT NULL,
    image_position VARCHAR(20) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_blog_animation_section
        FOREIGN KEY (blog_section_id) REFERENCES blog_sections(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    CONSTRAINT chk_blog_animation_navigation
        CHECK (navigation IN ('vertical', 'horizontal')),
    CONSTRAINT chk_blog_animation_image_position
        CHECK (image_position IN ('left', 'right'))
);

CREATE TABLE IF NOT EXISTS blog_animation_items (
    id SERIAL PRIMARY KEY,
    blog_section_id INT NOT NULL,
    sort_order INT NOT NULL DEFAULT -1,
    heading VARCHAR(255) NOT NULL,
    sub_heading VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    image_url TEXT,
    image_object_key TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_blog_animation_items_section
        FOREIGN KEY (blog_section_id) REFERENCES blog_section_animation_modules(blog_section_id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    CONSTRAINT uq_blog_animation_items_section_sort
        UNIQUE (blog_section_id, sort_order),
    CONSTRAINT chk_blog_animation_items_sort_order
        CHECK (sort_order >= 0),
    CONSTRAINT chk_blog_animation_items_heading_not_blank
        CHECK (btrim(heading) <> ''),
    CONSTRAINT chk_blog_animation_items_sub_heading_not_blank
        CHECK (btrim(sub_heading) <> ''),
    CONSTRAINT chk_blog_animation_items_description_not_blank
        CHECK (btrim(description) <> ''),
    CONSTRAINT chk_blog_animation_items_image_reference
        CHECK (btrim(image_url) <> '' OR btrim(COALESCE(image_object_key, '')) <> '')
);

CREATE INDEX IF NOT EXISTS idx_blogs_publish_date
    ON blogs(publish_date DESC);

CREATE INDEX IF NOT EXISTS idx_blogs_heading
    ON blogs(heading);

CREATE INDEX IF NOT EXISTS idx_blogs_created_by
    ON blogs(created_by);

CREATE INDEX IF NOT EXISTS idx_blogs_updated_by
    ON blogs(updated_by);

CREATE INDEX IF NOT EXISTS idx_blog_details_blog_id
    ON blog_details(blog_id);

CREATE INDEX IF NOT EXISTS idx_blog_sections_detail_sort
    ON blog_sections(blog_detail_id, sort_order, id);

CREATE INDEX IF NOT EXISTS idx_blog_sections_type
    ON blog_sections(section_type);

CREATE INDEX IF NOT EXISTS idx_blog_animation_items_section_sort
    ON blog_animation_items(blog_section_id, sort_order, id);

CREATE OR REPLACE FUNCTION assign_blog_section_sort_order()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.sort_order >= 0 THEN
        RETURN NEW;
    END IF;

    PERFORM 1
    FROM blog_details
    WHERE id = NEW.blog_detail_id
    FOR UPDATE;

    SELECT COALESCE(MAX(sort_order), -1) + 1
    INTO NEW.sort_order
    FROM blog_sections
    WHERE blog_detail_id = NEW.blog_detail_id
      AND id <> COALESCE(NEW.id, 0);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION assign_blog_animation_item_sort_order()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.sort_order >= 0 THEN
        RETURN NEW;
    END IF;

    PERFORM 1
    FROM blog_sections
    WHERE id = NEW.blog_section_id
    FOR UPDATE;

    SELECT COALESCE(MAX(sort_order), -1) + 1
    INTO NEW.sort_order
    FROM blog_animation_items
    WHERE blog_section_id = NEW.blog_section_id
      AND id <> COALESCE(NEW.id, 0);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION touch_blog_from_detail()
RETURNS TRIGGER AS $$
DECLARE
    target_blog_id INT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_blog_id := OLD.blog_id;
    ELSIF TG_OP = 'INSERT' THEN
        target_blog_id := NEW.blog_id;
    ELSE
        target_blog_id := COALESCE(NEW.blog_id, OLD.blog_id);
    END IF;

    UPDATE blogs
    SET updated_at = CURRENT_TIMESTAMP
    WHERE id = target_blog_id;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION touch_blog_detail_from_section()
RETURNS TRIGGER AS $$
DECLARE
    target_detail_id INT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_detail_id := OLD.blog_detail_id;
    ELSIF TG_OP = 'INSERT' THEN
        target_detail_id := NEW.blog_detail_id;
    ELSE
        target_detail_id := COALESCE(NEW.blog_detail_id, OLD.blog_detail_id);
    END IF;

    UPDATE blog_details
    SET updated_at = CURRENT_TIMESTAMP
    WHERE id = target_detail_id;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION touch_blog_detail_from_section_child()
RETURNS TRIGGER AS $$
DECLARE
    target_section_id INT;
    target_detail_id INT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_section_id := OLD.blog_section_id;
    ELSIF TG_OP = 'INSERT' THEN
        target_section_id := NEW.blog_section_id;
    ELSE
        target_section_id := COALESCE(NEW.blog_section_id, OLD.blog_section_id);
    END IF;

    SELECT blog_detail_id
    INTO target_detail_id
    FROM blog_sections
    WHERE id = target_section_id;

    UPDATE blog_details
    SET updated_at = CURRENT_TIMESTAMP
    WHERE id = target_detail_id;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION validate_blog_section_type()
RETURNS TRIGGER AS $$
DECLARE
    expected_type TEXT := TG_ARGV[0];
    actual_type TEXT;
BEGIN
    SELECT section_type
    INTO actual_type
    FROM blog_sections
    WHERE id = NEW.blog_section_id;

    IF actual_type IS NULL THEN
        RAISE EXCEPTION 'blog_section_id % does not exist', NEW.blog_section_id;
    END IF;
    IF actual_type <> expected_type THEN
        RAISE EXCEPTION 'blog_section_id % must reference a % section, found %',
            NEW.blog_section_id, expected_type, actual_type;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_blogs_set_updated_at ON blogs;
CREATE TRIGGER trg_blogs_set_updated_at
BEFORE UPDATE ON blogs
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_blog_details_set_updated_at ON blog_details;
CREATE TRIGGER trg_blog_details_set_updated_at
BEFORE UPDATE ON blog_details
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_blog_details_touch_blog ON blog_details;
CREATE TRIGGER trg_blog_details_touch_blog
AFTER INSERT OR UPDATE OR DELETE ON blog_details
FOR EACH ROW
EXECUTE FUNCTION touch_blog_from_detail();

DROP TRIGGER IF EXISTS trg_blog_sections_assign_sort_order ON blog_sections;
CREATE TRIGGER trg_blog_sections_assign_sort_order
BEFORE INSERT OR UPDATE ON blog_sections
FOR EACH ROW
EXECUTE FUNCTION assign_blog_section_sort_order();

DROP TRIGGER IF EXISTS trg_blog_sections_set_updated_at ON blog_sections;
CREATE TRIGGER trg_blog_sections_set_updated_at
BEFORE UPDATE ON blog_sections
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_blog_sections_touch_detail ON blog_sections;
CREATE TRIGGER trg_blog_sections_touch_detail
AFTER INSERT OR UPDATE OR DELETE ON blog_sections
FOR EACH ROW
EXECUTE FUNCTION touch_blog_detail_from_section();

DROP TRIGGER IF EXISTS trg_blog_animation_items_assign_sort_order ON blog_animation_items;
CREATE TRIGGER trg_blog_animation_items_assign_sort_order
BEFORE INSERT OR UPDATE ON blog_animation_items
FOR EACH ROW
EXECUTE FUNCTION assign_blog_animation_item_sort_order();

DROP TRIGGER IF EXISTS trg_blog_animation_items_set_updated_at ON blog_animation_items;
CREATE TRIGGER trg_blog_animation_items_set_updated_at
BEFORE UPDATE ON blog_animation_items
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_blog_animation_items_touch_detail ON blog_animation_items;
CREATE TRIGGER trg_blog_animation_items_touch_detail
AFTER INSERT OR UPDATE OR DELETE ON blog_animation_items
FOR EACH ROW
EXECUTE FUNCTION touch_blog_detail_from_section_child();

DO $$
DECLARE
    table_name TEXT;
    expected_type TEXT;
BEGIN
    FOR table_name, expected_type IN
        SELECT *
        FROM (VALUES
            ('blog_section_heading_modules', 'heading'),
            ('blog_section_image_modules', 'image'),
            ('blog_section_typography_modules', 'typography'),
            ('blog_section_action_modules', 'action'),
            ('blog_section_video_modules', 'video'),
            ('blog_section_animation_modules', 'animation')
        ) AS module_tables(table_name, expected_type)
    LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS %I ON %I', 'trg_' || table_name || '_set_updated_at', table_name);
        EXECUTE format(
            'CREATE TRIGGER %I BEFORE UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION set_updated_at()',
            'trg_' || table_name || '_set_updated_at',
            table_name
        );
        EXECUTE format('DROP TRIGGER IF EXISTS %I ON %I', 'trg_' || table_name || '_touch_detail', table_name);
        EXECUTE format(
            'CREATE TRIGGER %I AFTER INSERT OR UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION touch_blog_detail_from_section_child()',
            'trg_' || table_name || '_touch_detail',
            table_name
        );
        EXECUTE format('DROP TRIGGER IF EXISTS %I ON %I', 'trg_' || table_name || '_validate_type', table_name);
        EXECUTE format(
            'CREATE TRIGGER %I BEFORE INSERT OR UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION validate_blog_section_type(%L)',
            'trg_' || table_name || '_validate_type',
            table_name,
            expected_type
        );
    END LOOP;
END;
$$;

COMMIT;

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

-- Newsletters Migration
-- Prerequisites: tables users(id) must already exist.

BEGIN;

CREATE TABLE IF NOT EXISTS newsletter_entries (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    category VARCHAR(20) NOT NULL DEFAULT '',
    send_date DATE NOT NULL,
    content_html TEXT NOT NULL DEFAULT '',
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    visibility VARCHAR(30) NOT NULL DEFAULT 'public',
    publish_at TIMESTAMP,
    created_by INT,
    updated_by INT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_newsletter_entries_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_newsletter_entries_updated_by
        FOREIGN KEY (updated_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT chk_newsletter_entries_title_not_blank
        CHECK (btrim(title) <> ''),

    CONSTRAINT chk_newsletter_entries_category
        CHECK (category IN ('', 'csaa', 'cst')),

    CONSTRAINT chk_newsletter_entries_status
        CHECK (status IN ('draft', 'published', 'scheduled')),

    CONSTRAINT chk_newsletter_entries_visibility
        CHECK (visibility IN ('public', 'private'))
);

CREATE TABLE IF NOT EXISTS newsletter_media (
    id SERIAL PRIMARY KEY,
    newsletter_entry_id INT NOT NULL,
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

    CONSTRAINT fk_newsletter_media_newsletter_entry
        FOREIGN KEY (newsletter_entry_id) REFERENCES newsletter_entries(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_newsletter_media_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_newsletter_media_updated_by
        FOREIGN KEY (updated_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT chk_newsletter_media_display_name_not_blank
        CHECK (btrim(display_name) <> ''),

    CONSTRAINT chk_newsletter_media_file_url_not_blank
        CHECK (btrim(file_url) <> ''),

    CONSTRAINT chk_newsletter_media_media_role
        CHECK (media_role IN ('attachment')),

    CONSTRAINT chk_newsletter_media_file_size
        CHECK (file_size IS NULL OR file_size >= 0),

    CONSTRAINT chk_newsletter_media_sort_order
        CHECK (sort_order >= 0)
);

CREATE INDEX IF NOT EXISTS idx_newsletter_entries_send_date
    ON newsletter_entries(send_date DESC);

CREATE INDEX IF NOT EXISTS idx_newsletter_entries_status
    ON newsletter_entries(status);

CREATE INDEX IF NOT EXISTS idx_newsletter_entries_visibility
    ON newsletter_entries(visibility);

CREATE INDEX IF NOT EXISTS idx_newsletter_entries_category
    ON newsletter_entries(category);

CREATE INDEX IF NOT EXISTS idx_newsletter_entries_created_by
    ON newsletter_entries(created_by);

CREATE INDEX IF NOT EXISTS idx_newsletter_entries_updated_by
    ON newsletter_entries(updated_by);

CREATE INDEX IF NOT EXISTS idx_newsletter_media_newsletter_entry_id
    ON newsletter_media(newsletter_entry_id);

CREATE INDEX IF NOT EXISTS idx_newsletter_media_media_role
    ON newsletter_media(media_role);

CREATE INDEX IF NOT EXISTS idx_newsletter_media_sort_order
    ON newsletter_media(newsletter_entry_id, sort_order, id);

DROP TRIGGER IF EXISTS trg_newsletter_entries_set_updated_at ON newsletter_entries;
CREATE TRIGGER trg_newsletter_entries_set_updated_at
BEFORE UPDATE ON newsletter_entries
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_newsletter_media_set_updated_at ON newsletter_media;
CREATE TRIGGER trg_newsletter_media_set_updated_at
BEFORE UPDATE ON newsletter_media
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE OR REPLACE FUNCTION assign_newsletter_media_sort_order()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.sort_order >= 0 THEN
        RETURN NEW;
    END IF;

    PERFORM 1
    FROM newsletter_entries
    WHERE id = NEW.newsletter_entry_id
    FOR UPDATE;

    SELECT COALESCE(MAX(sort_order), -1) + 1
    INTO NEW.sort_order
    FROM newsletter_media
    WHERE newsletter_entry_id = NEW.newsletter_entry_id
      AND id <> COALESCE(NEW.id, 0);

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_newsletter_media_assign_sort_order ON newsletter_media;
CREATE TRIGGER trg_newsletter_media_assign_sort_order
BEFORE INSERT OR UPDATE ON newsletter_media
FOR EACH ROW
EXECUTE FUNCTION assign_newsletter_media_sort_order();

CREATE TABLE IF NOT EXISTS resource_entries (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category VARCHAR(50) NOT NULL,
    visibility VARCHAR(20) NOT NULL DEFAULT 'public',
    link_url TEXT,
    file_name VARCHAR(255),
    gcp_object_key TEXT,
    file_url TEXT,
    mime_type VARCHAR(255),
    file_size BIGINT,
    created_by INT,
    updated_by INT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_resource_entries_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_resource_entries_updated_by
        FOREIGN KEY (updated_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL
);

ALTER TABLE resource_entries
    ADD COLUMN IF NOT EXISTS description TEXT;

ALTER TABLE resource_entries
    ADD COLUMN IF NOT EXISTS link_url TEXT;

ALTER TABLE resource_entries
    ADD COLUMN IF NOT EXISTS gcp_object_key TEXT;

UPDATE resource_entries
SET category = CASE category
    WHEN 'brand_identity' THEN 'media'
    WHEN 'governance_legal' THEN 'report'
    WHEN 'training_manuals' THEN 'educational'
    WHEN 'media_kits' THEN 'media'
    ELSE category
END;

UPDATE resource_entries
SET category = 'report'
WHERE category NOT IN ('educational', 'media', 'link', 'report');

UPDATE resource_entries
SET description = COALESCE(NULLIF(BTRIM(description), ''), NULLIF(BTRIM(name), ''), 'Resource');

UPDATE resource_entries
SET link_url = NULLIF(BTRIM(link_url), '');

UPDATE resource_entries
SET file_name = NULLIF(BTRIM(file_name), '');

UPDATE resource_entries
SET file_url = NULLIF(BTRIM(file_url), '');

UPDATE resource_entries
SET gcp_object_key = NULLIF(BTRIM(gcp_object_key), '');

UPDATE resource_entries
SET mime_type = NULLIF(BTRIM(mime_type), '');

ALTER TABLE resource_entries
    ALTER COLUMN description SET DEFAULT '';

ALTER TABLE resource_entries
    ALTER COLUMN description SET NOT NULL;

ALTER TABLE resource_entries
    ALTER COLUMN file_name DROP NOT NULL;

ALTER TABLE resource_entries
    ALTER COLUMN file_url DROP NOT NULL;

ALTER TABLE resource_entries
    ALTER COLUMN mime_type DROP NOT NULL;

ALTER TABLE resource_entries
    ALTER COLUMN file_size DROP NOT NULL;

ALTER TABLE resource_entries
    DROP CONSTRAINT IF EXISTS chk_resource_entries_name_not_blank;

ALTER TABLE resource_entries
    DROP CONSTRAINT IF EXISTS chk_resource_entries_description_not_blank;

ALTER TABLE resource_entries
    DROP CONSTRAINT IF EXISTS chk_resource_entries_category;

ALTER TABLE resource_entries
    DROP CONSTRAINT IF EXISTS chk_resource_entries_visibility;

ALTER TABLE resource_entries
    DROP CONSTRAINT IF EXISTS chk_resource_entries_file_name_not_blank;

ALTER TABLE resource_entries
    DROP CONSTRAINT IF EXISTS chk_resource_entries_file_url_not_blank;

ALTER TABLE resource_entries
    DROP CONSTRAINT IF EXISTS chk_resource_entries_link_or_document;

ALTER TABLE resource_entries
    DROP CONSTRAINT IF EXISTS chk_resource_entries_file_size;

ALTER TABLE resource_entries
    ADD CONSTRAINT chk_resource_entries_name_not_blank
        CHECK (BTRIM(name) <> '');

ALTER TABLE resource_entries
    ADD CONSTRAINT chk_resource_entries_description_not_blank
        CHECK (BTRIM(description) <> '');

ALTER TABLE resource_entries
    ADD CONSTRAINT chk_resource_entries_category
        CHECK (category IN ('educational', 'media', 'link', 'report'));

ALTER TABLE resource_entries
    ADD CONSTRAINT chk_resource_entries_visibility
        CHECK (visibility IN ('public', 'internal'));

ALTER TABLE resource_entries
    ADD CONSTRAINT chk_resource_entries_file_size
        CHECK (file_size IS NULL OR file_size >= 0);

ALTER TABLE resource_entries
    ADD CONSTRAINT chk_resource_entries_link_or_document
        CHECK (
            (
                category = 'link'
                AND BTRIM(COALESCE(link_url, '')) <> ''
                AND BTRIM(COALESCE(file_name, '')) = ''
                AND BTRIM(COALESCE(file_url, '')) = ''
            )
            OR
            (
                category <> 'link'
                AND BTRIM(COALESCE(link_url, '')) = ''
                AND BTRIM(COALESCE(file_name, '')) <> ''
                AND BTRIM(COALESCE(file_url, '')) <> ''
            )
        );

CREATE INDEX IF NOT EXISTS idx_resource_entries_category
    ON resource_entries(category);

CREATE INDEX IF NOT EXISTS idx_resource_entries_visibility
    ON resource_entries(visibility);

CREATE INDEX IF NOT EXISTS idx_resource_entries_updated_at
    ON resource_entries(updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_resource_entries_category
    ON resource_entries(category);

CREATE INDEX IF NOT EXISTS idx_resource_entries_visibility
    ON resource_entries(visibility);

CREATE INDEX IF NOT EXISTS idx_resource_entries_updated_at
    ON resource_entries(updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_resource_entries_created_by
    ON resource_entries(created_by);

CREATE INDEX IF NOT EXISTS idx_resource_entries_updated_by
    ON resource_entries(updated_by);

DROP TRIGGER IF EXISTS trg_resource_entries_set_updated_at ON resource_entries;
CREATE TRIGGER trg_resource_entries_set_updated_at
BEFORE UPDATE ON resource_entries
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- Memorial Migration
-- Prerequisites: table users(id) must already exist.

CREATE TABLE IF NOT EXISTS memorial_entries (
    id SERIAL PRIMARY KEY,
    full_name VARCHAR(255) NOT NULL,
    affiliation VARCHAR(255),
    category VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    biography TEXT,
    date_of_birth DATE,
    date_of_passing DATE,
    published_at TIMESTAMP,
    portrait_file_name VARCHAR(255),
    portrait_gcp_object_key TEXT,
    portrait_file_url TEXT,
    portrait_mime_type VARCHAR(255),
    portrait_file_size BIGINT,
    created_by INT,
    updated_by INT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_memorial_entries_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_memorial_entries_updated_by
        FOREIGN KEY (updated_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT chk_memorial_entries_full_name_not_blank
        CHECK (btrim(full_name) <> ''),

    CONSTRAINT chk_memorial_entries_category
        CHECK (category IN ('alumnus', 'veteran', 'founder', 'friend')),

    CONSTRAINT chk_memorial_entries_status
        CHECK (status IN ('draft', 'review', 'published')),

    CONSTRAINT chk_memorial_entries_dates
        CHECK (
            date_of_birth IS NULL
            OR date_of_passing IS NULL
            OR date_of_passing >= date_of_birth
        ),

    CONSTRAINT chk_memorial_entries_portrait_file_name
        CHECK (portrait_file_name IS NULL OR btrim(portrait_file_name) <> ''),

    CONSTRAINT chk_memorial_entries_portrait_file_size
        CHECK (portrait_file_size IS NULL OR portrait_file_size >= 0)
);

CREATE INDEX IF NOT EXISTS idx_memorial_entries_category
    ON memorial_entries(category);

CREATE INDEX IF NOT EXISTS idx_memorial_entries_status
    ON memorial_entries(status);

CREATE INDEX IF NOT EXISTS idx_memorial_entries_updated_at
    ON memorial_entries(updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_memorial_entries_published_at
    ON memorial_entries(published_at DESC);

CREATE INDEX IF NOT EXISTS idx_memorial_entries_created_by
    ON memorial_entries(created_by);

CREATE INDEX IF NOT EXISTS idx_memorial_entries_updated_by
    ON memorial_entries(updated_by);

DROP TRIGGER IF EXISTS trg_memorial_entries_set_updated_at ON memorial_entries;
CREATE TRIGGER trg_memorial_entries_set_updated_at
BEFORE UPDATE ON memorial_entries
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS memorial_gallery_images (
    id SERIAL PRIMARY KEY,
    memorial_entry_id INT NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    gcp_object_key TEXT,
    file_url TEXT NOT NULL,
    mime_type VARCHAR(255),
    file_size BIGINT,
    sort_order INT NOT NULL DEFAULT 0,
    uploaded_by INT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_memorial_gallery_images_entry
        FOREIGN KEY (memorial_entry_id) REFERENCES memorial_entries(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_memorial_gallery_images_uploaded_by
        FOREIGN KEY (uploaded_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT chk_memorial_gallery_images_file_name
        CHECK (btrim(file_name) <> ''),

    CONSTRAINT chk_memorial_gallery_images_file_url
        CHECK (btrim(file_url) <> ''),

    CONSTRAINT chk_memorial_gallery_images_file_size
        CHECK (file_size IS NULL OR file_size >= 0),

    CONSTRAINT chk_memorial_gallery_images_sort_order
        CHECK (sort_order >= 0)
);

CREATE INDEX IF NOT EXISTS idx_memorial_gallery_images_entry_id
    ON memorial_gallery_images(memorial_entry_id);

CREATE INDEX IF NOT EXISTS idx_memorial_gallery_images_sort_order
    ON memorial_gallery_images(memorial_entry_id, sort_order, id);

CREATE INDEX IF NOT EXISTS idx_memorial_gallery_images_uploaded_by
    ON memorial_gallery_images(uploaded_by);

DROP TRIGGER IF EXISTS trg_memorial_gallery_images_set_updated_at ON memorial_gallery_images;
CREATE TRIGGER trg_memorial_gallery_images_set_updated_at
BEFORE UPDATE ON memorial_gallery_images
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- Password Reset OTP Table for Forgot Password Functionality
CREATE TABLE IF NOT EXISTS password_reset_otps (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    email VARCHAR(100) NOT NULL,
    otp VARCHAR(6) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    is_used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_password_reset_otps_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    
    CONSTRAINT chk_password_reset_otps_otp_length
        CHECK (LENGTH(otp) = 6 AND otp ~ '^\d+$')
);

CREATE INDEX IF NOT EXISTS idx_password_reset_otps_user_id 
    ON password_reset_otps(user_id);

CREATE INDEX IF NOT EXISTS idx_password_reset_otps_email 
    ON password_reset_otps(email);

CREATE INDEX IF NOT EXISTS idx_password_reset_otps_expires_at 
    ON password_reset_otps(expires_at);

CREATE INDEX IF NOT EXISTS idx_password_reset_otps_email_unused 
    ON password_reset_otps(email, is_used, expires_at);

-- Trigger for auto-updating updated_at on password_reset_otps
DROP TRIGGER IF EXISTS trg_password_reset_otps_set_updated_at ON password_reset_otps;
CREATE TRIGGER trg_password_reset_otps_set_updated_at
BEFORE UPDATE ON password_reset_otps
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- Books Module Migration
-- Prerequisites: table users(id) must already exist.

CREATE TABLE IF NOT EXISTS books (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    admin_notification_emails TEXT[] NOT NULL DEFAULT '{}',
    active_version_id INT,
    created_by INT,
    updated_by INT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_books_title_not_blank
        CHECK (BTRIM(title) <> ''),

    CONSTRAINT fk_books_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_books_updated_by
        FOREIGN KEY (updated_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_books_title
    ON books(title);

CREATE INDEX IF NOT EXISTS idx_books_active_version_id
    ON books(active_version_id);

CREATE INDEX IF NOT EXISTS idx_books_updated_at
    ON books(updated_at DESC);

DROP TRIGGER IF EXISTS trg_books_set_updated_at ON books;
CREATE TRIGGER trg_books_set_updated_at
BEFORE UPDATE ON books
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS book_versions (
    id SERIAL PRIMARY KEY,
    book_id INT NOT NULL,
    version_number INT NOT NULL,
    source_page_count INT NOT NULL,
    content_template_page_number INT NOT NULL,
    section_template_page_number INT NOT NULL,
    allow_page_image BOOLEAN NOT NULL DEFAULT FALSE,
    allow_new_sections BOOLEAN NOT NULL DEFAULT TRUE,
    layout_settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_pdf_file_name VARCHAR(255) NOT NULL,
    source_pdf_file_url TEXT,
    source_pdf_storage_uri TEXT,
    source_pdf_object_key TEXT,
    content_template_pdf_file_name VARCHAR(255) NOT NULL DEFAULT '',
    content_template_pdf_file_url TEXT,
    content_template_pdf_storage_uri TEXT,
    content_template_pdf_object_key TEXT,
    content_image_template_pdf_file_name VARCHAR(255) NOT NULL DEFAULT '',
    content_image_template_pdf_file_url TEXT,
    content_image_template_pdf_storage_uri TEXT,
    content_image_template_pdf_object_key TEXT,
    section_template_pdf_file_name VARCHAR(255) NOT NULL DEFAULT '',
    section_template_pdf_file_url TEXT,
    section_template_pdf_storage_uri TEXT,
    section_template_pdf_object_key TEXT,
    generated_pdf_file_name VARCHAR(255) NOT NULL DEFAULT '',
    generated_pdf_file_url TEXT,
    generated_pdf_storage_uri TEXT,
    generated_pdf_object_key TEXT,
    last_generated_at TIMESTAMP,
    created_by INT,
    updated_by INT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_book_versions_book
        FOREIGN KEY (book_id) REFERENCES books(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_book_versions_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_book_versions_updated_by
        FOREIGN KEY (updated_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT chk_book_versions_version_number_positive
        CHECK (version_number > 0),

    CONSTRAINT chk_book_versions_source_page_count_positive
        CHECK (source_page_count > 0),

    CONSTRAINT chk_book_versions_content_template_page_number_positive
        CHECK (content_template_page_number > 0),

    CONSTRAINT chk_book_versions_section_template_page_number_positive
        CHECK (section_template_page_number > 0),

    CONSTRAINT chk_book_versions_template_pages_within_source
        CHECK (
            content_template_page_number <= source_page_count
            AND section_template_page_number <= source_page_count
        ),

    CONSTRAINT chk_book_versions_source_pdf_file_name_not_blank
        CHECK (BTRIM(source_pdf_file_name) <> '')
);

ALTER TABLE book_versions
    ADD COLUMN IF NOT EXISTS content_template_pdf_file_name VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS content_template_pdf_file_url TEXT,
    ADD COLUMN IF NOT EXISTS content_template_pdf_storage_uri TEXT,
    ADD COLUMN IF NOT EXISTS content_template_pdf_object_key TEXT,
    ADD COLUMN IF NOT EXISTS content_image_template_pdf_file_name VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS content_image_template_pdf_file_url TEXT,
    ADD COLUMN IF NOT EXISTS content_image_template_pdf_storage_uri TEXT,
    ADD COLUMN IF NOT EXISTS content_image_template_pdf_object_key TEXT,
    ADD COLUMN IF NOT EXISTS section_template_pdf_file_name VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS section_template_pdf_file_url TEXT,
    ADD COLUMN IF NOT EXISTS section_template_pdf_storage_uri TEXT,
    ADD COLUMN IF NOT EXISTS section_template_pdf_object_key TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS ux_book_versions_book_id_version_number
    ON book_versions(book_id, version_number);

CREATE INDEX IF NOT EXISTS idx_book_versions_book_id
    ON book_versions(book_id);

CREATE INDEX IF NOT EXISTS idx_book_versions_last_generated_at
    ON book_versions(last_generated_at DESC);

CREATE INDEX IF NOT EXISTS idx_book_versions_updated_at
    ON book_versions(updated_at DESC);

DROP TRIGGER IF EXISTS trg_book_versions_set_updated_at ON book_versions;
CREATE TRIGGER trg_book_versions_set_updated_at
BEFORE UPDATE ON book_versions
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

ALTER TABLE books
    DROP CONSTRAINT IF EXISTS fk_books_active_version;

ALTER TABLE books
    ADD CONSTRAINT fk_books_active_version
        FOREIGN KEY (active_version_id) REFERENCES book_versions(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS book_version_sections (
    id SERIAL PRIMARY KEY,
    book_version_id INT NOT NULL,
    name VARCHAR(150) NOT NULL,
    source_start_page INT,
    source_end_page INT,
    current_start_page INT NOT NULL DEFAULT 0,
    current_end_page INT NOT NULL DEFAULT 0,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_book_version_sections_version
        FOREIGN KEY (book_version_id) REFERENCES book_versions(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT chk_book_version_sections_name_not_blank
        CHECK (BTRIM(name) <> ''),

    CONSTRAINT chk_book_version_sections_source_pages_valid
        CHECK (
            (
                source_start_page IS NULL
                AND source_end_page IS NULL
            )
            OR
            (
                source_start_page IS NOT NULL
                AND source_end_page IS NOT NULL
                AND source_start_page > 0
                AND source_end_page >= source_start_page
            )
        ),

    CONSTRAINT chk_book_version_sections_current_pages_valid
        CHECK (
            current_start_page >= 0
            AND current_end_page >= 0
            AND current_end_page >= current_start_page
        ),

    CONSTRAINT chk_book_version_sections_sort_order_non_negative
        CHECK (sort_order >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_book_version_sections_name
    ON book_version_sections(book_version_id, LOWER(name));

CREATE INDEX IF NOT EXISTS idx_book_version_sections_version_sort
    ON book_version_sections(book_version_id, sort_order, id);

CREATE INDEX IF NOT EXISTS idx_book_version_sections_current_pages
    ON book_version_sections(book_version_id, current_start_page, current_end_page);

DROP TRIGGER IF EXISTS trg_book_version_sections_set_updated_at ON book_version_sections;
CREATE TRIGGER trg_book_version_sections_set_updated_at
BEFORE UPDATE ON book_version_sections
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS book_version_fields (
    id SERIAL PRIMARY KEY,
    book_version_id INT NOT NULL,
    label VARCHAR(150) NOT NULL,
    input_type VARCHAR(30) NOT NULL,
    placement VARCHAR(30) NOT NULL,
    show_label BOOLEAN NOT NULL DEFAULT TRUE,
    is_required BOOLEAN NOT NULL DEFAULT FALSE,
    is_email_field BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_book_version_fields_version
        FOREIGN KEY (book_version_id) REFERENCES book_versions(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT chk_book_version_fields_label_not_blank
        CHECK (BTRIM(label) <> ''),

    CONSTRAINT chk_book_version_fields_input_type
        CHECK (input_type IN ('single_line', 'rich_text')),

    CONSTRAINT chk_book_version_fields_placement
        CHECK (placement IN ('heading', 'body')),

    CONSTRAINT chk_book_version_fields_sort_order_non_negative
        CHECK (sort_order >= 0)
);

CREATE INDEX IF NOT EXISTS idx_book_version_fields_version_sort
    ON book_version_fields(book_version_id, sort_order, id);

DROP TRIGGER IF EXISTS trg_book_version_fields_set_updated_at ON book_version_fields;
CREATE TRIGGER trg_book_version_fields_set_updated_at
BEFORE UPDATE ON book_version_fields
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS book_submissions (
    id SERIAL PRIMARY KEY,
    book_id INT NOT NULL,
    book_version_id INT NOT NULL,
    target_section_id INT,
    new_section_name VARCHAR(150),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    submitter_email VARCHAR(255),
    image_file_name VARCHAR(255),
    image_file_url TEXT,
    image_storage_uri TEXT,
    image_object_key TEXT,
    image_mime_type VARCHAR(255),
    image_file_size BIGINT,
    reviewed_by INT,
    reviewed_at TIMESTAMP,
    rejection_reason TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_book_submissions_book
        FOREIGN KEY (book_id) REFERENCES books(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_book_submissions_version
        FOREIGN KEY (book_version_id) REFERENCES book_versions(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_book_submissions_target_section
        FOREIGN KEY (target_section_id) REFERENCES book_version_sections(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_book_submissions_reviewed_by
        FOREIGN KEY (reviewed_by) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT chk_book_submissions_status
        CHECK (status IN ('pending', 'approved', 'rejected')),

    CONSTRAINT chk_book_submissions_target_required
        CHECK (
            target_section_id IS NOT NULL
            OR BTRIM(COALESCE(new_section_name, '')) <> ''
        ),

    CONSTRAINT chk_book_submissions_new_section_name
        CHECK (
            new_section_name IS NULL
            OR BTRIM(new_section_name) <> ''
        ),

    CONSTRAINT chk_book_submissions_image_file_size
        CHECK (image_file_size IS NULL OR image_file_size >= 0)
);

CREATE INDEX IF NOT EXISTS idx_book_submissions_book_id
    ON book_submissions(book_id);

CREATE INDEX IF NOT EXISTS idx_book_submissions_version_id
    ON book_submissions(book_version_id);

CREATE INDEX IF NOT EXISTS idx_book_submissions_status
    ON book_submissions(status);

CREATE INDEX IF NOT EXISTS idx_book_submissions_target_section_id
    ON book_submissions(target_section_id);

CREATE INDEX IF NOT EXISTS idx_book_submissions_created_at
    ON book_submissions(created_at DESC);

DROP TRIGGER IF EXISTS trg_book_submissions_set_updated_at ON book_submissions;
CREATE TRIGGER trg_book_submissions_set_updated_at
BEFORE UPDATE ON book_submissions
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS book_submission_values (
    id SERIAL PRIMARY KEY,
    book_submission_id INT NOT NULL,
    book_field_id INT NOT NULL,
    value TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_book_submission_values_submission
        FOREIGN KEY (book_submission_id) REFERENCES book_submissions(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_book_submission_values_field
        FOREIGN KEY (book_field_id) REFERENCES book_version_fields(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_book_submission_values_submission_field
    ON book_submission_values(book_submission_id, book_field_id);

CREATE INDEX IF NOT EXISTS idx_book_submission_values_submission_id
    ON book_submission_values(book_submission_id);

CREATE INDEX IF NOT EXISTS idx_book_submission_values_field_id
    ON book_submission_values(book_field_id);

DROP TRIGGER IF EXISTS trg_book_submission_values_set_updated_at ON book_submission_values;
CREATE TRIGGER trg_book_submission_values_set_updated_at
BEFORE UPDATE ON book_submission_values
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS knowledge_center_submissions (
    id SERIAL PRIMARY KEY,
    submitter_name VARCHAR(255) NOT NULL,
    submitter_email VARCHAR(255) NOT NULL,
    submitter_phone VARCHAR(80) NOT NULL DEFAULT '',
    submission_type VARCHAR(20) NOT NULL,
    message TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'open',
    completion_notes TEXT NOT NULL DEFAULT '',
    completed_by_user_id INT,
    completed_by_name VARCHAR(255) NOT NULL DEFAULT '',
    completed_by_email VARCHAR(255) NOT NULL DEFAULT '',
    completed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_knowledge_center_submissions_completed_by
        FOREIGN KEY (completed_by_user_id) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT chk_knowledge_center_submissions_name_not_blank
        CHECK (BTRIM(submitter_name) <> ''),

    CONSTRAINT chk_knowledge_center_submissions_email_not_blank
        CHECK (BTRIM(submitter_email) <> ''),

    CONSTRAINT chk_knowledge_center_submissions_submission_type
        CHECK (submission_type IN ('post', 'video', 'both')),

    CONSTRAINT chk_knowledge_center_submissions_status
        CHECK (status IN ('open', 'completed')),

    CONSTRAINT chk_knowledge_center_submissions_message_not_blank
        CHECK (BTRIM(message) <> ''),

    CONSTRAINT chk_knowledge_center_submissions_completion_state
        CHECK (
            (
                status = 'open'
                AND completed_by_user_id IS NULL
                AND completed_at IS NULL
                AND BTRIM(COALESCE(completion_notes, '')) = ''
                AND BTRIM(COALESCE(completed_by_name, '')) = ''
                AND BTRIM(COALESCE(completed_by_email, '')) = ''
            )
            OR
            (
                status = 'completed'
                AND completed_by_user_id IS NOT NULL
                AND completed_at IS NOT NULL
                AND BTRIM(COALESCE(completion_notes, '')) <> ''
                AND BTRIM(COALESCE(completed_by_name, '')) <> ''
                AND BTRIM(COALESCE(completed_by_email, '')) <> ''
            )
        )
);

CREATE INDEX IF NOT EXISTS idx_knowledge_center_submissions_status
    ON knowledge_center_submissions(status);

CREATE INDEX IF NOT EXISTS idx_knowledge_center_submissions_created_at
    ON knowledge_center_submissions(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_knowledge_center_submissions_completed_at
    ON knowledge_center_submissions(completed_at DESC);

CREATE INDEX IF NOT EXISTS idx_knowledge_center_submissions_submitter_email
    ON knowledge_center_submissions(submitter_email);

DROP TRIGGER IF EXISTS trg_knowledge_center_submissions_set_updated_at ON knowledge_center_submissions;
CREATE TRIGGER trg_knowledge_center_submissions_set_updated_at
BEFORE UPDATE ON knowledge_center_submissions
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

COMMIT;
