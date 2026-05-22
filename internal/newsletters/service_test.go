package newsletters

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"nordikcsaaapi/internal/util"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupMockNewsletterDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
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

	return db, mock, func() { _ = sqlDB.Close() }
}

type newsletterHookRecorder struct {
	uploads   []string
	downloads []string
	deletes   []string
}

func stubNewsletterHooks(uploadErr, downloadErr, deleteErr error) (*newsletterHookRecorder, func()) {
	recorder := &newsletterHookRecorder{}

	prevUpload := uploadBytesToGCSHook
	prevDownload := downloadGCSObjectHook
	prevDelete := deleteGCSObjectHook
	prevNow := newsletterNowFunc

	uploadBytesToGCSHook = func(data []byte, bucketName, objectName, contentType string) (string, int64, error) {
		if uploadErr != nil {
			return "", 0, uploadErr
		}
		recorder.uploads = append(recorder.uploads, bucketName+"/"+objectName)
		return "gs://" + bucketName + "/" + objectName, int64(len(data)), nil
	}
	downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
		if downloadErr != nil {
			return nil, "", downloadErr
		}
		recorder.downloads = append(recorder.downloads, bucketName+"/"+objectName)
		contentType := "application/pdf"
		if strings.HasSuffix(strings.ToLower(objectName), ".png") {
			contentType = "image/png"
		}
		return []byte("downloaded:" + bucketName + "/" + objectName), contentType, nil
	}
	deleteGCSObjectHook = func(bucketName, objectName string) error {
		recorder.deletes = append(recorder.deletes, bucketName+"/"+objectName)
		return deleteErr
	}
	newsletterNowFunc = func() time.Time {
		return time.Date(2026, 5, 19, 11, 30, 0, 123, time.UTC)
	}

	return recorder, func() {
		uploadBytesToGCSHook = prevUpload
		downloadGCSObjectHook = prevDownload
		deleteGCSObjectHook = prevDelete
		newsletterNowFunc = prevNow
	}
}

func TestNewsletterServiceStoreUnavailable(t *testing.T) {
	svc := &NewsletterService{}
	req := validSaveNewsletterEntryRequest()

	if _, err := svc.ListNewsletterEntries(ListNewsletterFilter{}); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ListNewsletterEntries ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.GetNewsletterEntry(1); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected GetNewsletterEntry ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.GetNewsletterMediaContent(1, 2); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected GetNewsletterMediaContent ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.CreateNewsletterEntry(req, intPtr(7)); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected CreateNewsletterEntry ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.UpdateNewsletterEntry(1, req, intPtr(7)); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected UpdateNewsletterEntry ErrStoreUnavailable, got %v", err)
	}
	if err := svc.DeleteNewsletterEntry(1); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected DeleteNewsletterEntry ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.AddNewsletterMedia(1, AddNewsletterMediaRequest{}, intPtr(7)); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected AddNewsletterMedia ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.UpdateNewsletterMedia(1, 2, UpdateNewsletterMediaRequest{}); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected UpdateNewsletterMedia ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.ReorderNewsletterMedia(1, []int{1}); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ReorderNewsletterMedia ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.DeleteNewsletterMedia(1, []int{1}); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected DeleteNewsletterMedia ErrStoreUnavailable, got %v", err)
	}
}

func TestListNewsletterEntriesSuccessAndValidation(t *testing.T) {
	db, mock, cleanup := setupMockNewsletterDB(t)
	defer cleanup()

	svc := &NewsletterService{DB: db}

	mock.ExpectQuery(`SELECT count\(\*\) FROM "newsletter_entries"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(6))
	mock.ExpectQuery(`SELECT \* FROM "newsletter_entries"`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "<p>Hello</p>", "published", "public",
			nil, 7, 7, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
		))
	mock.ExpectQuery(`SELECT \* FROM "newsletter_media" WHERE newsletter_entry_id IN \(\$1\) ORDER BY newsletter_entry_id ASC,sort_order ASC,id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 9, "Agenda", "agenda.pdf", "news-letters/documents/agenda.pdf", "gs://drive-bucket/news-letters/documents/agenda.pdf", "application/pdf", 1024, "attachment", 0, 7, 7, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		))

	resp, err := svc.ListNewsletterEntries(ListNewsletterFilter{
		Status:     "published",
		Visibility: "public",
		SearchTerm: "spring",
		SortBy:     "title",
		SortOrder:  "asc",
		Page:       2,
		PageSize:   5,
	})
	if err != nil {
		t.Fatalf("ListNewsletterEntries returned error: %v", err)
	}
	if resp.Total != 6 || len(resp.Items) != 1 || resp.Page != 2 || resp.PageSize != 5 || resp.TotalPages != 2 {
		t.Fatalf("unexpected list response: %#v", resp)
	}
	if resp.Items[0].Title != "Spring Update" || resp.Items[0].Status != "published" || resp.Items[0].Category != "csaa" {
		t.Fatalf("unexpected summary item: %#v", resp.Items[0])
	}
	if resp.Items[0].ContentHTML != "<p>Hello</p>" {
		t.Fatalf("expected list content_html, got %#v", resp.Items[0])
	}
	if len(resp.Items[0].Media) != 1 || resp.Items[0].Media[0].DisplayName != "Agenda" {
		t.Fatalf("expected list media, got %#v", resp.Items[0].Media)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}

	if _, err := svc.ListNewsletterEntries(ListNewsletterFilter{Status: "bad"}); err == nil {
		t.Fatal("expected invalid status error")
	}
	if _, err := svc.ListNewsletterEntries(ListNewsletterFilter{Visibility: "bad"}); err == nil {
		t.Fatal("expected invalid visibility error")
	}
}

func TestGetNewsletterEntrySuccessAndNotFound(t *testing.T) {
	db, mock, cleanup := setupMockNewsletterDB(t)
	defer cleanup()

	svc := &NewsletterService{DB: db}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Update", "cst", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "<p>Hello</p>", "published", "public",
			time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC), 7, 8, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		))
	mock.ExpectQuery(`SELECT \* FROM "newsletter_media" WHERE newsletter_entry_id = \$1 ORDER BY sort_order ASC, id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 9, "Agenda", "agenda.pdf", "news-letters/documents/agenda.pdf", "gs://drive-bucket/news-letters/documents/agenda.pdf", "application/pdf", 1024, "attachment", 0, 7, 7, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		))

	resp, err := svc.GetNewsletterEntry(9)
	if err != nil {
		t.Fatalf("GetNewsletterEntry returned error: %v", err)
	}
	if resp.ID != 9 || resp.Category != "cst" || len(resp.Media) != 1 || resp.Media[0].DisplayName != "Agenda" {
		t.Fatalf("unexpected detail response: %#v", resp)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnError(gorm.ErrRecordNotFound)

	if _, err := svc.GetNewsletterEntry(99); !errors.Is(err, ErrNewsletterEntryNotFound) {
		t.Fatalf("expected ErrNewsletterEntryNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetNewsletterMediaContentAndObjectResolution(t *testing.T) {
	db, mock, cleanup := setupMockNewsletterDB(t)
	defer cleanup()

	svc := &NewsletterService{DB: db, BucketName: "drive-bucket"}
	recorder, restore := stubNewsletterHooks(nil, nil, nil)
	defer restore()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "", "published", "public", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_media" WHERE id = $1 AND newsletter_entry_id = $2 ORDER BY "newsletter_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 9, "Agenda", "agenda.pdf", "", "gs://other-bucket/folder/agenda.pdf", "application/pdf", 1024, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))

	resp, err := svc.GetNewsletterMediaContent(9, 4)
	if err != nil {
		t.Fatalf("GetNewsletterMediaContent returned error: %v", err)
	}
	if string(resp.Content) != "downloaded:other-bucket/folder/agenda.pdf" {
		t.Fatalf("unexpected content: %q", string(resp.Content))
	}
	if resp.FileName != "agenda.pdf" {
		t.Fatalf("unexpected file name: %q", resp.FileName)
	}
	if len(recorder.downloads) != 1 || recorder.downloads[0] != "other-bucket/folder/agenda.pdf" {
		t.Fatalf("unexpected downloads: %#v", recorder.downloads)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "", "published", "public", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_media" WHERE id = $1 AND newsletter_entry_id = $2 ORDER BY "newsletter_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			5, 9, "External", "external.pdf", "", "https://example.com/files/external.pdf", "application/pdf", 0, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))

	if _, err := svc.GetNewsletterMediaContent(9, 5); err == nil || err.Error() != "media content is not available from storage" {
		t.Fatalf("expected unavailable storage error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}

	_, restore = stubNewsletterHooks(nil, util.ErrObjectNotFound, nil)
	defer restore()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "", "published", "public", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_media" WHERE id = $1 AND newsletter_entry_id = $2 ORDER BY "newsletter_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			6, 9, "Agenda", "agenda.pdf", "folder/agenda.pdf", "", "application/pdf", 0, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))

	if _, err := svc.GetNewsletterMediaContent(9, 6); !errors.Is(err, ErrNewsletterMediaNotFound) {
		t.Fatalf("expected ErrNewsletterMediaNotFound, got %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "", "published", "public", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_media" WHERE id = $1 AND newsletter_entry_id = $2 ORDER BY "newsletter_media"."id" LIMIT $3`)).
		WillReturnError(gorm.ErrRecordNotFound)
	if _, err := svc.GetNewsletterMediaContent(9, 99); !errors.Is(err, ErrNewsletterMediaNotFound) {
		t.Fatalf("expected ErrNewsletterMediaNotFound for missing media row, got %v", err)
	}

	db, mock, cleanup = setupMockNewsletterDB(t)
	defer cleanup()
	svc = &NewsletterService{DB: db}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "", "published", "public", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_media" WHERE id = $1 AND newsletter_entry_id = $2 ORDER BY "newsletter_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			7, 9, "Agenda", "agenda.pdf", "folder/agenda.pdf", "", "application/pdf", 0, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))
	if _, err := svc.GetNewsletterMediaContent(9, 7); !errors.Is(err, ErrMediaBucketNotConfigured) {
		t.Fatalf("expected ErrMediaBucketNotConfigured, got %v", err)
	}
}

func TestGetNewsletterMediaContentFallbackContentTypesAndDownloadErrors(t *testing.T) {
	prevDownload := downloadGCSObjectHook
	defer func() {
		downloadGCSObjectHook = prevDownload
	}()

	db, mock, cleanup := setupMockNewsletterDB(t)
	defer cleanup()

	svc := &NewsletterService{DB: db, BucketName: "drive-bucket"}
	downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
		return []byte("pdf-bytes"), "", nil
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "", "published", "public", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_media" WHERE id = $1 AND newsletter_entry_id = $2 ORDER BY "newsletter_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 9, "Agenda", "agenda.pdf", "folder/agenda.pdf", "", "application/pdf", 1024, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))

	resp, err := svc.GetNewsletterMediaContent(9, 4)
	if err != nil {
		t.Fatalf("GetNewsletterMediaContent returned error: %v", err)
	}
	if resp.ContentType != "application/pdf" {
		t.Fatalf("expected media mime fallback, got %#v", resp)
	}

	db, mock, cleanup = setupMockNewsletterDB(t)
	defer cleanup()
	svc = &NewsletterService{DB: db, BucketName: "drive-bucket"}
	downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
		return nil, "", nil
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "", "published", "public", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_media" WHERE id = $1 AND newsletter_entry_id = $2 ORDER BY "newsletter_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			5, 9, "Poster", "poster.bin", "folder/poster.bin", "", "", 128, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))

	resp, err = svc.GetNewsletterMediaContent(9, 5)
	if err != nil {
		t.Fatalf("GetNewsletterMediaContent returned error: %v", err)
	}
	if resp.ContentType != "application/octet-stream" {
		t.Fatalf("expected octet-stream fallback, got %#v", resp)
	}

	db, mock, cleanup = setupMockNewsletterDB(t)
	defer cleanup()
	svc = &NewsletterService{DB: db, BucketName: "drive-bucket"}
	downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
		return nil, "", errors.New("download failed")
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "", "published", "public", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_media" WHERE id = $1 AND newsletter_entry_id = $2 ORDER BY "newsletter_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			6, 9, "Agenda", "agenda.pdf", "folder/agenda.pdf", "", "application/pdf", 1024, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))

	if _, err := svc.GetNewsletterMediaContent(9, 6); err == nil || err.Error() != "download failed" {
		t.Fatalf("expected download failure to be returned, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCreateAndUpdateNewsletterEntry(t *testing.T) {
	db, mock, cleanup := setupMockNewsletterDB(t)
	defer cleanup()

	svc := &NewsletterService{DB: db}

	createReq := validSaveNewsletterEntryRequest()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "newsletter_entries"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectCommit()

	createResp, err := svc.CreateNewsletterEntry(createReq, intPtr(7))
	if err != nil {
		t.Fatalf("CreateNewsletterEntry returned error: %v", err)
	}
	if createResp.ID != 11 || createResp.Title != "Spring Update" || createResp.Category != "csaa" {
		t.Fatalf("unexpected create response: %#v", createResp)
	}

	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "<p>Hello</p>", "draft", "private",
			nil, 7, 7, now, now,
		))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "newsletter_entries" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updateReq := validSaveNewsletterEntryRequest()
	updateReq.Title = "Spring Update Revised"
	updateReq.Category = "cst"
	updateReq.Status = "published"
	updateReq.Visibility = "public"
	updateResp, err := svc.UpdateNewsletterEntry(11, updateReq, intPtr(8))
	if err != nil {
		t.Fatalf("UpdateNewsletterEntry returned error: %v", err)
	}
	if updateResp.Title != "Spring Update Revised" || updateResp.Status != "published" || updateResp.Category != "cst" {
		t.Fatalf("unexpected update response: %#v", updateResp)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestDeleteNewsletterEntryCleansUpStoredObjects(t *testing.T) {
	db, mock, cleanup := setupMockNewsletterDB(t)
	defer cleanup()

	svc := &NewsletterService{DB: db, BucketName: "drive-bucket"}
	recorder, restore := stubNewsletterHooks(nil, nil, nil)
	defer restore()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "", "published", "public",
			nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(`SELECT \* FROM "newsletter_media" WHERE newsletter_entry_id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			3, 11, "Agenda", "agenda.pdf", "", "gs://drive-bucket/news-letters/documents/agenda.pdf", "application/pdf", 100, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "newsletter_media" WHERE newsletter_entry_id = \$1`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM "newsletter_entries" WHERE "newsletter_entries"."id" = \$1`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := svc.DeleteNewsletterEntry(11); err != nil {
		t.Fatalf("DeleteNewsletterEntry returned error: %v", err)
	}
	if len(recorder.deletes) != 1 || recorder.deletes[0] != "drive-bucket/news-letters/documents/agenda.pdf" {
		t.Fatalf("expected media cleanup, got %#v", recorder.deletes)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestAddNewsletterMedia(t *testing.T) {
	db, mock, cleanup := setupMockNewsletterDB(t)
	defer cleanup()

	svc := &NewsletterService{DB: db, BucketName: "drive-bucket", BucketPrefix: "main-folder"}
	recorder, restore := stubNewsletterHooks(nil, nil, nil)
	defer restore()

	if _, err := svc.AddNewsletterMedia(1, AddNewsletterMediaRequest{}, intPtr(7)); err == nil {
		t.Fatal("expected media validation error")
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "", "published", "public", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT MAX\(sort_order\) FROM "newsletter_media" WHERE newsletter_entry_id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(nil))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "newsletter_media"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(21))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "newsletter_media"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(22))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "newsletter_entries" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	resp, err := svc.AddNewsletterMedia(11, AddNewsletterMediaRequest{
		Media: []NewsletterUploadInput{
			{DisplayName: "Agenda", FileName: "agenda.pdf", MimeType: "application/pdf", Content: []byte("agenda")},
			{DisplayName: "Minutes", FileURL: "gs://drive-bucket/main-folder/news-letters/documents/minutes.pdf", MimeType: "application/pdf"},
		},
	}, intPtr(7))
	if err != nil {
		t.Fatalf("AddNewsletterMedia returned error: %v", err)
	}
	if resp.UploadedCount != 1 {
		t.Fatalf("expected uploaded count 1, got %#v", resp)
	}
	if len(recorder.uploads) != 1 || !strings.Contains(recorder.uploads[0], "main-folder/news-letters/documents/") {
		t.Fatalf("unexpected uploads: %#v", recorder.uploads)
	}

	media, uploadedKey, err := svc.buildNewsletterMediaModel(11, NewsletterUploadInput{
		DisplayName: "External",
		FileURL:     "gs://drive-bucket/main-folder/news-letters/documents/external.pdf",
		MimeType:    "application/pdf",
	}, intPtr(7), 0)
	if err != nil {
		t.Fatalf("buildNewsletterMediaModel returned error: %v", err)
	}
	if uploadedKey != "" || media.GCPObjectKey != "main-folder/news-letters/documents/external.pdf" {
		t.Fatalf("unexpected direct media model: %#v uploaded=%q", media, uploadedKey)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}

	db, mock, cleanup = setupMockNewsletterDB(t)
	defer cleanup()
	svc = &NewsletterService{DB: db, BucketName: "drive-bucket"}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnError(gorm.ErrRecordNotFound)
	if _, err := svc.AddNewsletterMedia(99, AddNewsletterMediaRequest{Media: []NewsletterUploadInput{{FileURL: "gs://drive-bucket/a.pdf"}}}, intPtr(7)); !errors.Is(err, ErrNewsletterEntryNotFound) {
		t.Fatalf("expected ErrNewsletterEntryNotFound, got %v", err)
	}

	db, mock, cleanup = setupMockNewsletterDB(t)
	defer cleanup()
	svc = &NewsletterService{DB: db, BucketName: "drive-bucket"}
	recorder, restore = stubNewsletterHooks(nil, nil, nil)
	defer restore()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "", "published", "public", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT MAX\(sort_order\) FROM "newsletter_media" WHERE newsletter_entry_id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(nil))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "newsletter_media"`)).
		WillReturnError(errors.New("insert failed"))
	mock.ExpectRollback()

	if _, err := svc.AddNewsletterMedia(11, AddNewsletterMediaRequest{
		Media: []NewsletterUploadInput{{FileName: "agenda.pdf", MimeType: "application/pdf", Content: []byte("agenda")}},
	}, intPtr(7)); err == nil {
		t.Fatal("expected add media insert error")
	}
	if len(recorder.deletes) != 1 || !strings.Contains(recorder.deletes[0], "drive-bucket/news-letters/documents/") {
		t.Fatalf("expected uploaded media cleanup after insert failure, got %#v", recorder.deletes)
	}
}

func TestUpdateNewsletterMedia(t *testing.T) {
	db, mock, cleanup := setupMockNewsletterDB(t)
	defer cleanup()

	svc := &NewsletterService{DB: db}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_media" WHERE id = $1 AND newsletter_entry_id = $2 ORDER BY "newsletter_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "news-letters/documents/agenda.pdf", "gs://drive-bucket/news-letters/documents/agenda.pdf", "application/pdf", 100, "attachment", 0, 7, 7, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "newsletter_media" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "newsletter_entries" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	resp, err := svc.UpdateNewsletterMedia(11, 4, UpdateNewsletterMediaRequest{DisplayName: "Agenda v2", FileName: "agenda-v2.pdf"})
	if err != nil {
		t.Fatalf("UpdateNewsletterMedia returned error: %v", err)
	}
	if resp.DisplayName != "Agenda v2" || resp.FileName != "agenda-v2.pdf" {
		t.Fatalf("unexpected media response: %#v", resp)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_media" WHERE id = $1 AND newsletter_entry_id = $2 ORDER BY "newsletter_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "news-letters/documents/agenda.pdf", "gs://drive-bucket/news-letters/documents/agenda.pdf", "application/pdf", 100, "attachment", 0, 7, 7, time.Now(), time.Now(),
		))
	if _, err := svc.UpdateNewsletterMedia(11, 4, UpdateNewsletterMediaRequest{DisplayName: " ", FileName: " "}); err == nil {
		t.Fatal("expected update media validation error")
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_media" WHERE id = $1 AND newsletter_entry_id = $2 ORDER BY "newsletter_media"."id" LIMIT $3`)).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnError(gorm.ErrRecordNotFound)
	if _, err := svc.UpdateNewsletterMedia(11, 99, UpdateNewsletterMediaRequest{DisplayName: "Missing"}); !errors.Is(err, ErrNewsletterEntryNotFound) {
		t.Fatalf("expected ErrNewsletterEntryNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestUpdateNewsletterMediaAdditionalBranches(t *testing.T) {
	db, mock, cleanup := setupMockNewsletterDB(t)
	defer cleanup()

	svc := &NewsletterService{DB: db}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_media" WHERE id = $1 AND newsletter_entry_id = $2 ORDER BY "newsletter_media"."id" LIMIT $3`)).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "", "published", "public", nil, nil, nil, time.Now(), time.Now(),
		))

	if _, err := svc.UpdateNewsletterMedia(11, 99, UpdateNewsletterMediaRequest{DisplayName: "Missing"}); !errors.Is(err, ErrNewsletterMediaNotFound) {
		t.Fatalf("expected ErrNewsletterMediaNotFound, got %v", err)
	}

	db, mock, cleanup = setupMockNewsletterDB(t)
	defer cleanup()
	svc = &NewsletterService{DB: db}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_media" WHERE id = $1 AND newsletter_entry_id = $2 ORDER BY "newsletter_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "news-letters/documents/agenda.pdf", "gs://drive-bucket/news-letters/documents/agenda.pdf", "application/pdf", 100, "attachment", 0, 7, 7, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "newsletter_media" SET`)).
		WillReturnError(errors.New("save failed"))
	mock.ExpectRollback()

	if _, err := svc.UpdateNewsletterMedia(11, 4, UpdateNewsletterMediaRequest{DisplayName: "Agenda v2"}); err == nil || err.Error() != "save failed" {
		t.Fatalf("expected save failure, got %v", err)
	}
}

func TestReorderAndDeleteNewsletterMedia(t *testing.T) {
	db, mock, cleanup := setupMockNewsletterDB(t)
	defer cleanup()

	svc := &NewsletterService{DB: db}
	recorder, restore := stubNewsletterHooks(nil, nil, nil)
	defer restore()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "", "published", "public", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "newsletter_media" WHERE newsletter_entry_id = \$1 ORDER BY sort_order ASC,id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "", "gs://drive-bucket/news-letters/documents/agenda.pdf", "application/pdf", 100, "attachment", 0, nil, nil, time.Now(), time.Now(),
		).AddRow(
			5, 11, "Minutes", "minutes.pdf", "", "gs://drive-bucket/news-letters/documents/minutes.pdf", "application/pdf", 100, "attachment", 1, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "newsletter_media" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "newsletter_media" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "newsletter_entries" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	reorderResp, err := svc.ReorderNewsletterMedia(11, []int{5, 4})
	if err != nil {
		t.Fatalf("ReorderNewsletterMedia returned error: %v", err)
	}
	if reorderResp.UpdatedCount != 2 {
		t.Fatalf("unexpected reorder response: %#v", reorderResp)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "", "published", "public", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(`SELECT \* FROM "newsletter_media" WHERE id IN \(\$1,\$2\) AND newsletter_entry_id = \$3`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "", "gs://drive-bucket/news-letters/documents/agenda.pdf", "application/pdf", 100, "attachment", 0, nil, nil, time.Now(), time.Now(),
		).AddRow(
			5, 11, "Minutes", "minutes.pdf", "", "gs://drive-bucket/news-letters/documents/minutes.pdf", "application/pdf", 100, "attachment", 1, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "newsletter_media" WHERE id IN \(\$1,\$2\) AND newsletter_entry_id = \$3`).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(`SELECT \* FROM "newsletter_media" WHERE newsletter_entry_id = \$1 ORDER BY sort_order ASC,id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "newsletter_entries" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	deleteResp, err := svc.DeleteNewsletterMedia(11, []int{4, 5})
	if err != nil {
		t.Fatalf("DeleteNewsletterMedia returned error: %v", err)
	}
	if deleteResp.DeletedCount != 2 {
		t.Fatalf("unexpected delete response: %#v", deleteResp)
	}
	if len(recorder.deletes) != 2 {
		t.Fatalf("expected stored object cleanup, got %#v", recorder.deletes)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestReorderAndDeleteNewsletterMediaValidationAndErrorBranches(t *testing.T) {
	db, mock, cleanup := setupMockNewsletterDB(t)
	defer cleanup()

	svc := &NewsletterService{DB: db}

	if _, err := svc.ReorderNewsletterMedia(11, nil); err == nil {
		t.Fatal("expected media_ids validation error")
	}
	if _, err := svc.DeleteNewsletterMedia(11, []int{1, 1}); err == nil {
		t.Fatal("expected duplicate media_ids validation error")
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "", "published", "public", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "newsletter_media" WHERE newsletter_entry_id = \$1 ORDER BY sort_order ASC,id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "", "gs://drive-bucket/news-letters/documents/agenda.pdf", "application/pdf", 100, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectRollback()

	if _, err := svc.ReorderNewsletterMedia(11, []int{9}); !errors.Is(err, ErrNewsletterMediaNotFound) {
		t.Fatalf("expected ErrNewsletterMediaNotFound for unknown requested media, got %v", err)
	}

	db, mock, cleanup = setupMockNewsletterDB(t)
	defer cleanup()
	svc = &NewsletterService{DB: db}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnError(gorm.ErrRecordNotFound)
	if _, err := svc.DeleteNewsletterMedia(99, []int{4}); !errors.Is(err, ErrNewsletterEntryNotFound) {
		t.Fatalf("expected ErrNewsletterEntryNotFound for delete media, got %v", err)
	}

	db, mock, cleanup = setupMockNewsletterDB(t)
	defer cleanup()
	svc = &NewsletterService{DB: db}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "newsletter_entries" WHERE "newsletter_entries"."id" = $1 ORDER BY "newsletter_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "category", "send_date", "content_html", "status", "visibility",
			"publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Update", "csaa", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "", "published", "public", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(`SELECT \* FROM "newsletter_media" WHERE id IN \(\$1\) AND newsletter_entry_id = \$2`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "", "gs://drive-bucket/news-letters/documents/agenda.pdf", "application/pdf", 100, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "newsletter_media" WHERE id IN \(\$1\) AND newsletter_entry_id = \$2`).
		WillReturnError(errors.New("delete failed"))
	mock.ExpectRollback()

	if _, err := svc.DeleteNewsletterMedia(11, []int{4}); err == nil || err.Error() != "delete failed" {
		t.Fatalf("expected delete failure, got %v", err)
	}
}

func TestNewsletterHelpersAndValidation(t *testing.T) {
	recorder, restore := stubNewsletterHooks(nil, nil, nil)
	defer restore()

	req := validSaveNewsletterEntryRequest()
	req.Status = "scheduled"
	if _, _, _, err := normalizeSaveNewsletterEntryRequest(req); err == nil {
		t.Fatal("expected publish_at required validation error")
	}

	req = validSaveNewsletterEntryRequest()
	req.Title = " "
	if _, _, _, err := normalizeSaveNewsletterEntryRequest(req); err == nil {
		t.Fatal("expected title required validation error")
	}

	req = validSaveNewsletterEntryRequest()
	req.Status = "bad"
	if _, _, _, err := normalizeSaveNewsletterEntryRequest(req); err == nil {
		t.Fatal("expected invalid status error")
	}

	req = validSaveNewsletterEntryRequest()
	req.Visibility = "bad"
	if _, _, _, err := normalizeSaveNewsletterEntryRequest(req); err == nil {
		t.Fatal("expected invalid visibility error")
	}

	req = validSaveNewsletterEntryRequest()
	req.Category = "bad"
	if _, _, _, err := normalizeSaveNewsletterEntryRequest(req); err == nil {
		t.Fatal("expected category validation error")
	}

	req = validSaveNewsletterEntryRequest()
	req.SendDate = " "
	if _, _, _, err := normalizeSaveNewsletterEntryRequest(req); err == nil {
		t.Fatal("expected send_date required validation error")
	}

	req = validSaveNewsletterEntryRequest()
	req.SendDate = "bad"
	if _, _, _, err := normalizeSaveNewsletterEntryRequest(req); err == nil {
		t.Fatal("expected send_date validation error")
	}

	req = validSaveNewsletterEntryRequest()
	req.Title = strings.Repeat("a", 256)
	if _, _, _, err := normalizeSaveNewsletterEntryRequest(req); err == nil {
		t.Fatal("expected title length validation error")
	}

	req = validSaveNewsletterEntryRequest()
	req.Status = ""
	req.Visibility = ""
	req.Category = ""
	if normalized, sendDate, publishAt, err := normalizeSaveNewsletterEntryRequest(req); err != nil {
		t.Fatalf("normalizeSaveNewsletterEntryRequest returned error: %v", err)
	} else {
		if normalized.Status != "draft" || normalized.Visibility != "public" {
			t.Fatalf("expected normalized defaults, got %#v", normalized)
		}
		if sendDate.Format("2006-01-02") != "2026-05-01" || publishAt != nil {
			t.Fatalf("unexpected normalized dates: send=%v publish=%v", sendDate, publishAt)
		}
	}

	req = validSaveNewsletterEntryRequest()
	req.Status = "scheduled"
	req.PublishAt = strPtr("2026-05-02T10:00:00Z")
	if normalized, _, publishAt, err := normalizeSaveNewsletterEntryRequest(req); err != nil || normalized.Status != "scheduled" || publishAt == nil {
		t.Fatalf("expected scheduled status to normalize successfully, got %#v publishAt=%v err=%v", normalized, publishAt, err)
	}

	if parsed, err := parseOptionalNewsletterTime(strPtr("2026-05-01T10:30:00Z")); err != nil || parsed == nil {
		t.Fatalf("expected publish_at parse success, got %v", err)
	}
	if parsed, err := parseOptionalNewsletterTime(nil); err != nil || parsed != nil {
		t.Fatalf("expected nil publish_at to be accepted, got parsed=%v err=%v", parsed, err)
	}
	if parsed, err := parseOptionalNewsletterTime(strPtr("2026-05-01")); err != nil || parsed == nil {
		t.Fatalf("expected date-only publish_at parse success, got parsed=%v err=%v", parsed, err)
	}
	if _, err := parseOptionalNewsletterTime(strPtr("bad")); err == nil {
		t.Fatal("expected publish_at parse error")
	}

	svc := &NewsletterService{BucketName: "drive-bucket", BucketPrefix: "main-folder"}
	media, uploadedKey, err := svc.buildNewsletterMediaModel(11, NewsletterUploadInput{
		DisplayName: "External",
		FileURL:     "gs://drive-bucket/main-folder/news-letters/documents/existing.pdf",
		MimeType:    "application/pdf",
	}, intPtr(7), 0)
	if err != nil || uploadedKey != "" || media.GCPObjectKey != "main-folder/news-letters/documents/existing.pdf" {
		t.Fatalf("unexpected referenced media result: media=%#v uploaded=%q err=%v", media, uploadedKey, err)
	}

	media, uploadedKey, err = svc.buildNewsletterMediaModel(11, NewsletterUploadInput{
		FileName: "agenda.pdf",
		MimeType: "application/pdf",
		Content:  []byte("agenda"),
	}, intPtr(7), 0)
	if err != nil || uploadedKey == "" || media.FileURL == "" || media.SortOrder != 0 {
		t.Fatalf("unexpected uploaded media result: media=%#v uploadedKey=%q err=%v", media, uploadedKey, err)
	}

	if _, _, err := svc.buildNewsletterMediaModel(11, NewsletterUploadInput{}, intPtr(7), 0); err == nil {
		t.Fatal("expected missing media file validation error")
	}
	if _, _, err := (&NewsletterService{}).buildNewsletterMediaModel(11, NewsletterUploadInput{
		FileName: "agenda.pdf",
		Content:  []byte("agenda"),
	}, intPtr(7), 0); !errors.Is(err, ErrMediaBucketNotConfigured) {
		t.Fatalf("expected ErrMediaBucketNotConfigured, got %v", err)
	}

	if bucket, key, err := svc.resolveStoredObjectReference("", "gs://other-bucket/folder/a.pdf"); err != nil || bucket != "other-bucket" || key != "folder/a.pdf" {
		t.Fatalf("unexpected object resolution: bucket=%q key=%q err=%v", bucket, key, err)
	}
	if bucket, key, err := svc.resolveStoredObjectReference("folder/b.pdf", ""); err != nil || bucket != "drive-bucket" || key != "folder/b.pdf" {
		t.Fatalf("unexpected explicit object resolution: bucket=%q key=%q err=%v", bucket, key, err)
	}
	if _, _, err := svc.resolveStoredObjectReference("", "https://example.com/a.pdf"); err == nil || err.Error() != "media content is not available from storage" {
		t.Fatalf("expected non-storage reference error, got %v", err)
	}
	if _, _, err := svc.resolveStoredObjectReference("", ""); err == nil || err.Error() != "media content is not available from storage" {
		t.Fatalf("expected empty storage reference error, got %v", err)
	}
	if _, _, err := svc.resolveStoredObjectReference("", "gs://drive-bucket"); err == nil || err.Error() != "media content is not available from storage" {
		t.Fatalf("expected missing object path storage error, got %v", err)
	}
	if _, _, err := (&NewsletterService{}).resolveStoredObjectReference("folder/a.pdf", ""); !errors.Is(err, ErrMediaBucketNotConfigured) {
		t.Fatalf("expected ErrMediaBucketNotConfigured, got %v", err)
	}

	svc.deleteStoredObjectBestEffort("", "gs://drive-bucket/main-folder/news-letters/documents/a.pdf")
	if len(recorder.deletes) == 0 || recorder.deletes[len(recorder.deletes)-1] != "drive-bucket/main-folder/news-letters/documents/a.pdf" {
		t.Fatalf("unexpected helper delete cleanup: %#v", recorder.deletes)
	}
	svc.deleteObjectBestEffort("main-folder/news-letters/documents/b.pdf")
	if recorder.deletes[len(recorder.deletes)-1] != "drive-bucket/main-folder/news-letters/documents/b.pdf" {
		t.Fatalf("expected direct delete cleanup, got %#v", recorder.deletes)
	}
	beforeDeletes := len(recorder.deletes)
	svc.deleteStoredObjectBestEffort("", "https://example.com/not-storage.pdf")
	if len(recorder.deletes) != beforeDeletes {
		t.Fatalf("expected non-storage delete helper to skip cleanup, got %#v", recorder.deletes)
	}

	if cleanIDs, err := validateNewsletterMediaIDs([]int{3, 2, 1}); err != nil || len(cleanIDs) != 3 {
		t.Fatalf("expected valid media ids, got %#v err=%v", cleanIDs, err)
	}
	if _, err := validateNewsletterMediaIDs(nil); err == nil {
		t.Fatal("expected required media id error")
	}
	if _, err := validateNewsletterMediaIDs([]int{1, 1}); err == nil {
		t.Fatal("expected duplicate media id error")
	}
	if _, err := validateNewsletterMediaIDs([]int{0}); err == nil {
		t.Fatal("expected positive media id error")
	}

	if got := allowedNewsletterSortColumn("title"); got != "title" {
		t.Fatalf("unexpected sort column: %q", got)
	}
	if got := allowedNewsletterSortColumn("created_at"); got != "created_at" {
		t.Fatalf("unexpected created_at sort column: %q", got)
	}
	if got := allowedNewsletterSortColumn("updated_at"); got != "updated_at" {
		t.Fatalf("unexpected updated_at sort column: %q", got)
	}
	if got := allowedNewsletterSortColumn("status"); got != "status" {
		t.Fatalf("unexpected status sort column: %q", got)
	}
	if got := allowedNewsletterSortColumn("visibility"); got != "visibility" {
		t.Fatalf("unexpected visibility sort column: %q", got)
	}
	if got := allowedNewsletterSortColumn("category"); got != "category" {
		t.Fatalf("unexpected category sort column: %q", got)
	}
	if got := allowedNewsletterSortColumn("bad"); got != "send_date" {
		t.Fatalf("expected default sort column, got %q", got)
	}
	if got := normalizeMimeType(""); got != "application/octet-stream" {
		t.Fatalf("unexpected normalized mime type: %q", got)
	}
	if got := sanitizeStoredFilename(` \tmp\agenda.pdf `); got != "agenda.pdf" {
		t.Fatalf("unexpected stored filename: %q", got)
	}
	if got := sanitizeStoredFilename(`/`); got != "" {
		t.Fatalf("expected slash-only filename to sanitize to empty, got %q", got)
	}
	if got := safeFileExtension("agenda.pdf"); got != ".pdf" {
		t.Fatalf("unexpected safe extension: %q", got)
	}
	if got := safeFileExtension("agenda.abcdefghijklmnopqrstuvwxyz"); got != "" {
		t.Fatalf("expected overlong extension to be dropped, got %q", got)
	}
	if !looksLikeGCSReference("gs://drive-bucket/a.pdf") || looksLikeGCSReference("https://example.com/a.pdf") {
		t.Fatalf("unexpected GCS reference detection")
	}
	if looksLikeGCSReference("") {
		t.Fatal("expected empty storage reference to be false")
	}
	if !looksLikeGCSReference("https://storage.googleapis.com/drive-bucket/a.pdf") {
		t.Fatal("expected storage.googleapis.com URL to be treated as GCS")
	}
	if !looksLikeGCSReference("folder/a.pdf") {
		t.Fatal("expected bare object path to be treated as storage reference")
	}
	if !isAllowedNewsletterStatus("draft") || isAllowedNewsletterStatus("archived") {
		t.Fatal("unexpected newsletter status validation")
	}
	if !isAllowedNewsletterVisibility("public") || isAllowedNewsletterVisibility("scheduled") {
		t.Fatal("unexpected newsletter visibility validation")
	}
	if !isAllowedNewsletterCategory("csaa") || isAllowedNewsletterCategory("press") {
		t.Fatal("unexpected newsletter category validation")
	}

	entry := NewsletterEntry{ID: 9, Title: "Spring Update", Category: "csaa", SendDate: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), Status: "published", Visibility: "public"}
	if summary := newsletterSummaryFromModel(entry, nil); summary.Title != "Spring Update" {
		t.Fatalf("unexpected summary conversion: %#v", summary)
	}
	if summary := newsletterSummaryFromModel(entry, nil); summary.Media == nil {
		t.Fatalf("expected summary media to default to empty slice, got %#v", summary)
	}
	if mutation := newsletterMutationFromModel(entry); mutation.ID != 9 {
		t.Fatalf("unexpected mutation conversion: %#v", mutation)
	}
	if detail := newsletterDetailFromModel(entry, nil); detail.Media == nil || detail.Title != "Spring Update" {
		t.Fatalf("unexpected detail conversion: %#v", detail)
	}
	if mediaResp := newsletterMediaFromModel(NewsletterMedia{ID: 4, DisplayName: "Agenda"}); mediaResp.DisplayName != "Agenda" {
		t.Fatalf("unexpected media conversion: %#v", mediaResp)
	}
	if mediaPtr := newsletterMediaPtrFromModel(NewsletterMedia{ID: 4, DisplayName: "Agenda"}); mediaPtr == nil || mediaPtr.DisplayName != "Agenda" {
		t.Fatalf("unexpected media pointer conversion: %#v", mediaPtr)
	}

	if (NewsletterEntry{}).TableName() != "newsletter_entries" || (NewsletterMedia{}).TableName() != "newsletter_media" {
		t.Fatal("unexpected table names")
	}
}

func TestNextSortOrderAndResequenceHelpers(t *testing.T) {
	db, mock, cleanup := setupMockNewsletterDB(t)
	defer cleanup()

	mock.ExpectBegin()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("db.Begin returned error: %v", tx.Error)
	}

	mock.ExpectQuery(`SELECT MAX\(sort_order\) FROM "newsletter_media" WHERE newsletter_entry_id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(3))
	nextSort, err := nextNewsletterMediaSortOrder(tx, 11)
	if err != nil || nextSort != 4 {
		t.Fatalf("expected next sort 4, got %d err=%v", nextSort, err)
	}

	mock.ExpectQuery(`SELECT \* FROM "newsletter_media" WHERE newsletter_entry_id = \$1 ORDER BY sort_order ASC,id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "", "gs://drive-bucket/news-letters/documents/agenda.pdf", "application/pdf", 100, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))
	if err := resequenceNewsletterMedia(tx, 11); err != nil {
		t.Fatalf("expected resequence no-op success, got %v", err)
	}

	mock.ExpectRollback()
	if err := tx.Rollback().Error; err != nil {
		t.Fatalf("tx.Rollback returned error: %v", err)
	}

	mock.ExpectBegin()
	tx = db.Begin()
	if tx.Error != nil {
		t.Fatalf("db.Begin returned error: %v", tx.Error)
	}

	mock.ExpectQuery(`SELECT \* FROM "newsletter_media" WHERE newsletter_entry_id = \$1 ORDER BY sort_order ASC,id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "newsletter_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "", "gs://drive-bucket/news-letters/documents/agenda.pdf", "application/pdf", 100, "attachment", 2, nil, nil, time.Now(), time.Now(),
		).AddRow(
			5, 11, "Minutes", "minutes.pdf", "", "gs://drive-bucket/news-letters/documents/minutes.pdf", "application/pdf", 100, "attachment", 4, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "newsletter_media" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "newsletter_media" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := resequenceNewsletterMedia(tx, 11); err != nil {
		t.Fatalf("expected resequence update success, got %v", err)
	}

	mock.ExpectRollback()
	if err := tx.Rollback().Error; err != nil {
		t.Fatalf("tx.Rollback returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func validSaveNewsletterEntryRequest() SaveNewsletterEntryRequest {
	return SaveNewsletterEntryRequest{
		Title:       "Spring Update",
		Category:    "csaa",
		SendDate:    "2026-05-01",
		ContentHTML: "<p>Hello</p>",
		Status:      "draft",
		Visibility:  "private",
	}
}

func intPtr(v int) *int {
	return &v
}

func strPtr(v string) *string {
	return &v
}
