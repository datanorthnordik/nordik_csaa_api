package pages

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	return db, mock, func() {
		_ = sqlDB.Close()
	}
}

func TestPageServiceReturnsStoreUnavailableWithoutDB(t *testing.T) {
	service := &PageService{}

	if _, err := service.ListPages(PageListFilters{}); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ListPages to return ErrStoreUnavailable, got %v", err)
	}
	if _, err := service.GetPage(1); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected GetPage to return ErrStoreUnavailable, got %v", err)
	}
	if _, err := service.GetPageBySlug("/home"); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected GetPageBySlug to return ErrStoreUnavailable, got %v", err)
	}
	if _, err := service.GetPageHeroImageContent(1); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected GetPageHeroImageContent to return ErrStoreUnavailable, got %v", err)
	}
	if _, err := service.GetPageCTABannerImageContent(1); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected GetPageCTABannerImageContent to return ErrStoreUnavailable, got %v", err)
	}
	if _, err := service.CreatePage(validSavePageRequest()); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected CreatePage to return ErrStoreUnavailable, got %v", err)
	}
	if _, err := service.UpdatePage(1, validSavePageRequest()); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected UpdatePage to return ErrStoreUnavailable, got %v", err)
	}
	if err := service.DeletePage(1); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected DeletePage to return ErrStoreUnavailable, got %v", err)
	}
}

func TestListPagesSuccessAndValidation(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &PageService{DB: db}

	mock.ExpectQuery(`SELECT count\(\*\) FROM "pages" WHERE .*LOWER\(pages\.page_title\) LIKE \$1 OR LOWER\(pages\.url_slug\) LIKE \$2.*pages\.status = \$3`).
		WithArgs("%home%", "%home%", "published").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(12))
	mock.ExpectQuery(`SELECT .* FROM "pages" LEFT JOIN users AS modified_users ON modified_users.id = pages.modified_by LEFT JOIN pages AS parent_pages ON parent_pages.id = pages.parent_id WHERE .*LOWER\(pages\.page_title\) LIKE \$1 OR LOWER\(pages\.url_slug\) LIKE \$2.*pages\.status = \$3 ORDER BY pages\.last_modified DESC LIMIT \$4 OFFSET \$5`).
		WithArgs("%home%", "%home%", "published", 5, 5).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "parent_id", "page_type", "parent_page_title", "parent_page_url_slug", "status", "last_modified", "modified_by", "modified_by_name", "created_at", "updated_at",
		}).AddRow(
			9, "Homepage", "/about/home", 3, PageTypePage, "About", "/about", PageStatusPublished, time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC), 7, "Jane Doe", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC),
		))

	resp, err := service.ListPages(PageListFilters{
		Page:          2,
		PageSize:      5,
		SearchTerm:    "home",
		Status:        "published",
		UsePagination: true,
	})
	if err != nil {
		t.Fatalf("ListPages returned error: %v", err)
	}
	if resp.Pagination.TotalItems != 12 || len(resp.Items) != 1 {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if resp.Items[0].PageTitle != "Homepage" || resp.Items[0].ModifiedByName != "Jane Doe" {
		t.Fatalf("unexpected list item: %#v", resp.Items[0])
	}
	if resp.Items[0].ParentID == nil || *resp.Items[0].ParentID != 3 || resp.Items[0].ParentPageURLSlug != "/about" {
		t.Fatalf("expected parent page metadata, got %#v", resp.Items[0])
	}
	if resp.Items[0].PageType != PageTypePage {
		t.Fatalf("expected page type %q, got %#v", PageTypePage, resp.Items[0])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}

	if _, err := normalizeListPagesFilter(PageListFilters{Status: "bad"}); err == nil {
		t.Fatal("expected invalid status validation error")
	}
	if _, err := normalizeListPagesFilter(PageListFilters{SortBy: "drop table"}); err == nil {
		t.Fatal("expected invalid sort_by validation error")
	}
	if _, err := normalizeListPagesFilter(PageListFilters{SortOrder: "sideways"}); err == nil {
		t.Fatal("expected invalid sort_order validation error")
	}
}

func TestListPagesReturnsEmptyArrayWhenNoRowsMatch(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &PageService{DB: db}

	mock.ExpectQuery(`SELECT count\(\*\) FROM "pages"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT .* FROM "pages" LEFT JOIN users AS modified_users ON modified_users.id = pages.modified_by LEFT JOIN pages AS parent_pages ON parent_pages.id = pages.parent_id ORDER BY pages\.last_modified DESC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "parent_id", "page_type", "parent_page_title", "parent_page_url_slug", "status", "last_modified", "modified_by", "modified_by_name", "created_at", "updated_at",
		}))

	resp, err := service.ListPages(PageListFilters{})
	if err != nil {
		t.Fatalf("ListPages returned error: %v", err)
	}
	if resp.Items == nil {
		t.Fatal("expected empty items slice, got nil")
	}
	if len(resp.Items) != 0 {
		t.Fatalf("expected no items, got %#v", resp.Items)
	}
	if resp.Pagination.PageSize != 0 || resp.Pagination.TotalPages != 0 {
		t.Fatalf("expected unpaginated empty response metadata, got %#v", resp.Pagination)
	}
}

func TestGetPageSuccessAndNotFound(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &PageService{DB: db}

	mock.ExpectQuery(`SELECT .* FROM "pages" LEFT JOIN users AS created_users ON created_users.id = pages.created_by LEFT JOIN users AS modified_users ON modified_users.id = pages.modified_by LEFT JOIN pages AS parent_pages ON parent_pages.id = pages.parent_id WHERE pages.id = \$1 LIMIT \$2`).
		WithArgs(12, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "parent_id", "page_type", "parent_page_title", "parent_page_url_slug", "status", "hero_image_enabled", "hero_image_url", "hero_image_object_key",
			"seo_page_title", "seo_page_description", "created_by", "modified_by", "last_modified", "created_at", "updated_at",
			"created_by_name", "modified_by_name",
		}).AddRow(
			12, "About Us", "/about/about-us", 3, PageTypeModule, "About", "/about", PageStatusDraft, true, "gs://drive-bucket/pages/12/hero.png", "pages/12/hero.png",
			"About CSAA", "Page description", 3, 7, time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC),
			"Alex Rivera", "Jane Doe",
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "page_details" WHERE page_id = $1 ORDER BY "page_details"."id" LIMIT $2`)).
		WithArgs(12, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	resp, err := service.GetPage(12)
	if err != nil {
		t.Fatalf("GetPage returned error: %v", err)
	}
	if resp.ID != 12 || resp.HeroImageFetchURL != "/api/pages/12/hero/content" {
		t.Fatalf("unexpected detail response: %#v", resp)
	}
	if resp.ParentID == nil || *resp.ParentID != 3 || resp.ParentPageURLSlug != "/about" {
		t.Fatalf("expected parent page metadata, got %#v", resp)
	}
	if resp.PageType != PageTypeModule {
		t.Fatalf("expected module page type, got %#v", resp)
	}
	if resp.PageDetail == nil || resp.PageDetail.PageID != 12 || len(resp.PageDetail.Sections) != 0 {
		t.Fatalf("expected synthesized empty page detail, got %#v", resp.PageDetail)
	}

	mock.ExpectQuery(`SELECT .* FROM "pages" LEFT JOIN users AS created_users ON created_users.id = pages.created_by LEFT JOIN users AS modified_users ON modified_users.id = pages.modified_by LEFT JOIN pages AS parent_pages ON parent_pages.id = pages.parent_id WHERE pages.id = \$1 LIMIT \$2`).
		WithArgs(99, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	if _, err := service.GetPage(99); !errors.Is(err, ErrPageNotFound) {
		t.Fatalf("expected ErrPageNotFound, got %v", err)
	}
}

func TestGetPageBySlugReturnsPublishedPage(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &PageService{DB: db}

	mock.ExpectQuery(`SELECT .* FROM "pages" LEFT JOIN users AS created_users ON created_users.id = pages.created_by LEFT JOIN users AS modified_users ON modified_users.id = pages.modified_by LEFT JOIN pages AS parent_pages ON parent_pages.id = pages.parent_id WHERE pages.url_slug = \$1 AND pages.status = \$2 LIMIT \$3`).
		WithArgs("/about", PageStatusPublished, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "parent_id", "page_type", "parent_page_title", "parent_page_url_slug", "status", "hero_image_enabled", "hero_image_url", "hero_image_object_key",
			"seo_page_title", "seo_page_description", "created_by", "modified_by", "last_modified", "created_at", "updated_at",
			"created_by_name", "modified_by_name",
		}).AddRow(
			12, "About Us", "/about", nil, PageTypePage, "", "", PageStatusPublished, false, "", "",
			"About CSAA", "Page description", 3, 7, time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC),
			"Alex Rivera", "Jane Doe",
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "page_details" WHERE page_id = $1 ORDER BY "page_details"."id" LIMIT $2`)).
		WithArgs(12, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	resp, err := service.GetPageBySlug(" About ")
	if err != nil {
		t.Fatalf("GetPageBySlug returned error: %v", err)
	}
	if resp.URLSlug != "/about" || resp.Status != PageStatusPublished {
		t.Fatalf("unexpected detail response: %#v", resp)
	}
	if resp.PageDetail == nil || resp.PageDetail.PageID != 12 {
		t.Fatalf("expected synthesized empty page detail, got %#v", resp.PageDetail)
	}
}

func TestGetPageHeroImageContent(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &PageService{DB: db, BucketName: "drive-bucket"}
	restore := stubPageMediaHooks()
	defer restore()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id","hero_image_url","hero_image_object_key","hero_image_enabled" FROM "pages" WHERE "pages"."id" = $1 ORDER BY "pages"."id" LIMIT $2`)).
		WithArgs(12, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "hero_image_url", "hero_image_object_key", "hero_image_enabled",
		}).AddRow(
			12, "gs://drive-bucket/pages/12/hero_20260501100000_banner.png", "pages/12/hero_20260501100000_banner.png", true,
		))

	resp, err := service.GetPageHeroImageContent(12)
	if err != nil {
		t.Fatalf("GetPageHeroImageContent returned error: %v", err)
	}
	if string(resp.Content) != "downloaded:drive-bucket/pages/12/hero_20260501100000_banner.png" {
		t.Fatalf("unexpected content: %q", string(resp.Content))
	}
	if resp.FileName != "hero_20260501100000_banner.png" {
		t.Fatalf("unexpected file name: %q", resp.FileName)
	}
}

func TestGetPageCTABannerImageContent(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &PageService{DB: db, BucketName: "drive-bucket"}
	restore := stubPageMediaHooks()
	defer restore()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "page_section_cta_banner_modules" WHERE page_section_id = $1 ORDER BY "page_section_cta_banner_modules"."page_section_id" LIMIT $2`)).
		WithArgs(42, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "banner_heading", "banner_message", "button_text", "button_url", "open_in_new_tab", "image_url", "image_object_key", "created_at", "updated_at",
		}).AddRow(
			42, "Community Support", "Learn more", "Read more", "https://example.com", false, "gs://drive-bucket/pages/sections/42/cta_image_20260501100000_logo.png", "pages/sections/42/cta_image_20260501100000_logo.png", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		))

	resp, err := service.GetPageCTABannerImageContent(42)
	if err != nil {
		t.Fatalf("GetPageCTABannerImageContent returned error: %v", err)
	}
	if string(resp.Content) != "downloaded:drive-bucket/pages/sections/42/cta_image_20260501100000_logo.png" {
		t.Fatalf("unexpected content: %q", string(resp.Content))
	}
	if resp.FileName != "cta_image_20260501100000_logo.png" {
		t.Fatalf("unexpected file name: %q", resp.FileName)
	}
}

func TestJSONRawMessageUnmarshalsObjectPayloads(t *testing.T) {
	var req SavePageRequest
	payload := []byte(`{
		"page_title":"News & Media",
		"url_slug":"/news-media",
		"status":"published",
		"hero_image_enabled":false,
		"remove_hero_image":true,
		"page_detail":{
			"template_key":"default",
			"settings":{},
			"sections":[
				{
					"section_name":"Header Module",
					"section_type":"header",
					"sort_order":0,
					"is_enabled":true,
					"settings":{},
					"header":{
						"main_header_text":"News & Media",
						"sub_header_text":"",
						"hierarchy":"h1_hero"
					}
				}
			]
		}
	}`)

	if err := json.Unmarshal(payload, &req); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if req.PageDetail == nil {
		t.Fatal("expected page_detail to be present")
	}
	if string(req.PageDetail.Settings) != "{}" {
		t.Fatalf("expected page_detail.settings to be preserved as raw JSON object, got %q", string(req.PageDetail.Settings))
	}
	if len(req.PageDetail.Sections) != 1 {
		t.Fatalf("expected one section, got %#v", req.PageDetail.Sections)
	}
	if string(req.PageDetail.Sections[0].Settings) != "{}" {
		t.Fatalf("expected section settings to be preserved as raw JSON object, got %q", string(req.PageDetail.Sections[0].Settings))
	}
}

func TestApplyPageDocumentStoredFileFieldsPreservesChecksumForUnchangedReference(t *testing.T) {
	oldChecksum := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	row := PageDocument{}
	oldRow := PageDocument{
		FileURL:        "gs://bucket/page-documents/test.pdf",
		GCPObjectKey:   "page-documents/test.pdf",
		FileSize:       4096,
		ChecksumSHA256: &oldChecksum,
	}

	applyPageDocumentStoredFileFields(
		&row,
		oldRow,
		"gs://bucket/page-documents/test.pdf",
		"page-documents/test.pdf",
		0,
		"",
		true,
	)

	if row.FileURL != oldRow.FileURL || row.GCPObjectKey != oldRow.GCPObjectKey {
		t.Fatalf("expected stored object reference to remain unchanged, got %#v", row)
	}
	if row.FileSize != oldRow.FileSize {
		t.Fatalf("expected existing file size to be preserved, got %d", row.FileSize)
	}
	if row.ChecksumSHA256 == nil || *row.ChecksumSHA256 != oldChecksum {
		t.Fatalf("expected existing checksum to be preserved, got %#v", row.ChecksumSHA256)
	}
}

func TestApplyPageDocumentStoredFileFieldsClearsChecksumForChangedReferenceOnlyUpdate(t *testing.T) {
	oldChecksum := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	row := PageDocument{}
	oldRow := PageDocument{
		FileURL:        "gs://bucket/page-documents/original.pdf",
		GCPObjectKey:   "page-documents/original.pdf",
		FileSize:       4096,
		ChecksumSHA256: &oldChecksum,
	}

	applyPageDocumentStoredFileFields(
		&row,
		oldRow,
		"gs://bucket/page-documents/relinked.pdf",
		"page-documents/relinked.pdf",
		0,
		"",
		true,
	)

	if row.FileURL != "gs://bucket/page-documents/relinked.pdf" || row.GCPObjectKey != "page-documents/relinked.pdf" {
		t.Fatalf("expected updated stored object reference, got %#v", row)
	}
	if row.FileSize != 0 {
		t.Fatalf("expected changed reference without upload to reset file size, got %d", row.FileSize)
	}
	if row.ChecksumSHA256 != nil {
		t.Fatalf("expected checksum to be cleared for changed reference-only update, got %#v", row.ChecksumSHA256)
	}
}

func TestCreateUpdateAndDeletePageSuccess(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &PageService{DB: db, BucketName: "drive-bucket"}
	restore := stubPageMediaHooks()
	defer restore()

	createReq := validSavePageRequest()
	createReq.HeroImage = &PageUploadInput{
		FileName:   "banner.png",
		MimeType:   "image/png",
		DataBase64: "aGVsbG8=",
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "pages"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "pages" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	createResp, err := service.CreatePage(createReq)
	if err != nil {
		t.Fatalf("CreatePage returned error: %v", err)
	}
	if createResp.ID != 11 || createResp.Status != PageStatusDraft {
		t.Fatalf("unexpected create response: %#v", createResp)
	}
	if createResp.PageType != PageTypePage {
		t.Fatalf("expected created page type %q, got %#v", PageTypePage, createResp)
	}

	updateReq := validSavePageRequest()
	updateReq.Status = PageStatusPublished
	updateReq.HeroImage = &PageUploadInput{
		FileName:   "new-banner.png",
		MimeType:   "image/png",
		DataBase64: "aGVsbG8=",
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "pages" WHERE "pages"."id" = $1 ORDER BY "pages"."id" LIMIT $2`)).
		WithArgs(11, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "page_type", "status", "hero_image_enabled", "hero_image_url", "hero_image_object_key",
			"seo_page_title", "seo_page_description", "created_by", "modified_by", "parent_id", "last_modified", "created_at", "updated_at",
		}).AddRow(
			11, "Homepage", "/home", PageTypePage, PageStatusDraft, true, "gs://drive-bucket/pages/11/hero_20260501100000_old.png", "pages/11/hero_20260501100000_old.png",
			"Homepage SEO", "Desc", 7, 7, nil, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "pages" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updateResp, err := service.UpdatePage(11, updateReq)
	if err != nil {
		t.Fatalf("UpdatePage returned error: %v", err)
	}
	if updateResp.Status != PageStatusPublished {
		t.Fatalf("unexpected update response: %#v", updateResp)
	}
	if updateResp.PageType != PageTypePage {
		t.Fatalf("expected updated page type %q, got %#v", PageTypePage, updateResp)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "pages" WHERE "pages"."id" = $1 ORDER BY "pages"."id" LIMIT $2`)).
		WithArgs(11, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "page_type", "status", "hero_image_enabled", "hero_image_url", "hero_image_object_key",
			"seo_page_title", "seo_page_description", "created_by", "modified_by", "parent_id", "last_modified", "created_at", "updated_at",
		}).AddRow(
			11, "Homepage", "/home", PageTypePage, PageStatusPublished, true, "gs://drive-bucket/pages/11/hero_20260501100000_new-banner.png", "pages/11/hero_20260501100000_new-banner.png",
			"Homepage SEO", "Desc", 7, 7, nil, time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
		))
	mock.ExpectQuery(`SELECT DISTINCT documents\.id, documents\.file_url, COALESCE\(documents\.gcp_object_key, ''\) AS gcp_object_key FROM "page_details" JOIN page_sections ON page_sections\.page_detail_id = page_details\.id JOIN page_section_documents ON page_section_documents\.page_section_id = page_sections\.id JOIN documents ON documents\.id = page_section_documents\.document_id WHERE page_details\.page_id = \$1`).
		WithArgs(11).
		WillReturnRows(sqlmock.NewRows([]string{"id", "file_url", "gcp_object_key"}))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "pages" WHERE "pages"."id" = $1`)).
		WithArgs(11).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := service.DeletePage(11); err != nil {
		t.Fatalf("DeletePage returned error: %v", err)
	}
}

func TestUpdatePageSyncsExistingMenuItemsToNewParent(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &PageService{DB: db}
	req := validSavePageRequest()
	req.HeroImageEnabled = false
	parentID := 20
	req.ParentID = &parentID
	req.URLSlug = "/about/home"

	pageID := 11
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "pages" WHERE "pages"."id" = $1 ORDER BY "pages"."id" LIMIT $2`)).
		WithArgs(pageID, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "page_type", "status", "hero_image_enabled", "hero_image_url", "hero_image_object_key",
			"seo_page_title", "seo_page_description", "created_by", "modified_by", "parent_id", "last_modified", "created_at", "updated_at",
		}).AddRow(
			pageID, "Homepage", "/home", PageTypePage, PageStatusDraft, false, "", "",
			"Homepage SEO", "Desc", 7, 7, nil, now, now, now,
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id","url_slug" FROM "pages" WHERE "pages"."id" = $1 LIMIT $2`)).
		WithArgs(parentID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url_slug"}).AddRow(parentID, "/about"))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "pages" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT .* FROM "menu_items" WHERE page_id = \$1 ORDER BY id ASC`).
		WithArgs(pageID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "menu_id", "parent_id"}).
			AddRow(31, 5, nil).
			AddRow(32, 6, 40))
	mock.ExpectQuery(`SELECT .* FROM "menu_items" WHERE page_id = \$1 ORDER BY id ASC`).
		WithArgs(parentID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "menu_id"}).
			AddRow(21, 5))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "menu_items" SET "parent_id"=$1 WHERE id = $2`)).
		WithArgs(21, 31).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "menu_items" SET "parent_id"=NULL WHERE id = $1`)).
		WithArgs(32).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	resp, err := service.UpdatePage(pageID, req)
	if err != nil {
		t.Fatalf("UpdatePage returned error: %v", err)
	}
	if resp.ParentID == nil || *resp.ParentID != parentID {
		t.Fatalf("expected updated parent_id %d, got %#v", parentID, resp)
	}
}

func TestCreatePageWithMissingParentPageReturnsValidationError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &PageService{DB: db}
	req := validSavePageRequest()
	parentID := 25
	req.ParentID = &parentID
	req.URLSlug = "/about/team"

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id","url_slug" FROM "pages" WHERE "pages"."id" = $1 LIMIT $2`)).
		WithArgs(parentID, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectRollback()

	_, err := service.CreatePage(req)
	if err == nil || err.Error() != "parent_id references a page that does not exist" {
		t.Fatalf("expected missing parent validation error, got %v", err)
	}
}

func TestUpdatePageRejectsSelfParentPage(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &PageService{DB: db}
	req := validSavePageRequest()
	pageID := 11
	req.ParentID = &pageID
	req.URLSlug = "/home/child"

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "pages" WHERE "pages"."id" = $1 ORDER BY "pages"."id" LIMIT $2`)).
		WithArgs(pageID, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "page_type", "status", "hero_image_enabled", "hero_image_url", "hero_image_object_key",
			"seo_page_title", "seo_page_description", "created_by", "modified_by", "parent_id", "last_modified", "created_at", "updated_at",
		}).AddRow(
			pageID, "Homepage", "/home", PageTypePage, PageStatusDraft, false, "", "",
			"Homepage SEO", "Desc", 7, 7, nil, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		))
	mock.ExpectRollback()

	_, err := service.UpdatePage(pageID, req)
	if err == nil || err.Error() != "parent_id cannot reference the page itself" {
		t.Fatalf("expected self parent validation error, got %v", err)
	}
}

func TestPageValidationAndHelpers(t *testing.T) {
	req := validSavePageRequest()

	req.PageTitle = "   "
	if _, err := normalizeSavePageRequest(req); err == nil {
		t.Fatal("expected page_title validation error")
	}

	req = validSavePageRequest()
	req.URLSlug = "bad slug!"
	if _, err := normalizeSavePageRequest(req); err == nil {
		t.Fatal("expected url_slug validation error")
	}

	req = validSavePageRequest()
	req.Status = "bad"
	if _, err := normalizeSavePageRequest(req); err == nil {
		t.Fatal("expected invalid status validation error")
	}

	req = validSavePageRequest()
	parentID := 0
	req.ParentID = &parentID
	if _, err := normalizeSavePageRequest(req); err == nil {
		t.Fatal("expected parent_id validation error")
	}

	if got := normalizeURLSlug(" About/Our History "); got != "/about/our-history" {
		t.Fatalf("unexpected normalized slug: %q", got)
	}

	if err := validatePageSlugParentPrefix("/about/team", "/about"); err != nil {
		t.Fatalf("expected prefixed child slug to be accepted, got %v", err)
	}
	if err := validatePageSlugParentPrefix("/team", "/about"); err == nil {
		t.Fatal("expected parent slug prefix validation error")
	}

	service := &PageService{BucketName: "drive-bucket", BucketPrefix: "main-folder"}
	restore := stubPageMediaHooks()
	defer restore()

	url, objectKey, uploadedObject, err := service.buildHeroImageFields(5, PageUploadInput{
		FileName: "Banner.png",
		MimeType: "image/png",
		Content:  []byte("hello"),
	})
	if err != nil {
		t.Fatalf("buildHeroImageFields returned error: %v", err)
	}
	if uploadedObject != "main-folder/pages/5/hero_20260501100000_banner.png" {
		t.Fatalf("unexpected uploaded object: %q", uploadedObject)
	}
	if objectKey != "pages/5/hero_20260501100000_banner.png" {
		t.Fatalf("unexpected object key: %q", objectKey)
	}
	if url != "gs://drive-bucket/main-folder/pages/5/hero_20260501100000_banner.png" {
		t.Fatalf("unexpected hero image url: %q", url)
	}

	if (Page{}).TableName() != "pages" {
		t.Fatal("unexpected pages table name")
	}
}

func TestNormalizeSavePageSectionRequestAcceptsIconsGalleryViewMode(t *testing.T) {
	galleryID := 14

	section, err := normalizeSavePageSectionRequest(
		SavePageSectionRequest{
			SectionType: PageSectionTypeGallery,
			Gallery: &PageGallerySectionInput{
				GalleryID: &galleryID,
				ViewMode:  " Icons ",
			},
		},
		0,
	)
	if err != nil {
		t.Fatalf("expected icons gallery view mode to be accepted, got %v", err)
	}
	if section.Gallery == nil {
		t.Fatal("expected gallery section payload to be initialized")
	}
	if section.Gallery.ViewMode != PageGalleryViewIcons {
		t.Fatalf("expected normalized view mode %q, got %q", PageGalleryViewIcons, section.Gallery.ViewMode)
	}
}

func TestNormalizeSavePageSectionRequestDefaultsGalleryDisplayFlags(t *testing.T) {
	section, err := normalizeSavePageSectionRequest(
		SavePageSectionRequest{
			SectionType: PageSectionTypeGallery,
			Gallery: &PageGallerySectionInput{
				ViewMode: "carousel",
			},
		},
		0,
	)
	if err != nil {
		t.Fatalf("expected gallery defaults to be accepted, got %v", err)
	}
	if section.Gallery == nil {
		t.Fatal("expected gallery section payload to be initialized")
	}
	if section.Gallery.ShowTitleDescription == nil || !*section.Gallery.ShowTitleDescription {
		t.Fatalf("expected show_title_description to default to true, got %#v", section.Gallery.ShowTitleDescription)
	}
	if section.Gallery.AutoScrollEnabled == nil || *section.Gallery.AutoScrollEnabled {
		t.Fatalf("expected auto_scroll_enabled to default to false, got %#v", section.Gallery.AutoScrollEnabled)
	}
}

func TestPageSectionGalleryModuleCreateIncludesFalseFlags(t *testing.T) {
	db, _, cleanup := setupMockDB(t)
	defer cleanup()

	galleryID := 14
	sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Create(&PageSectionGalleryModule{
			PageSectionID:        42,
			GalleryID:            &galleryID,
			ViewMode:             PageGalleryViewCarousel,
			ShowTitleDescription: false,
			AutoScrollEnabled:    false,
		})
	})

	if !strings.Contains(sql, `"show_title_description"`) {
		t.Fatalf("expected show_title_description column in insert SQL, got %q", sql)
	}
	if !strings.Contains(sql, `"auto_scroll_enabled"`) {
		t.Fatalf("expected auto_scroll_enabled column in insert SQL, got %q", sql)
	}
}

func TestNormalizeSavePageSectionRequestNormalizesHeaderTextAlign(t *testing.T) {
	section, err := normalizeSavePageSectionRequest(
		SavePageSectionRequest{
			SectionType: PageSectionTypeHeader,
			Header: &PageHeaderSectionInput{
				MainHeaderText: "News & Media",
				TextAlign:      " Center ",
			},
		},
		0,
	)
	if err != nil {
		t.Fatalf("expected header text alignment to be accepted, got %v", err)
	}
	if section.Header == nil {
		t.Fatal("expected header payload to be initialized")
	}
	if section.Header.TextAlign != PageTextAlignCenter {
		t.Fatalf("expected normalized text alignment %q, got %q", PageTextAlignCenter, section.Header.TextAlign)
	}
	if section.Header.Hierarchy != PageHeaderHierarchyHero {
		t.Fatalf("expected default hierarchy %q, got %q", PageHeaderHierarchyHero, section.Header.Hierarchy)
	}
}

func TestNormalizeSavePageSectionRequestDefaultsHeaderTextAlignToLeft(t *testing.T) {
	section, err := normalizeSavePageSectionRequest(
		SavePageSectionRequest{
			SectionType: PageSectionTypeHeader,
			Header: &PageHeaderSectionInput{
				MainHeaderText: "News & Media",
			},
		},
		0,
	)
	if err != nil {
		t.Fatalf("expected missing header text alignment to default, got %v", err)
	}
	if section.Header == nil {
		t.Fatal("expected header payload to be initialized")
	}
	if section.Header.TextAlign != PageTextAlignLeft {
		t.Fatalf("expected default text alignment %q, got %q", PageTextAlignLeft, section.Header.TextAlign)
	}
	if section.Header.UnderlineEnabled == nil || *section.Header.UnderlineEnabled {
		t.Fatalf("expected underline_enabled to default to false, got %#v", section.Header.UnderlineEnabled)
	}
}

func TestNormalizeSavePageSectionRequestRejectsUnknownHeaderTextAlign(t *testing.T) {
	_, err := normalizeSavePageSectionRequest(
		SavePageSectionRequest{
			SectionType: PageSectionTypeHeader,
			Header: &PageHeaderSectionInput{
				MainHeaderText: "News & Media",
				TextAlign:      "justify",
			},
		},
		0,
	)
	if err == nil || err.Error() != "invalid page_detail.sections[0].header.text_align" {
		t.Fatalf("expected invalid header text alignment error, got %v", err)
	}
}

func TestNormalizeSavePageSectionRequestAllowsHeaderDescriptionAndH2Underline(t *testing.T) {
	section, err := normalizeSavePageSectionRequest(
		SavePageSectionRequest{
			SectionType: PageSectionTypeHeader,
			Header: &PageHeaderSectionInput{
				MainHeaderText:   "News & Media",
				Description:      " Latest updates ",
				Hierarchy:        PageHeaderHierarchySection,
				UnderlineEnabled: boolPtr(true),
			},
		},
		0,
	)
	if err != nil {
		t.Fatalf("expected h2 underline support to be accepted, got %v", err)
	}
	if section.Header == nil {
		t.Fatal("expected header payload to be initialized")
	}
	if section.Header.Description != "Latest updates" {
		t.Fatalf("expected trimmed description, got %#v", section.Header)
	}
	if section.Header.UnderlineEnabled == nil || !*section.Header.UnderlineEnabled {
		t.Fatalf("expected underline_enabled=true to be preserved, got %#v", section.Header.UnderlineEnabled)
	}
}

func TestNormalizeSavePageSectionRequestRejectsUnderlineOutsideH2(t *testing.T) {
	_, err := normalizeSavePageSectionRequest(
		SavePageSectionRequest{
			SectionType: PageSectionTypeHeader,
			Header: &PageHeaderSectionInput{
				MainHeaderText:   "News & Media",
				Hierarchy:        PageHeaderHierarchyHero,
				UnderlineEnabled: boolPtr(true),
			},
		},
		0,
	)
	if err == nil || err.Error() != "page_detail.sections[0].header.underline_enabled can only be true when hierarchy is h2_section" {
		t.Fatalf("expected h2-only underline validation error, got %v", err)
	}
}

func TestNormalizeSavePageDetailRequestRejectsMultipleHeroHeaders(t *testing.T) {
	_, err := normalizeSavePageDetailRequest(&SavePageDetailRequest{
		Sections: []SavePageSectionRequest{
			{
				SectionType: PageSectionTypeHeader,
				Header: &PageHeaderSectionInput{
					MainHeaderText: "Welcome",
					Hierarchy:      PageHeaderHierarchyHero,
				},
			},
			{
				SectionType: PageSectionTypeHeader,
				Header: &PageHeaderSectionInput{
					MainHeaderText: "About",
					Hierarchy:      PageHeaderHierarchyHero,
				},
			},
		},
	})
	if err == nil || err.Error() != "page_detail.sections[1].header.hierarchy only one h1_hero header is allowed per page" {
		t.Fatalf("expected duplicate h1 validation error, got %v", err)
	}
}

func TestNormalizeSavePageSectionRequestRejectsUnknownGalleryViewMode(t *testing.T) {
	_, err := normalizeSavePageSectionRequest(
		SavePageSectionRequest{
			SectionType: PageSectionTypeGallery,
			Gallery: &PageGallerySectionInput{
				ViewMode: "slideshow",
			},
		},
		0,
	)
	if err == nil || err.Error() != "invalid page_detail.sections[0].gallery.view_mode" {
		t.Fatalf("expected invalid gallery view mode error, got %v", err)
	}
}

func TestGetPageContentDetailIncludesHeaderTextAlign(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &PageService{DB: db}
	createdAt := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "page_details" WHERE page_id = $1 ORDER BY "page_details"."id" LIMIT $2`)).
		WithArgs(12, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_id", "template_key", "settings", "schema_version", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			5, 12, "default", "{}", 1, 7, 7, createdAt, updatedAt,
		))
	mock.ExpectQuery(`SELECT .* FROM "page_sections" WHERE page_detail_id = \$1 ORDER BY sort_order ASC.*id ASC`).
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_detail_id", "section_name", "section_type", "sort_order", "is_enabled", "settings", "created_at", "updated_at",
		}).AddRow(
			42, 5, "Header Module", PageSectionTypeHeader, 0, true, "{}", createdAt, updatedAt,
		))
	mock.ExpectQuery(`SELECT .* FROM "page_section_header_modules" WHERE page_section_id IN \(\$1\)`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "main_header_text", "sub_header_text", "description", "hierarchy", "text_align", "underline_enabled", "created_at", "updated_at",
		}).AddRow(
			42, "News & Media", "Latest updates", "Community stories", PageHeaderHierarchySection, PageTextAlignCenter, true, createdAt, updatedAt,
		))
	mock.ExpectQuery(`SELECT .* FROM "page_section_typography_modules" WHERE page_section_id IN \(\$1\)`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "body_html", "body_text", "text_align", "created_at", "updated_at",
		}))
	mock.ExpectQuery(`SELECT .* FROM "page_section_gallery_modules" WHERE page_section_id IN \(\$1\)`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "gallery_id", "view_mode", "created_at", "updated_at",
		}))
	mock.ExpectQuery(`SELECT .* FROM "page_section_quote_modules" WHERE page_section_id IN \(\$1\)`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "quote_content", "attribution", "created_at", "updated_at",
		}))
	mock.ExpectQuery(`SELECT .* FROM "page_section_cta_banner_modules" WHERE page_section_id IN \(\$1\)`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "banner_heading", "banner_message", "button_text", "button_url", "open_in_new_tab", "created_at", "updated_at",
		}))
	mock.ExpectQuery(`SELECT .*page_section_documents.*JOIN documents ON documents.id = page_section_documents.document_id.*page_section_documents.page_section_id IN \(\$1\).*`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "document_id", "display_name", "description", "original_file_name", "file_url", "gcp_object_key", "mime_type", "file_size", "sort_order", "created_at", "updated_at",
		}))

	resp, err := service.getPageContentDetail(12)
	if err != nil {
		t.Fatalf("getPageContentDetail returned error: %v", err)
	}
	if resp == nil || len(resp.Sections) != 1 {
		t.Fatalf("expected one section in content detail response, got %#v", resp)
	}
	if resp.Sections[0].Header == nil {
		t.Fatalf("expected header section response, got %#v", resp.Sections[0])
	}
	if resp.Sections[0].Header.TextAlign != PageTextAlignCenter {
		t.Fatalf("expected header text alignment %q, got %#v", PageTextAlignCenter, resp.Sections[0].Header)
	}
	if resp.Sections[0].Header.Description != "Community stories" {
		t.Fatalf("expected header description to round-trip, got %#v", resp.Sections[0].Header)
	}
	if !resp.Sections[0].Header.UnderlineEnabled {
		t.Fatalf("expected underline_enabled=true in response, got %#v", resp.Sections[0].Header)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetPageContentDetailIncludesGalleryDisplayFlags(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &PageService{DB: db}
	createdAt := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "page_details" WHERE page_id = $1 ORDER BY "page_details"."id" LIMIT $2`)).
		WithArgs(12, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_id", "template_key", "settings", "schema_version", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			5, 12, "default", "{}", 1, 7, 7, createdAt, updatedAt,
		))
	mock.ExpectQuery(`SELECT .* FROM "page_sections" WHERE page_detail_id = \$1 ORDER BY sort_order ASC.*id ASC`).
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_detail_id", "section_name", "section_type", "sort_order", "is_enabled", "settings", "created_at", "updated_at",
		}).AddRow(
			42, 5, "Gallery Module", PageSectionTypeGallery, 0, true, "{}", createdAt, updatedAt,
		))
	mock.ExpectQuery(`SELECT .* FROM "page_section_header_modules" WHERE page_section_id IN \(\$1\)`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "main_header_text", "sub_header_text", "description", "hierarchy", "text_align", "underline_enabled", "created_at", "updated_at",
		}))
	mock.ExpectQuery(`SELECT .* FROM "page_section_typography_modules" WHERE page_section_id IN \(\$1\)`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "body_html", "body_text", "text_align", "created_at", "updated_at",
		}))
	mock.ExpectQuery(`SELECT .* FROM "page_section_gallery_modules" WHERE page_section_id IN \(\$1\)`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "gallery_id", "view_mode", "show_title_description", "auto_scroll_enabled", "created_at", "updated_at",
		}).AddRow(
			42, 14, PageGalleryViewCarousel, false, true, createdAt, updatedAt,
		))
	mock.ExpectQuery(`SELECT .* FROM "page_section_quote_modules" WHERE page_section_id IN \(\$1\)`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "quote_content", "attribution", "created_at", "updated_at",
		}))
	mock.ExpectQuery(`SELECT .* FROM "page_section_cta_banner_modules" WHERE page_section_id IN \(\$1\)`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "banner_heading", "banner_message", "button_text", "button_url", "open_in_new_tab", "created_at", "updated_at",
		}))
	mock.ExpectQuery(`SELECT .*page_section_documents.*JOIN documents ON documents.id = page_section_documents.document_id.*page_section_documents.page_section_id IN \(\$1\).*`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "document_id", "display_name", "description", "original_file_name", "file_url", "gcp_object_key", "mime_type", "file_size", "sort_order", "created_at", "updated_at",
		}))

	resp, err := service.getPageContentDetail(12)
	if err != nil {
		t.Fatalf("getPageContentDetail returned error: %v", err)
	}
	if resp == nil || len(resp.Sections) != 1 {
		t.Fatalf("expected one section in content detail response, got %#v", resp)
	}
	if resp.Sections[0].Gallery == nil {
		t.Fatalf("expected gallery section response, got %#v", resp.Sections[0])
	}
	if resp.Sections[0].Gallery.ShowTitleDescription {
		t.Fatalf("expected show_title_description=false, got %#v", resp.Sections[0].Gallery)
	}
	if !resp.Sections[0].Gallery.AutoScrollEnabled {
		t.Fatalf("expected auto_scroll_enabled=true, got %#v", resp.Sections[0].Gallery)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetPageContentDetailIncludesCTAImageFetchURL(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &PageService{DB: db}
	createdAt := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "page_details" WHERE page_id = $1 ORDER BY "page_details"."id" LIMIT $2`)).
		WithArgs(12, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_id", "template_key", "settings", "schema_version", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			5, 12, "default", "{}", 1, 7, 7, createdAt, updatedAt,
		))
	mock.ExpectQuery(`SELECT .* FROM "page_sections" WHERE page_detail_id = \$1 ORDER BY sort_order ASC.*id ASC`).
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_detail_id", "section_name", "section_type", "sort_order", "is_enabled", "settings", "created_at", "updated_at",
		}).AddRow(
			42, 5, "CTA Banner", PageSectionTypeCTABanner, 0, true, "{}", createdAt, updatedAt,
		))
	mock.ExpectQuery(`SELECT .* FROM "page_section_header_modules" WHERE page_section_id IN \(\$1\)`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "main_header_text", "sub_header_text", "description", "hierarchy", "text_align", "underline_enabled", "created_at", "updated_at",
		}))
	mock.ExpectQuery(`SELECT .* FROM "page_section_typography_modules" WHERE page_section_id IN \(\$1\)`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "body_html", "body_text", "text_align", "created_at", "updated_at",
		}))
	mock.ExpectQuery(`SELECT .* FROM "page_section_gallery_modules" WHERE page_section_id IN \(\$1\)`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "gallery_id", "view_mode", "show_title_description", "auto_scroll_enabled", "created_at", "updated_at",
		}))
	mock.ExpectQuery(`SELECT .* FROM "page_section_quote_modules" WHERE page_section_id IN \(\$1\)`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "quote_content", "attribution", "created_at", "updated_at",
		}))
	mock.ExpectQuery(`SELECT .* FROM "page_section_cta_banner_modules" WHERE page_section_id IN \(\$1\)`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "banner_heading", "banner_message", "button_text", "button_url", "open_in_new_tab", "image_url", "image_object_key", "created_at", "updated_at",
		}).AddRow(
			42, "Community Support", "We are here for the community.", "Learn more", "https://example.com", true, "gs://drive-bucket/pages/sections/42/cta_image_20260501100000_logo.png", "pages/sections/42/cta_image_20260501100000_logo.png", createdAt, updatedAt,
		))
	mock.ExpectQuery(`SELECT .*page_section_documents.*JOIN documents ON documents.id = page_section_documents.document_id.*page_section_documents.page_section_id IN \(\$1\).*`).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{
			"page_section_id", "document_id", "display_name", "description", "original_file_name", "file_url", "gcp_object_key", "mime_type", "file_size", "sort_order", "created_at", "updated_at",
		}))

	resp, err := service.getPageContentDetail(12)
	if err != nil {
		t.Fatalf("getPageContentDetail returned error: %v", err)
	}
	if resp == nil || len(resp.Sections) != 1 {
		t.Fatalf("expected one section in content detail response, got %#v", resp)
	}
	if resp.Sections[0].CTABanner == nil || resp.Sections[0].CTABanner.Image == nil {
		t.Fatalf("expected CTA image response, got %#v", resp.Sections[0].CTABanner)
	}
	if resp.Sections[0].CTABanner.Image.FetchURL != "/api/pages/sections/42/cta-image/content" {
		t.Fatalf("unexpected CTA image fetch url: %#v", resp.Sections[0].CTABanner.Image)
	}
	if resp.Sections[0].CTABanner.Image.StorageURI != "gs://drive-bucket/pages/sections/42/cta_image_20260501100000_logo.png" {
		t.Fatalf("unexpected CTA image storage uri: %#v", resp.Sections[0].CTABanner.Image)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetPageDocumentContent(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &PageService{DB: db, BucketName: "drive-bucket"}
	restore := stubPageMediaHooks()
	defer restore()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "documents" WHERE "documents"."id" = $1 ORDER BY "documents"."id" LIMIT $2`)).
		WithArgs(17, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "display_name", "description", "original_file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "checksum_sha256", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			17, "Board Agenda", "Meeting agenda", "agenda.pdf", "page-documents/agenda.pdf", "gs://drive-bucket/page-documents/agenda.pdf", "application/pdf", 1024, "", 7, 7, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		))

	resp, err := service.GetPageDocumentContent(17)
	if err != nil {
		t.Fatalf("GetPageDocumentContent returned error: %v", err)
	}
	if string(resp.Content) != "downloaded:drive-bucket/page-documents/agenda.pdf" {
		t.Fatalf("unexpected content: %q", string(resp.Content))
	}
	if resp.FileName != "agenda.pdf" {
		t.Fatalf("unexpected file name: %q", resp.FileName)
	}
}

func TestModulePagesCannotBeUpdatedOrDeleted(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &PageService{DB: db}
	req := validSavePageRequest()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "pages" WHERE "pages"."id" = $1 ORDER BY "pages"."id" LIMIT $2`)).
		WithArgs(22, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "page_type", "status", "hero_image_enabled", "hero_image_url", "hero_image_object_key",
			"seo_page_title", "seo_page_description", "created_by", "modified_by", "parent_id", "last_modified", "created_at", "updated_at",
		}).AddRow(
			22, "Events", "/events", PageTypeModule, PageStatusPublished, false, "", "",
			"Events", "Events landing page", 7, 7, nil, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		))
	mock.ExpectRollback()

	if _, err := service.UpdatePage(22, req); !errors.Is(err, ErrPageModuleManaged) {
		t.Fatalf("expected ErrPageModuleManaged from UpdatePage, got %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "pages" WHERE "pages"."id" = $1 ORDER BY "pages"."id" LIMIT $2`)).
		WithArgs(22, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "page_type", "status", "hero_image_enabled", "hero_image_url", "hero_image_object_key",
			"seo_page_title", "seo_page_description", "created_by", "modified_by", "parent_id", "last_modified", "created_at", "updated_at",
		}).AddRow(
			22, "Events", "/events", PageTypeModule, PageStatusPublished, false, "", "",
			"Events", "Events landing page", 7, 7, nil, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		))
	mock.ExpectRollback()

	if err := service.DeletePage(22); !errors.Is(err, ErrPageModuleManaged) {
		t.Fatalf("expected ErrPageModuleManaged from DeletePage, got %v", err)
	}
}

func validSavePageRequest() SavePageRequest {
	userID := 7
	return SavePageRequest{
		PageTitle:          "Homepage",
		URLSlug:            "/home",
		Status:             PageStatusDraft,
		HeroImageEnabled:   true,
		SEOPageTitle:       "Homepage SEO",
		SEOPageDescription: "Description",
		CreatedBy:          &userID,
		ModifiedBy:         &userID,
	}
}

func stubPageMediaHooks() func() {
	prevUpload := uploadBase64ToGCSHook
	prevUploadBytes := uploadBytesToGCSHook
	prevDownload := downloadGCSObjectHook
	prevDelete := deleteGCSObjectHook
	prevNow := pagesNowFunc

	uploadBase64ToGCSHook = func(base64Data, bucketName, objectName, contentType string) (string, int64, error) {
		return "gs://" + bucketName + "/" + objectName, int64(len(base64Data)), nil
	}
	uploadBytesToGCSHook = func(data []byte, bucketName, objectName, contentType string) (string, int64, error) {
		return "gs://" + bucketName + "/" + objectName, int64(len(data)), nil
	}
	downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
		return []byte("downloaded:" + bucketName + "/" + objectName), "image/png", nil
	}
	deleteGCSObjectHook = func(bucketName, objectName string) error {
		return nil
	}
	pagesNowFunc = func() time.Time {
		return time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	}

	return func() {
		uploadBase64ToGCSHook = prevUpload
		uploadBytesToGCSHook = prevUploadBytes
		downloadGCSObjectHook = prevDownload
		deleteGCSObjectHook = prevDelete
		pagesNowFunc = prevNow
	}
}
