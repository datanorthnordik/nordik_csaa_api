package gallery

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
	return db, mock, func() { _ = sqlDB.Close() }
}

func TestGalleryServiceStoreUnavailable(t *testing.T) {
	svc := &GalleryService{}
	if _, err := svc.CreateGallery(SaveGalleryRequest{Name: "A"}, nil); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.UpdateGallery(1, SaveGalleryRequest{Name: "A"}, nil); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ErrStoreUnavailable, got %v", err)
	}
	if err := svc.DeleteGallery(1); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.AddGalleryImages(1, AddGalleryImagesRequest{}, nil); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.DeleteGalleryImages(1, nil); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ErrStoreUnavailable, got %v", err)
	}
}

func TestCreateUpdateDeleteGalleryAndImages(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &GalleryService{DB: db, BucketName: "drive-bucket"}
	restore := stubHooks()
	defer restore()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "galleries"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "galleries" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	resp, err := svc.CreateGallery(SaveGalleryRequest{
		Name:       "Homepage",
		Published:  true,
		CoverImage: &GalleryUploadInput{AltText: "Cover", FileName: "cover.png", MimeType: "image/png", DataBase64: "aGVsbG8="},
	}, intPtr(7))
	if err != nil || resp.ID != 5 {
		t.Fatalf("unexpected create result: resp=%#v err=%v", resp, err)
	}

	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "galleries" WHERE "galleries"."id" = $1 ORDER BY "galleries"."id" LIMIT $2`)).
		WithArgs(5, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "description", "cover_image_url", "cover_image_object_key", "cover_image_alt_text", "published", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(5, "Homepage", "", "gs://drive-bucket/galleries/5/cover/cover.png", "galleries/5/cover/cover.png", "Cover", true, 7, 7, now, now))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "galleries" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	resp, err = svc.UpdateGallery(5, SaveGalleryRequest{Name: "Homepage Updated", Description: "New", RemoveCoverImage: true}, intPtr(9))
	if err != nil || resp.Name != "Homepage Updated" {
		t.Fatalf("unexpected update result: resp=%#v err=%v", resp, err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "galleries" WHERE "galleries"."id" = $1 ORDER BY "galleries"."id" LIMIT $2`)).
		WithArgs(5, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "description", "cover_image_url", "cover_image_object_key", "cover_image_alt_text", "published", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(5, "Homepage", "", "", "", "", true, 7, 7, now, now))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "gallery_images"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectCommit()

	addResp, err := svc.AddGalleryImages(5, AddGalleryImagesRequest{
		Images: []GalleryUploadInput{{Title: "First image", AltText: "Alt", FileName: "a.png", MimeType: "image/png", DataBase64: "aGVsbG8="}},
	}, intPtr(7))
	if err != nil || addResp.DeletedCount != 1 {
		t.Fatalf("unexpected add result: resp=%#v err=%v", addResp, err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "gallery_images" WHERE gallery_id = $1 AND file_url IN ($2,$3)`)).
		WithArgs(5, "gs://drive-bucket/galleries/5/images/a.png", "gs://drive-bucket/galleries/5/images/b.png").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "gallery_id", "title", "alt_text", "gcp_object_key", "file_url", "mime_type", "file_size", "uploaded_by", "created_at", "updated_at",
		}).AddRow(11, 5, "First image", "Alt", "galleries/5/images/a.png", "gs://drive-bucket/galleries/5/images/a.png", "image/png", 5, 7, now, now).
			AddRow(12, 5, "Second image", "Alt2", "galleries/5/images/b.png", "gs://drive-bucket/galleries/5/images/b.png", "image/png", 5, 7, now, now))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "gallery_images" WHERE "gallery_images"."id" IN ($1,$2)`)).
		WithArgs(11, 12).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	delResp, err := svc.DeleteGalleryImages(5, []string{"gs://drive-bucket/galleries/5/images/a.png", "gs://drive-bucket/galleries/5/images/b.png"})
	if err != nil || delResp.DeletedCount != 2 {
		t.Fatalf("unexpected delete images result: resp=%#v err=%v", delResp, err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "galleries" WHERE "galleries"."id" = $1 ORDER BY "galleries"."id" LIMIT $2`)).
		WithArgs(5, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "description", "cover_image_url", "cover_image_object_key", "cover_image_alt_text", "published", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(5, "Homepage", "", "gs://drive-bucket/galleries/5/cover/cover.png", "galleries/5/cover/cover.png", "Cover", true, 7, 7, now, now))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "gallery_images" WHERE gallery_id = $1`)).
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "gallery_id", "title", "alt_text", "gcp_object_key", "file_url", "mime_type", "file_size", "uploaded_by", "created_at", "updated_at",
		}).AddRow(13, 5, "First image", "Alt", "galleries/5/images/a.png", "gs://drive-bucket/galleries/5/images/a.png", "image/png", 5, 7, now, now))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "galleries" WHERE "galleries"."id" = $1`)).
		WithArgs(5).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := svc.DeleteGallery(5); err != nil {
		t.Fatalf("DeleteGallery returned error: %v", err)
	}
}

func TestGalleryValidationAndErrors(t *testing.T) {
	if _, err := normalizeSaveGalleryRequest(SaveGalleryRequest{}); err == nil {
		t.Fatal("expected name validation error")
	}
	if _, err := normalizeAddGalleryImagesRequest(AddGalleryImagesRequest{}); err == nil {
		t.Fatal("expected images validation error")
	}
	if _, err := normalizeAddGalleryImagesRequest(AddGalleryImagesRequest{
		Images: []GalleryUploadInput{{FileName: "doc.pdf", MimeType: "application/pdf", DataBase64: "aGVsbG8="}},
	}); err == nil {
		t.Fatal("expected image-only validation error")
	}

	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	svc := &GalleryService{DB: db, BucketName: "drive-bucket"}
	restore := stubHooks()
	defer restore()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "galleries" WHERE "galleries"."id" = $1 ORDER BY "galleries"."id" LIMIT $2`)).
		WithArgs(99, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectRollback()
	if _, err := svc.UpdateGallery(99, SaveGalleryRequest{Name: "Missing"}, nil); !errors.Is(err, ErrGalleryNotFound) {
		t.Fatalf("expected ErrGalleryNotFound, got %v", err)
	}

	if _, err := svc.DeleteGalleryImages(1, nil); err == nil {
		t.Fatal("expected storage url validation error")
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "gallery_images" WHERE gallery_id = $1 AND file_url IN ($2)`)).
		WithArgs(1, "gs://drive-bucket/galleries/1/images/missing.png").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()
	if _, err := svc.DeleteGalleryImages(1, []string{"gs://drive-bucket/galleries/1/images/missing.png"}); !errors.Is(err, ErrGalleryImageNotFound) {
		t.Fatalf("expected ErrGalleryImageNotFound, got %v", err)
	}
}

func stubHooks() func() {
	prevUpload := uploadBase64ToGCSHook
	prevDelete := deleteGCSObjectHook
	prevNow := nowFunc
	uploadBase64ToGCSHook = func(base64Data, bucketName, objectName, contentType string) (string, int64, error) {
		return "gs://" + bucketName + "/" + objectName, int64(len(base64Data)), nil
	}
	deleteGCSObjectHook = func(bucketName, objectName string) error { return nil }
	nowFunc = func() time.Time { return time.Date(2026, 5, 11, 14, 25, 21, 0, time.UTC) }
	return func() {
		uploadBase64ToGCSHook = prevUpload
		deleteGCSObjectHook = prevDelete
		nowFunc = prevNow
	}
}

func intPtr(v int) *int { return &v }
