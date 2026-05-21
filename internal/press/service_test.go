package press

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

	return db, mock, func() { _ = sqlDB.Close() }
}

type pressHookRecorder struct {
	uploads   []string
	downloads []string
	deletes   []string
}

func stubPressHooks(uploadErr, downloadErr, deleteErr error) (*pressHookRecorder, func()) {
	recorder := &pressHookRecorder{}

	prevUpload := uploadBytesToGCSHook
	prevDownload := downloadGCSObjectHook
	prevDelete := deleteGCSObjectHook
	prevNow := pressNowFunc

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
	pressNowFunc = func() time.Time {
		return time.Date(2026, 5, 15, 11, 30, 0, 123, time.UTC)
	}

	return recorder, func() {
		uploadBytesToGCSHook = prevUpload
		downloadGCSObjectHook = prevDownload
		deleteGCSObjectHook = prevDelete
		pressNowFunc = prevNow
	}
}

func TestPressServiceStoreUnavailable(t *testing.T) {
	svc := &PressService{}
	req := validSavePressEntryRequest()

	if _, err := svc.ListPressEntries(ListPressFilter{}); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ListPressEntries ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.GetPressEntry(1); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected GetPressEntry ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.GetPressMediaContent(1, 2); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected GetPressMediaContent ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.CreatePressEntry(req, intPtr(7)); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected CreatePressEntry ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.UpdatePressEntry(1, req, intPtr(7)); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected UpdatePressEntry ErrStoreUnavailable, got %v", err)
	}
	if err := svc.DeletePressEntry(1); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected DeletePressEntry ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.AddPressMedia(1, AddPressMediaRequest{}, intPtr(7)); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected AddPressMedia ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.UpdatePressMedia(1, 2, UpdatePressMediaRequest{}); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected UpdatePressMedia ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.ReorderPressMedia(1, []int{1}); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ReorderPressMedia ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.DeletePressMedia(1, []int{1}); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected DeletePressMedia ErrStoreUnavailable, got %v", err)
	}
}

func TestListPressEntriesSuccessAndValidation(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &PressService{DB: db}

	mock.ExpectQuery(`SELECT count\(\*\) FROM "press_entries"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(6))
	mock.ExpectQuery(`SELECT \* FROM "press_entries"`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "https://example.com/press", "<p>Hello</p>", "published", "public",
			"gs://drive-bucket/press-entries/covers/cover.png", "press-entries/covers/cover.png", nil, 7, 7, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
		))
	mock.ExpectQuery(`SELECT \* FROM "press_media" WHERE press_entry_id IN \(\$1\) ORDER BY press_entry_id ASC,sort_order ASC,id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 9, "Agenda", "agenda.pdf", "press-entries/media/agenda.pdf", "gs://drive-bucket/press-entries/media/agenda.pdf", "application/pdf", 1024, "attachment", 0, 7, 7, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		))

	resp, err := svc.ListPressEntries(ListPressFilter{
		Status:     "published",
		Visibility: "public",
		SearchTerm: "spring",
		SortBy:     "title",
		SortOrder:  "asc",
		Page:       2,
		PageSize:   5,
	})
	if err != nil {
		t.Fatalf("ListPressEntries returned error: %v", err)
	}
	if resp.Total != 6 || len(resp.Items) != 1 || resp.Page != 2 || resp.PageSize != 5 || resp.TotalPages != 2 {
		t.Fatalf("unexpected list response: %#v", resp)
	}
	if resp.Items[0].Title != "Spring Fair" || resp.Items[0].Status != "published" {
		t.Fatalf("unexpected summary item: %#v", resp.Items[0])
	}
	if resp.Items[0].SourceURL != "https://example.com/press" || resp.Items[0].ContentHTML != "<p>Hello</p>" {
		t.Fatalf("expected list item detail fields, got %#v", resp.Items[0])
	}
	if len(resp.Items[0].Media) != 1 || resp.Items[0].Media[0].DisplayName != "Agenda" {
		t.Fatalf("expected list item media, got %#v", resp.Items[0].Media)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}

	if _, err := svc.ListPressEntries(ListPressFilter{Status: "bad"}); err == nil {
		t.Fatal("expected invalid status error")
	}
	if _, err := svc.ListPressEntries(ListPressFilter{Visibility: "bad"}); err == nil {
		t.Fatal("expected invalid visibility error")
	}
}

func TestGetPressEntrySuccessAndNotFound(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &PressService{DB: db}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), 3, "https://example.com/press", "<p>Hello</p>", "published", "public",
			"gs://drive-bucket/press-entries/covers/cover.png", "press-entries/covers/cover.png", time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC), 7, 8, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		))
	mock.ExpectQuery(`SELECT \* FROM "press_media" WHERE press_entry_id = \$1 ORDER BY sort_order ASC, id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 9, "Agenda", "agenda.pdf", "press-entries/media/agenda.pdf", "gs://drive-bucket/press-entries/media/agenda.pdf", "application/pdf", 1024, "attachment", 0, 7, 7, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		))

	resp, err := svc.GetPressEntry(9)
	if err != nil {
		t.Fatalf("GetPressEntry returned error: %v", err)
	}
	if resp.ID != 9 || len(resp.Media) != 1 || resp.Media[0].DisplayName != "Agenda" {
		t.Fatalf("unexpected detail response: %#v", resp)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnError(gorm.ErrRecordNotFound)

	if _, err := svc.GetPressEntry(99); !errors.Is(err, ErrPressEntryNotFound) {
		t.Fatalf("expected ErrPressEntryNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetPressMediaContentAndObjectResolution(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &PressService{DB: db, BucketName: "drive-bucket"}
	recorder, restore := stubPressHooks(nil, nil, nil)
	defer restore()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_media" WHERE id = $1 AND press_entry_id = $2 ORDER BY "press_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 9, "Agenda", "agenda.pdf", "", "gs://other-bucket/folder/agenda.pdf", "application/pdf", 1024, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))

	resp, err := svc.GetPressMediaContent(9, 4)
	if err != nil {
		t.Fatalf("GetPressMediaContent returned error: %v", err)
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

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_media" WHERE id = $1 AND press_entry_id = $2 ORDER BY "press_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			5, 9, "External", "external.pdf", "", "https://example.com/files/external.pdf", "application/pdf", 0, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))

	if _, err := svc.GetPressMediaContent(9, 5); err == nil || err.Error() != "media content is not available from storage" {
		t.Fatalf("expected unavailable storage error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}

	_, restore = stubPressHooks(nil, util.ErrObjectNotFound, nil)
	defer restore()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_media" WHERE id = $1 AND press_entry_id = $2 ORDER BY "press_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			6, 9, "Agenda", "agenda.pdf", "folder/agenda.pdf", "", "application/pdf", 0, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))

	if _, err := svc.GetPressMediaContent(9, 6); !errors.Is(err, ErrPressMediaNotFound) {
		t.Fatalf("expected ErrPressMediaNotFound, got %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_media" WHERE id = $1 AND press_entry_id = $2 ORDER BY "press_media"."id" LIMIT $3`)).
		WillReturnError(gorm.ErrRecordNotFound)
	if _, err := svc.GetPressMediaContent(9, 99); !errors.Is(err, ErrPressMediaNotFound) {
		t.Fatalf("expected ErrPressMediaNotFound for missing media row, got %v", err)
	}

	db, mock, cleanup = setupMockDB(t)
	defer cleanup()
	svc = &PressService{DB: db}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_media" WHERE id = $1 AND press_entry_id = $2 ORDER BY "press_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			7, 9, "Agenda", "agenda.pdf", "folder/agenda.pdf", "", "application/pdf", 0, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))
	if _, err := svc.GetPressMediaContent(9, 7); !errors.Is(err, ErrMediaBucketNotConfigured) {
		t.Fatalf("expected ErrMediaBucketNotConfigured, got %v", err)
	}
}

func TestGetPressMediaContentFallbackContentTypesAndDownloadErrors(t *testing.T) {
	prevDownload := downloadGCSObjectHook
	defer func() {
		downloadGCSObjectHook = prevDownload
	}()

	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &PressService{DB: db, BucketName: "drive-bucket"}
	downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
		return []byte("pdf-bytes"), "", nil
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_media" WHERE id = $1 AND press_entry_id = $2 ORDER BY "press_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 9, "Agenda", "agenda.pdf", "folder/agenda.pdf", "", "application/pdf", 1024, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))

	resp, err := svc.GetPressMediaContent(9, 4)
	if err != nil {
		t.Fatalf("GetPressMediaContent returned error: %v", err)
	}
	if resp.ContentType != "application/pdf" {
		t.Fatalf("expected media mime fallback, got %#v", resp)
	}

	db, mock, cleanup = setupMockDB(t)
	defer cleanup()
	svc = &PressService{DB: db, BucketName: "drive-bucket"}
	downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
		return nil, "", nil
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_media" WHERE id = $1 AND press_entry_id = $2 ORDER BY "press_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			5, 9, "Poster", "poster.bin", "folder/poster.bin", "", "", 128, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))

	resp, err = svc.GetPressMediaContent(9, 5)
	if err != nil {
		t.Fatalf("GetPressMediaContent returned error: %v", err)
	}
	if resp.ContentType != "application/octet-stream" {
		t.Fatalf("expected octet-stream fallback, got %#v", resp)
	}

	db, mock, cleanup = setupMockDB(t)
	defer cleanup()
	svc = &PressService{DB: db, BucketName: "drive-bucket"}
	downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
		return nil, "", errors.New("download failed")
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_media" WHERE id = $1 AND press_entry_id = $2 ORDER BY "press_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			6, 9, "Agenda", "agenda.pdf", "folder/agenda.pdf", "", "application/pdf", 1024, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))

	if _, err := svc.GetPressMediaContent(9, 6); err == nil || err.Error() != "download failed" {
		t.Fatalf("expected download failure to be returned, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCreateAndUpdatePressEntry(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &PressService{DB: db, BucketName: "drive-bucket", BucketPrefix: "main-folder"}
	recorder, restore := stubPressHooks(nil, nil, nil)
	defer restore()

	createReq := validSavePressEntryRequest()
	createReq.CoverImage = &PressUploadInput{FileName: "cover.png", MimeType: "image/png", Content: []byte("cover")}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "press_entries"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectCommit()

	createResp, err := svc.CreatePressEntry(createReq, intPtr(7))
	if err != nil {
		t.Fatalf("CreatePressEntry returned error: %v", err)
	}
	if createResp.ID != 11 || createResp.Title != "Spring Fair" {
		t.Fatalf("unexpected create response: %#v", createResp)
	}
	if len(recorder.uploads) != 1 || !strings.Contains(recorder.uploads[0], "drive-bucket/main-folder/press-entries/covers/") {
		t.Fatalf("unexpected upload calls: %#v", recorder.uploads)
	}

	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "https://example.com/press", "<p>Hello</p>", "draft", "private",
			"gs://drive-bucket/main-folder/press-entries/covers/old.png", "main-folder/press-entries/covers/old.png", nil, 7, 7, now, now,
		))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "press_entries" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updateReq := validSavePressEntryRequest()
	updateReq.Title = "Spring Fair Updated"
	updateReq.Status = "published"
	updateReq.Visibility = "public"
	updateReq.CoverImage = &PressUploadInput{FileName: "new-cover.png", MimeType: "image/png", Content: []byte("new-cover")}
	updateResp, err := svc.UpdatePressEntry(11, updateReq, intPtr(8))
	if err != nil {
		t.Fatalf("UpdatePressEntry returned error: %v", err)
	}
	if updateResp.Title != "Spring Fair Updated" || updateResp.Status != "published" {
		t.Fatalf("unexpected update response: %#v", updateResp)
	}
	if len(recorder.deletes) == 0 || recorder.deletes[len(recorder.deletes)-1] != "drive-bucket/main-folder/press-entries/covers/old.png" {
		t.Fatalf("expected old cover cleanup, got %#v", recorder.deletes)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestDeletePressEntryCleansUpStoredObjects(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &PressService{DB: db, BucketName: "drive-bucket"}
	recorder, restore := stubPressHooks(nil, nil, nil)
	defer restore()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public",
			"gs://drive-bucket/press-entries/covers/cover.png", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(`SELECT \* FROM "press_media" WHERE press_entry_id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			3, 11, "Agenda", "agenda.pdf", "", "gs://drive-bucket/press-entries/media/agenda.pdf", "application/pdf", 100, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "press_media" WHERE press_entry_id = \$1`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM "press_entries" WHERE "press_entries"."id" = \$1`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := svc.DeletePressEntry(11); err != nil {
		t.Fatalf("DeletePressEntry returned error: %v", err)
	}
	if len(recorder.deletes) != 2 {
		t.Fatalf("expected 2 cleanup deletes, got %#v", recorder.deletes)
	}
	if recorder.deletes[0] != "drive-bucket/press-entries/covers/cover.png" || recorder.deletes[1] != "drive-bucket/press-entries/media/agenda.pdf" {
		t.Fatalf("unexpected cleanup calls: %#v", recorder.deletes)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestDeletePressEntryAdditionalErrorBranches(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &PressService{DB: db, BucketName: "drive-bucket"}
	recorder, restore := stubPressHooks(nil, nil, nil)
	defer restore()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public",
			"gs://drive-bucket/press-entries/covers/cover.png", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(`SELECT \* FROM "press_media" WHERE press_entry_id = \$1`).
		WillReturnError(errors.New("media lookup failed"))

	if err := svc.DeletePressEntry(11); err == nil || err.Error() != "media lookup failed" {
		t.Fatalf("expected media lookup failure, got %v", err)
	}
	if len(recorder.deletes) != 0 {
		t.Fatalf("expected no cleanup deletes on lookup failure, got %#v", recorder.deletes)
	}

	db, mock, cleanup = setupMockDB(t)
	defer cleanup()
	svc = &PressService{DB: db, BucketName: "drive-bucket"}
	recorder, restore = stubPressHooks(nil, nil, nil)
	defer restore()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public",
			"gs://drive-bucket/press-entries/covers/cover.png", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(`SELECT \* FROM "press_media" WHERE press_entry_id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			3, 11, "Agenda", "agenda.pdf", "press-entries/media/agenda.pdf", "", "application/pdf", 100, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "press_media" WHERE press_entry_id = \$1`).
		WillReturnError(errors.New("delete failed"))
	mock.ExpectRollback()

	if err := svc.DeletePressEntry(11); err == nil || err.Error() != "delete failed" {
		t.Fatalf("expected delete failure, got %v", err)
	}
	if len(recorder.deletes) != 0 {
		t.Fatalf("expected no cleanup deletes on transaction failure, got %#v", recorder.deletes)
	}
}

func TestCreatePressEntryCleanupOnDBError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &PressService{DB: db, BucketName: "drive-bucket"}
	recorder, restore := stubPressHooks(nil, nil, nil)
	defer restore()

	req := validSavePressEntryRequest()
	req.CoverImage = &PressUploadInput{FileName: "cover.png", MimeType: "image/png", Content: []byte("cover")}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "press_entries"`)).
		WillReturnError(errors.New("insert failed"))
	mock.ExpectRollback()

	if _, err := svc.CreatePressEntry(req, intPtr(7)); err == nil {
		t.Fatal("expected create error")
	}
	if len(recorder.deletes) != 1 || !strings.Contains(recorder.deletes[0], "drive-bucket/press-entries/covers/") {
		t.Fatalf("expected uploaded cover cleanup, got %#v", recorder.deletes)
	}
}

func TestUpdatePressEntryRemoveCoverAndSaveError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &PressService{DB: db, BucketName: "drive-bucket"}
	recorder, restore := stubPressHooks(nil, nil, nil)
	defer restore()

	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "draft", "private",
			"gs://drive-bucket/press-entries/covers/old.png", "press-entries/covers/old.png", nil, 7, 7, now, now,
		))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "press_entries" SET`)).
		WillReturnError(errors.New("save failed"))
	mock.ExpectRollback()

	req := validSavePressEntryRequest()
	req.RemoveCoverImage = true
	if _, err := svc.UpdatePressEntry(11, req, intPtr(7)); err == nil {
		t.Fatal("expected update save error")
	}
	if len(recorder.deletes) != 0 {
		t.Fatalf("expected no cleanup on failed remove-only update, got %#v", recorder.deletes)
	}
}

func TestAddPressMediaAndBuildPressMediaModel(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &PressService{DB: db, BucketName: "drive-bucket", BucketPrefix: "main-folder"}
	recorder, restore := stubPressHooks(nil, nil, nil)
	defer restore()

	if _, err := svc.AddPressMedia(1, AddPressMediaRequest{}, intPtr(7)); err == nil {
		t.Fatal("expected media validation error")
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT MAX\(sort_order\) FROM "press_media" WHERE press_entry_id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(nil))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "press_media"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(21))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "press_media"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(22))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "press_entries" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	resp, err := svc.AddPressMedia(11, AddPressMediaRequest{
		Media: []PressUploadInput{
			{DisplayName: "Agenda", FileName: "agenda.pdf", MimeType: "application/pdf", Content: []byte("agenda")},
			{DisplayName: "Minutes", FileURL: "gs://drive-bucket/main-folder/press-entries/media/minutes.pdf", MimeType: "application/pdf"},
		},
	}, intPtr(7))
	if err != nil {
		t.Fatalf("AddPressMedia returned error: %v", err)
	}
	if resp.UploadedCount != 1 {
		t.Fatalf("expected uploaded count 1, got %#v", resp)
	}
	if len(recorder.uploads) != 1 || !strings.Contains(recorder.uploads[0], "main-folder/press-entries/media/") {
		t.Fatalf("unexpected uploads: %#v", recorder.uploads)
	}

	media, uploadedKey, err := svc.buildPressMediaModel(11, PressUploadInput{
		DisplayName: "External",
		FileURL:     "gs://drive-bucket/main-folder/press-entries/media/external.pdf",
		MimeType:    "application/pdf",
	}, intPtr(7), 0)
	if err != nil {
		t.Fatalf("buildPressMediaModel returned error: %v", err)
	}
	if uploadedKey != "" || media.GCPObjectKey != "main-folder/press-entries/media/external.pdf" {
		t.Fatalf("unexpected direct media model: %#v uploaded=%q", media, uploadedKey)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}

	db, mock, cleanup = setupMockDB(t)
	defer cleanup()
	svc = &PressService{DB: db, BucketName: "drive-bucket"}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnError(gorm.ErrRecordNotFound)
	if _, err := svc.AddPressMedia(99, AddPressMediaRequest{Media: []PressUploadInput{{FileURL: "gs://drive-bucket/a.pdf"}}}, intPtr(7)); !errors.Is(err, ErrPressEntryNotFound) {
		t.Fatalf("expected ErrPressEntryNotFound, got %v", err)
	}

	db, mock, cleanup = setupMockDB(t)
	defer cleanup()
	svc = &PressService{DB: db, BucketName: "drive-bucket"}
	recorder, restore = stubPressHooks(nil, nil, nil)
	defer restore()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT MAX\(sort_order\) FROM "press_media" WHERE press_entry_id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(nil))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "press_media"`)).
		WillReturnError(errors.New("insert failed"))
	mock.ExpectRollback()
	if _, err := svc.AddPressMedia(11, AddPressMediaRequest{
		Media: []PressUploadInput{{FileName: "agenda.pdf", MimeType: "application/pdf", Content: []byte("agenda")}},
	}, intPtr(7)); err == nil {
		t.Fatal("expected add media insert error")
	}
	if len(recorder.deletes) != 1 || !strings.Contains(recorder.deletes[0], "drive-bucket/press-entries/media/") {
		t.Fatalf("expected uploaded media cleanup after insert failure, got %#v", recorder.deletes)
	}
}

func TestUpdatePressMedia(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &PressService{DB: db}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_media" WHERE id = $1 AND press_entry_id = $2 ORDER BY "press_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "press-entries/media/agenda.pdf", "gs://drive-bucket/press-entries/media/agenda.pdf", "application/pdf", 100, "attachment", 0, 7, 7, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "press_media" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "press_entries" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	resp, err := svc.UpdatePressMedia(11, 4, UpdatePressMediaRequest{DisplayName: "Agenda v2", FileName: "agenda-v2.pdf"})
	if err != nil {
		t.Fatalf("UpdatePressMedia returned error: %v", err)
	}
	if resp.DisplayName != "Agenda v2" || resp.FileName != "agenda-v2.pdf" {
		t.Fatalf("unexpected media response: %#v", resp)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_media" WHERE id = $1 AND press_entry_id = $2 ORDER BY "press_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "press-entries/media/agenda.pdf", "gs://drive-bucket/press-entries/media/agenda.pdf", "application/pdf", 100, "attachment", 0, 7, 7, time.Now(), time.Now(),
		))
	if _, err := svc.UpdatePressMedia(11, 4, UpdatePressMediaRequest{DisplayName: " ", FileName: " "}); err == nil {
		t.Fatal("expected update media validation error")
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_media" WHERE id = $1 AND press_entry_id = $2 ORDER BY "press_media"."id" LIMIT $3`)).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnError(gorm.ErrRecordNotFound)
	if _, err := svc.UpdatePressMedia(11, 99, UpdatePressMediaRequest{DisplayName: "Missing"}); !errors.Is(err, ErrPressEntryNotFound) {
		t.Fatalf("expected ErrPressEntryNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestUpdatePressMediaAdditionalBranches(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &PressService{DB: db}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_media" WHERE id = $1 AND press_entry_id = $2 ORDER BY "press_media"."id" LIMIT $3`)).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))

	if _, err := svc.UpdatePressMedia(11, 99, UpdatePressMediaRequest{DisplayName: "Missing"}); !errors.Is(err, ErrPressMediaNotFound) {
		t.Fatalf("expected ErrPressMediaNotFound, got %v", err)
	}

	db, mock, cleanup = setupMockDB(t)
	defer cleanup()
	svc = &PressService{DB: db}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_media" WHERE id = $1 AND press_entry_id = $2 ORDER BY "press_media"."id" LIMIT $3`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "press-entries/media/agenda.pdf", "gs://drive-bucket/press-entries/media/agenda.pdf", "application/pdf", 100, "attachment", 0, 7, 7, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "press_media" SET`)).
		WillReturnError(errors.New("save failed"))
	mock.ExpectRollback()

	if _, err := svc.UpdatePressMedia(11, 4, UpdatePressMediaRequest{DisplayName: "Agenda v2"}); err == nil || err.Error() != "save failed" {
		t.Fatalf("expected save failure, got %v", err)
	}
}

func TestReorderAndDeletePressMedia(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &PressService{DB: db}
	recorder, restore := stubPressHooks(nil, nil, nil)
	defer restore()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "press_media" WHERE press_entry_id = \$1 ORDER BY sort_order ASC,id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "", "gs://drive-bucket/press-entries/media/agenda.pdf", "application/pdf", 100, "attachment", 0, nil, nil, time.Now(), time.Now(),
		).AddRow(
			5, 11, "Minutes", "minutes.pdf", "", "gs://drive-bucket/press-entries/media/minutes.pdf", "application/pdf", 100, "attachment", 1, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "press_media" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "press_media" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "press_entries" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	reorderResp, err := svc.ReorderPressMedia(11, []int{5, 4})
	if err != nil {
		t.Fatalf("ReorderPressMedia returned error: %v", err)
	}
	if reorderResp.UpdatedCount != 2 {
		t.Fatalf("unexpected reorder response: %#v", reorderResp)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "press_media" WHERE press_entry_id = \$1 ORDER BY sort_order ASC,id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "", "gs://drive-bucket/press-entries/media/agenda.pdf", "application/pdf", 100, "attachment", 0, nil, nil, time.Now(), time.Now(),
		).AddRow(
			5, 11, "Minutes", "minutes.pdf", "", "gs://drive-bucket/press-entries/media/minutes.pdf", "application/pdf", 100, "attachment", 1, nil, nil, time.Now(), time.Now(),
		).AddRow(
			6, 11, "Poster", "poster.png", "", "gs://drive-bucket/press-entries/media/poster.png", "image/png", 100, "attachment", 2, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectRollback()
	if _, err := svc.ReorderPressMedia(11, []int{4, 5}); err == nil || err.Error() != "media_ids must include every press media item exactly once" {
		t.Fatalf("expected full reorder validation error, got %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(`SELECT \* FROM "press_media" WHERE id IN \(\$1,\$2\) AND press_entry_id = \$3`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "", "gs://drive-bucket/press-entries/media/agenda.pdf", "application/pdf", 100, "attachment", 0, nil, nil, time.Now(), time.Now(),
		).AddRow(
			5, 11, "Minutes", "minutes.pdf", "", "gs://drive-bucket/press-entries/media/minutes.pdf", "application/pdf", 100, "attachment", 1, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "press_media" WHERE id IN \(\$1,\$2\) AND press_entry_id = \$3`).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(`SELECT \* FROM "press_media" WHERE press_entry_id = \$1 ORDER BY sort_order ASC,id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			6, 11, "Poster", "poster.png", "", "gs://drive-bucket/press-entries/media/poster.png", "image/png", 100, "attachment", 2, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "press_media" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "press_entries" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	deleteResp, err := svc.DeletePressMedia(11, []int{4, 5})
	if err != nil {
		t.Fatalf("DeletePressMedia returned error: %v", err)
	}
	if deleteResp.DeletedCount != 2 {
		t.Fatalf("unexpected delete response: %#v", deleteResp)
	}
	if len(recorder.deletes) != 2 {
		t.Fatalf("expected 2 cleanup deletes, got %#v", recorder.deletes)
	}
	if _, err := svc.DeletePressMedia(11, nil); err == nil {
		t.Fatal("expected delete media validation error")
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(`SELECT \* FROM "press_media" WHERE id IN \(\$1,\$2\) AND press_entry_id = \$3`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "", "gs://drive-bucket/press-entries/media/agenda.pdf", "application/pdf", 100, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))
	if _, err := svc.DeletePressMedia(11, []int{4, 5}); !errors.Is(err, ErrPressMediaNotFound) {
		t.Fatalf("expected ErrPressMediaNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestReorderAndDeletePressMediaAdditionalBranches(t *testing.T) {
	svc := &PressService{DB: &gorm.DB{}}
	if _, err := svc.ReorderPressMedia(11, []int{4, 4}); err == nil {
		t.Fatal("expected duplicate media id validation error")
	}
	if _, err := svc.DeletePressMedia(11, []int{4, 4}); err == nil {
		t.Fatal("expected duplicate media id delete validation error")
	}

	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	svc = &PressService{DB: db}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "press_media" WHERE press_entry_id = \$1 ORDER BY sort_order ASC,id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}))
	mock.ExpectRollback()

	if _, err := svc.ReorderPressMedia(11, []int{4}); !errors.Is(err, ErrPressMediaNotFound) {
		t.Fatalf("expected ErrPressMediaNotFound for empty media set, got %v", err)
	}

	db, mock, cleanup = setupMockDB(t)
	defer cleanup()
	svc = &PressService{DB: db}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "press_media" WHERE press_entry_id = \$1 ORDER BY sort_order ASC,id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "", "gs://drive-bucket/press-entries/media/agenda.pdf", "application/pdf", 100, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectRollback()

	if _, err := svc.ReorderPressMedia(11, []int{9}); !errors.Is(err, ErrPressMediaNotFound) {
		t.Fatalf("expected ErrPressMediaNotFound for unknown requested media, got %v", err)
	}

	db, mock, cleanup = setupMockDB(t)
	defer cleanup()
	svc = &PressService{DB: db}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnError(gorm.ErrRecordNotFound)

	if _, err := svc.DeletePressMedia(99, []int{4}); !errors.Is(err, ErrPressEntryNotFound) {
		t.Fatalf("expected ErrPressEntryNotFound for delete media, got %v", err)
	}

	db, mock, cleanup = setupMockDB(t)
	defer cleanup()
	svc = &PressService{DB: db}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "press_entries" WHERE "press_entries"."id" = $1 ORDER BY "press_entries"."id" LIMIT $2`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "release_date", "category_id", "source_url", "content_html", "status", "visibility",
			"cover_image_url", "cover_image_gcp_key", "publish_at", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			11, "Spring Fair", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil, "", "", "published", "public", "", "", nil, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectQuery(`SELECT \* FROM "press_media" WHERE id IN \(\$1\) AND press_entry_id = \$2`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "", "gs://drive-bucket/press-entries/media/agenda.pdf", "application/pdf", 100, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "press_media" WHERE id IN \(\$1\) AND press_entry_id = \$2`).
		WillReturnError(errors.New("delete failed"))
	mock.ExpectRollback()

	if _, err := svc.DeletePressMedia(11, []int{4}); err == nil || err.Error() != "delete failed" {
		t.Fatalf("expected delete failure, got %v", err)
	}
}

func TestPressHelpersAndValidation(t *testing.T) {
	recorder, restore := stubPressHooks(nil, nil, nil)
	defer restore()

	req := validSavePressEntryRequest()
	req.Visibility = "scheduled"
	if _, _, _, err := normalizeSavePressEntryRequest(req); err == nil {
		t.Fatal("expected publish_at required validation error")
	}

	req = validSavePressEntryRequest()
	req.Title = " "
	if _, _, _, err := normalizeSavePressEntryRequest(req); err == nil {
		t.Fatal("expected title required validation error")
	}

	req = validSavePressEntryRequest()
	req.Status = "bad"
	if _, _, _, err := normalizeSavePressEntryRequest(req); err == nil {
		t.Fatal("expected invalid status error")
	}

	req = validSavePressEntryRequest()
	req.Visibility = "bad"
	if _, _, _, err := normalizeSavePressEntryRequest(req); err == nil {
		t.Fatal("expected invalid visibility error")
	}

	req = validSavePressEntryRequest()
	req.CategoryID = intPtr(0)
	if _, _, _, err := normalizeSavePressEntryRequest(req); err == nil {
		t.Fatal("expected category validation error")
	}

	req = validSavePressEntryRequest()
	req.ReleaseDate = " "
	if _, _, _, err := normalizeSavePressEntryRequest(req); err == nil {
		t.Fatal("expected release_date required validation error")
	}

	req = validSavePressEntryRequest()
	req.ReleaseDate = "bad"
	if _, _, _, err := normalizeSavePressEntryRequest(req); err == nil {
		t.Fatal("expected release_date validation error")
	}

	req = validSavePressEntryRequest()
	req.Title = strings.Repeat("a", 256)
	if _, _, _, err := normalizeSavePressEntryRequest(req); err == nil {
		t.Fatal("expected title length validation error")
	}

	req = validSavePressEntryRequest()
	if normalized, releaseDate, publishAt, err := normalizeSavePressEntryRequest(req); err != nil {
		t.Fatalf("normalizeSavePressEntryRequest returned error: %v", err)
	} else {
		if normalized.Status != "draft" || normalized.Visibility != "private" {
			t.Fatalf("expected normalized defaults, got %#v", normalized)
		}
		if releaseDate.Format("2006-01-02") != "2026-05-01" || publishAt != nil {
			t.Fatalf("unexpected normalized dates: release=%v publish=%v", releaseDate, publishAt)
		}
	}

	req = validSavePressEntryRequest()
	req.Visibility = "scheduled"
	req.PublishAt = strPtr("2026-05-02T10:00:00Z")
	if normalized, _, publishAt, err := normalizeSavePressEntryRequest(req); err != nil || normalized.Visibility != "scheduled" || publishAt == nil {
		t.Fatalf("expected scheduled visibility to normalize successfully, got %#v publishAt=%v err=%v", normalized, publishAt, err)
	}

	if parsed, err := parseOptionalPressTime(strPtr("2026-05-01T10:30:00Z")); err != nil || parsed == nil {
		t.Fatalf("expected publish_at parse success, got %v", err)
	}
	if parsed, err := parseOptionalPressTime(nil); err != nil || parsed != nil {
		t.Fatalf("expected nil publish_at to be accepted, got parsed=%v err=%v", parsed, err)
	}
	if parsed, err := parseOptionalPressTime(strPtr("2026-05-01")); err != nil || parsed == nil {
		t.Fatalf("expected date-only publish_at parse success, got parsed=%v err=%v", parsed, err)
	}
	if _, err := parseOptionalPressTime(strPtr("bad")); err == nil {
		t.Fatal("expected publish_at parse error")
	}

	svc := &PressService{BucketName: "drive-bucket", BucketPrefix: "main-folder"}
	fileURL, objectKey, uploaded, err := svc.resolveCoverImage(PressUploadInput{
		FileURL: "gs://drive-bucket/main-folder/press-entries/covers/existing.png",
	}, intPtr(7))
	if err != nil || uploaded || objectKey != "main-folder/press-entries/covers/existing.png" || fileURL == "" {
		t.Fatalf("unexpected referenced cover result: fileURL=%q objectKey=%q uploaded=%v err=%v", fileURL, objectKey, uploaded, err)
	}
	if fileURL, objectKey, uploaded, err := svc.resolveCoverImage(PressUploadInput{}, intPtr(7)); err != nil || uploaded || fileURL != "" || objectKey != "" {
		t.Fatalf("expected empty cover reference to be accepted, got fileURL=%q objectKey=%q uploaded=%v err=%v", fileURL, objectKey, uploaded, err)
	}

	fileURL, objectKey, uploaded, err = svc.resolveCoverImage(PressUploadInput{
		FileName: "cover.png",
		MimeType: "image/png",
		Content:  []byte("cover"),
	}, intPtr(7))
	if err != nil || !uploaded || fileURL == "" || objectKey == "" {
		t.Fatalf("unexpected uploaded cover result: fileURL=%q objectKey=%q uploaded=%v err=%v", fileURL, objectKey, uploaded, err)
	}
	if _, _, _, err := (&PressService{}).resolveCoverImage(PressUploadInput{Content: []byte("cover")}, intPtr(7)); !errors.Is(err, ErrMediaBucketNotConfigured) {
		t.Fatalf("expected ErrMediaBucketNotConfigured for uploaded cover without bucket, got %v", err)
	}

	media, uploadedKey, err := svc.buildPressMediaModel(11, PressUploadInput{
		FileName: "agenda.pdf",
		MimeType: "application/pdf",
		Content:  []byte("agenda"),
	}, intPtr(7), 0)
	if err != nil || uploadedKey == "" || media.FileURL == "" || media.SortOrder != 0 {
		t.Fatalf("unexpected uploaded media result: media=%#v uploadedKey=%q err=%v", media, uploadedKey, err)
	}

	if _, _, err := svc.buildPressMediaModel(11, PressUploadInput{}, intPtr(7), 0); err == nil {
		t.Fatal("expected missing media file validation error")
	}
	if _, _, err := (&PressService{}).buildPressMediaModel(11, PressUploadInput{
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
	if _, _, err := (&PressService{}).resolveStoredObjectReference("folder/a.pdf", ""); !errors.Is(err, ErrMediaBucketNotConfigured) {
		t.Fatalf("expected ErrMediaBucketNotConfigured, got %v", err)
	}

	svc.deleteStoredObjectBestEffort("", "gs://drive-bucket/main-folder/press-entries/media/a.pdf")
	if len(recorder.deletes) == 0 || recorder.deletes[len(recorder.deletes)-1] != "drive-bucket/main-folder/press-entries/media/a.pdf" {
		t.Fatalf("unexpected helper delete cleanup: %#v", recorder.deletes)
	}
	svc.deleteObjectBestEffort("main-folder/press-entries/media/b.pdf")
	if recorder.deletes[len(recorder.deletes)-1] != "drive-bucket/main-folder/press-entries/media/b.pdf" {
		t.Fatalf("expected direct delete cleanup, got %#v", recorder.deletes)
	}
	beforeDeletes := len(recorder.deletes)
	svc.deleteStoredObjectBestEffort("", "https://example.com/not-storage.pdf")
	if len(recorder.deletes) != beforeDeletes {
		t.Fatalf("expected non-storage delete helper to skip cleanup, got %#v", recorder.deletes)
	}

	if cleanIDs, err := validatePressMediaIDs([]int{3, 2, 1}); err != nil || len(cleanIDs) != 3 {
		t.Fatalf("expected valid media ids, got %#v err=%v", cleanIDs, err)
	}
	if _, err := validatePressMediaIDs(nil); err == nil {
		t.Fatal("expected required media id error")
	}
	if _, err := validatePressMediaIDs([]int{1, 1}); err == nil {
		t.Fatal("expected duplicate media id error")
	}
	if _, err := validatePressMediaIDs([]int{0}); err == nil {
		t.Fatal("expected positive media id error")
	}

	if got := allowedPressSortColumn("title"); got != "title" {
		t.Fatalf("unexpected sort column: %q", got)
	}
	if got := allowedPressSortColumn("created_at"); got != "created_at" {
		t.Fatalf("unexpected created_at sort column: %q", got)
	}
	if got := allowedPressSortColumn("updated_at"); got != "updated_at" {
		t.Fatalf("unexpected updated_at sort column: %q", got)
	}
	if got := allowedPressSortColumn("status"); got != "status" {
		t.Fatalf("unexpected status sort column: %q", got)
	}
	if got := allowedPressSortColumn("visibility"); got != "visibility" {
		t.Fatalf("unexpected visibility sort column: %q", got)
	}
	if got := allowedPressSortColumn("bad"); got != "release_date" {
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

	entry := PressEntry{ID: 9, Title: "Spring Fair", ReleaseDate: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), Status: "published", Visibility: "public"}
	if summary := pressSummaryFromModel(entry, nil); summary.Title != "Spring Fair" {
		t.Fatalf("unexpected summary conversion: %#v", summary)
	}
	if summary := pressSummaryFromModel(entry, nil); summary.Media == nil {
		t.Fatalf("expected summary media to default to empty slice, got %#v", summary)
	}
	if mutation := pressMutationFromModel(entry); mutation.ID != 9 {
		t.Fatalf("unexpected mutation conversion: %#v", mutation)
	}
	if detail := pressDetailFromModel(entry, nil); detail.Media == nil || detail.Title != "Spring Fair" {
		t.Fatalf("unexpected detail conversion: %#v", detail)
	}
	if mediaResp := pressMediaFromModel(PressMedia{ID: 4, DisplayName: "Agenda"}); mediaResp.DisplayName != "Agenda" {
		t.Fatalf("unexpected media conversion: %#v", mediaResp)
	}
	if mediaPtr := pressMediaPtrFromModel(PressMedia{ID: 4, DisplayName: "Agenda"}); mediaPtr == nil || mediaPtr.DisplayName != "Agenda" {
		t.Fatalf("unexpected media pointer conversion: %#v", mediaPtr)
	}

	if (PressEntry{}).TableName() != "press_entries" || (PressMedia{}).TableName() != "press_media" {
		t.Fatal("unexpected table names")
	}
}

func TestNextSortOrderAndResequenceHelpers(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectBegin()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("db.Begin returned error: %v", tx.Error)
	}

	mock.ExpectQuery(`SELECT MAX\(sort_order\) FROM "press_media" WHERE press_entry_id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(3))
	nextSort, err := nextPressMediaSortOrder(tx, 11)
	if err != nil || nextSort != 4 {
		t.Fatalf("expected next sort 4, got %d err=%v", nextSort, err)
	}

	mock.ExpectQuery(`SELECT \* FROM "press_media" WHERE press_entry_id = \$1 ORDER BY sort_order ASC,id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "", "gs://drive-bucket/press-entries/media/agenda.pdf", "application/pdf", 100, "attachment", 0, nil, nil, time.Now(), time.Now(),
		))
	if err := resequencePressMedia(tx, 11); err != nil {
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

	mock.ExpectQuery(`SELECT \* FROM "press_media" WHERE press_entry_id = \$1 ORDER BY sort_order ASC,id ASC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "press_entry_id", "display_name", "file_name", "gcp_object_key", "file_url", "mime_type", "file_size", "media_role", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(
			4, 11, "Agenda", "agenda.pdf", "", "gs://drive-bucket/press-entries/media/agenda.pdf", "application/pdf", 100, "attachment", 2, nil, nil, time.Now(), time.Now(),
		).AddRow(
			5, 11, "Minutes", "minutes.pdf", "", "gs://drive-bucket/press-entries/media/minutes.pdf", "application/pdf", 100, "attachment", 4, nil, nil, time.Now(), time.Now(),
		))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "press_media" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "press_media" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := resequencePressMedia(tx, 11); err != nil {
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

func validSavePressEntryRequest() SavePressEntryRequest {
	return SavePressEntryRequest{
		Title:       "Spring Fair",
		ReleaseDate: "2026-05-01",
		SourceURL:   "https://example.com/press",
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
