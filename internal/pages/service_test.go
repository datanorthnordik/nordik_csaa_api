package pages

import (
	"errors"
	"regexp"
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
	if _, err := service.GetPageHeroImageContent(1); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected GetPageHeroImageContent to return ErrStoreUnavailable, got %v", err)
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

	mock.ExpectQuery(`SELECT count\(\*\) FROM "pages" WHERE .*LOWER\(page_title\) LIKE \$1 OR LOWER\(url_slug\) LIKE \$2.*status = \$3`).
		WithArgs("%home%", "%home%", "published").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(12))
	mock.ExpectQuery(`SELECT .* FROM "pages" LEFT JOIN users AS modified_users ON modified_users.id = pages.modified_by WHERE .*LOWER\(page_title\) LIKE \$1 OR LOWER\(url_slug\) LIKE \$2.*status = \$3 ORDER BY last_modified DESC LIMIT \$4 OFFSET \$5`).
		WithArgs("%home%", "%home%", "published", 5, 5).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "status", "last_modified", "modified_by", "modified_by_name", "created_at", "updated_at",
		}).AddRow(
			9, "Homepage", "/home", PageStatusPublished, time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC), 7, "Jane Doe", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC),
		))

	resp, err := service.ListPages(PageListFilters{
		Page:       2,
		PageSize:   5,
		SearchTerm: "home",
		Status:     "published",
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
	mock.ExpectQuery(`SELECT .* FROM "pages" LEFT JOIN users AS modified_users ON modified_users.id = pages.modified_by ORDER BY last_modified DESC LIMIT \$1`).
		WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "status", "last_modified", "modified_by", "modified_by_name", "created_at", "updated_at",
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
}

func TestGetPageSuccessAndNotFound(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &PageService{DB: db}

	mock.ExpectQuery(`SELECT .* FROM "pages" LEFT JOIN users AS created_users ON created_users.id = pages.created_by LEFT JOIN users AS modified_users ON modified_users.id = pages.modified_by WHERE pages.id = \$1 LIMIT \$2`).
		WithArgs(12, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "status", "hero_image_enabled", "hero_image_url", "hero_image_object_key",
			"seo_page_title", "seo_page_description", "created_by", "modified_by", "last_modified", "created_at", "updated_at",
			"created_by_name", "modified_by_name",
		}).AddRow(
			12, "About Us", "/about-us", PageStatusDraft, true, "gs://drive-bucket/pages/12/hero.png", "pages/12/hero.png",
			"About CSAA", "Page description", 3, 7, time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC),
			"Alex Rivera", "Jane Doe",
		))

	resp, err := service.GetPage(12)
	if err != nil {
		t.Fatalf("GetPage returned error: %v", err)
	}
	if resp.ID != 12 || resp.HeroImageFetchURL != "/api/pages/12/hero/content" {
		t.Fatalf("unexpected detail response: %#v", resp)
	}

	mock.ExpectQuery(`SELECT .* FROM "pages" LEFT JOIN users AS created_users ON created_users.id = pages.created_by LEFT JOIN users AS modified_users ON modified_users.id = pages.modified_by WHERE pages.id = \$1 LIMIT \$2`).
		WithArgs(99, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	if _, err := service.GetPage(99); !errors.Is(err, ErrPageNotFound) {
		t.Fatalf("expected ErrPageNotFound, got %v", err)
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
			"id", "page_title", "url_slug", "status", "hero_image_enabled", "hero_image_url", "hero_image_object_key",
			"seo_page_title", "seo_page_description", "created_by", "modified_by", "last_modified", "created_at", "updated_at",
		}).AddRow(
			11, "Homepage", "/home", PageStatusDraft, true, "gs://drive-bucket/pages/11/hero_20260501100000_old.png", "pages/11/hero_20260501100000_old.png",
			"Homepage SEO", "Desc", 7, 7, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
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

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "pages" WHERE "pages"."id" = $1 ORDER BY "pages"."id" LIMIT $2`)).
		WithArgs(11, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "status", "hero_image_enabled", "hero_image_url", "hero_image_object_key",
			"seo_page_title", "seo_page_description", "created_by", "modified_by", "last_modified", "created_at", "updated_at",
		}).AddRow(
			11, "Homepage", "/home", PageStatusPublished, true, "gs://drive-bucket/pages/11/hero_20260501100000_new-banner.png", "pages/11/hero_20260501100000_new-banner.png",
			"Homepage SEO", "Desc", 7, 7, time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
		))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "pages" WHERE "pages"."id" = $1`)).
		WithArgs(11).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := service.DeletePage(11); err != nil {
		t.Fatalf("DeletePage returned error: %v", err)
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

	if got := normalizeURLSlug(" About/Our History "); got != "/about/our-history" {
		t.Fatalf("unexpected normalized slug: %q", got)
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
