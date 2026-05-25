package memorial

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"nordikcsaaapi/internal/util"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMemorialServiceAdditionalCreateBranches(t *testing.T) {
	validReq := SaveMemorialRequest{
		FullName: "Ada Lovelace",
		Category: MemorialCategoryFounder,
		Status:   MemorialStatusDraft,
	}

	t.Run("validation error short circuits before opening a transaction", func(t *testing.T) {
		db, _, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		if _, err := svc.CreateMemorial(SaveMemorialRequest{
			Category: MemorialCategoryFounder,
			Status:   MemorialStatusDraft,
		}, nil); err == nil || !strings.Contains(err.Error(), "full_name is required") {
			t.Fatalf("expected full_name validation error, got %v", err)
		}
	})

	t.Run("begin error", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		mock.ExpectBegin().WillReturnError(errors.New("begin failed"))

		if _, err := svc.CreateMemorial(validReq, nil); err == nil || err.Error() != "begin failed" {
			t.Fatalf("expected begin failure, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("entry insert error rolls back", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "memorial_entries"`)).
			WillReturnError(errors.New("insert failed"))
		mock.ExpectRollback()

		if _, err := svc.CreateMemorial(validReq, nil); err == nil || err.Error() != "insert failed" {
			t.Fatalf("expected insert failure, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("save error rolls back", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "memorial_entries"`)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(21))
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "memorial_entries" SET`)).
			WillReturnError(errors.New("save failed"))
		mock.ExpectRollback()

		if _, err := svc.CreateMemorial(validReq, nil); err == nil || err.Error() != "save failed" {
			t.Fatalf("expected save failure, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("commit error is returned", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "memorial_entries"`)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(22))
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "memorial_entries" SET`)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit().WillReturnError(errors.New("commit failed"))

		if _, err := svc.CreateMemorial(validReq, nil); err == nil || err.Error() != "commit failed" {
			t.Fatalf("expected commit failure, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})
}

func TestMemorialServiceAdditionalUpdateBranches(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	validReq := SaveMemorialRequest{
		FullName: "Ada Lovelace Updated",
		Category: MemorialCategoryFriend,
		Status:   MemorialStatusReview,
	}

	t.Run("validation error short circuits before opening a transaction", func(t *testing.T) {
		db, _, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		if _, err := svc.UpdateMemorial(11, SaveMemorialRequest{
			FullName: "Ada Lovelace",
			Category: MemorialCategoryFounder,
			Status:   MemorialStatusDraft,
			DateOfBirth: "2026-02-01",
			DateOfPassing: "2026-01-01",
		}, nil); err == nil || !strings.Contains(err.Error(), "date_of_passing must be on or after date_of_birth") {
			t.Fatalf("expected date validation error, got %v", err)
		}
	})

	t.Run("begin error", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		mock.ExpectBegin().WillReturnError(errors.New("begin failed"))

		if _, err := svc.UpdateMemorial(11, validReq, nil); err == nil || err.Error() != "begin failed" {
			t.Fatalf("expected begin failure, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("lookup error rolls back", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnError(errors.New("lookup failed"))
		mock.ExpectRollback()

		if _, err := svc.UpdateMemorial(11, validReq, nil); err == nil || err.Error() != "lookup failed" {
			t.Fatalf("expected lookup failure, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("next gallery sort order error rolls back", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnRows(memorialEntryRows().AddRow(
				11, "Ada Lovelace", "Analytical Engine", MemorialCategoryFounder, MemorialStatusDraft, "<p>Hello</p>",
				nil, nil, nil, "", "", "", "", 0, 7, 7, now, now,
			))
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(sort_order\), -1\) AS max_sort_order FROM "memorial_gallery_images" WHERE memorial_entry_id = \$1`).
			WillReturnError(errors.New("sort order failed"))
		mock.ExpectRollback()

		if _, err := svc.UpdateMemorial(11, SaveMemorialRequest{
			FullName: "Ada Lovelace",
			Category: MemorialCategoryFounder,
			Status:   MemorialStatusDraft,
			GalleryImages: []MemorialUploadInput{
				{FileName: "gallery-new.png", MimeType: "image/png", Content: []byte("gallery")},
			},
		}, nil); err == nil || err.Error() != "sort order failed" {
			t.Fatalf("expected sort order failure, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("save error rolls back", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnRows(memorialEntryRows().AddRow(
				11, "Ada Lovelace", "Analytical Engine", MemorialCategoryFounder, MemorialStatusDraft, "<p>Hello</p>",
				nil, nil, nil, "", "", "", "", 0, 7, 7, now, now,
			))
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "memorial_entries" SET`)).
			WillReturnError(errors.New("save failed"))
		mock.ExpectRollback()

		if _, err := svc.UpdateMemorial(11, validReq, nil); err == nil || err.Error() != "save failed" {
			t.Fatalf("expected save failure, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("commit error is returned", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnRows(memorialEntryRows().AddRow(
				11, "Ada Lovelace", "Analytical Engine", MemorialCategoryFounder, MemorialStatusDraft, "<p>Hello</p>",
				nil, nil, nil, "", "", "", "", 0, 7, 7, now, now,
			))
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "memorial_entries" SET`)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit().WillReturnError(errors.New("commit failed"))

		if _, err := svc.UpdateMemorial(11, validReq, nil); err == nil || err.Error() != "commit failed" {
			t.Fatalf("expected commit failure, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})
}

func TestMemorialServiceAdditionalDeleteBranches(t *testing.T) {
	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)

	t.Run("begin error", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		mock.ExpectBegin().WillReturnError(errors.New("begin failed"))

		if err := svc.DeleteMemorial(11); err == nil || err.Error() != "begin failed" {
			t.Fatalf("expected begin failure, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("gallery lookup error rolls back", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnRows(memorialEntryRows().AddRow(
				11, "Ada Lovelace", "Analytical Engine", MemorialCategoryFounder, MemorialStatusDraft, "<p>Hello</p>",
				nil, nil, nil, "", "", "", "", 0, 7, 7, now, now,
			))
		mock.ExpectQuery(`SELECT \* FROM "memorial_gallery_images" WHERE memorial_entry_id = \$1`).
			WillReturnError(errors.New("gallery lookup failed"))
		mock.ExpectRollback()

		if err := svc.DeleteMemorial(11); err == nil || err.Error() != "gallery lookup failed" {
			t.Fatalf("expected gallery lookup failure, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("entry delete error rolls back", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnRows(memorialEntryRows().AddRow(
				11, "Ada Lovelace", "Analytical Engine", MemorialCategoryFounder, MemorialStatusDraft, "<p>Hello</p>",
				nil, nil, nil, "", "", "", "", 0, 7, 7, now, now,
			))
		mock.ExpectQuery(`SELECT \* FROM "memorial_gallery_images" WHERE memorial_entry_id = \$1`).
			WillReturnRows(memorialGalleryRows())
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "memorial_entries" WHERE "memorial_entries"."id" = $1`)).
			WillReturnError(errors.New("delete failed"))
		mock.ExpectRollback()

		if err := svc.DeleteMemorial(11); err == nil || err.Error() != "delete failed" {
			t.Fatalf("expected delete failure, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("commit error is returned", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnRows(memorialEntryRows().AddRow(
				11, "Ada Lovelace", "Analytical Engine", MemorialCategoryFounder, MemorialStatusDraft, "<p>Hello</p>",
				nil, nil, nil, "", "", "", "", 0, 7, 7, now, now,
			))
		mock.ExpectQuery(`SELECT \* FROM "memorial_gallery_images" WHERE memorial_entry_id = \$1`).
			WillReturnRows(memorialGalleryRows())
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "memorial_entries" WHERE "memorial_entries"."id" = $1`)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit().WillReturnError(errors.New("commit failed"))

		if err := svc.DeleteMemorial(11); err == nil || err.Error() != "commit failed" {
			t.Fatalf("expected commit failure, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})
}

func TestMemorialServiceAdditionalMediaAndHelperBranches(t *testing.T) {
	t.Run("portrait and gallery content fall back to object key filenames and octet-stream", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		previousDownload := downloadGCSObjectHook
		t.Cleanup(func() {
			downloadGCSObjectHook = previousDownload
		})
		downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
			return []byte("bits"), "", nil
		}

		svc := &MemorialService{DB: db, BucketName: "drive-bucket"}
		now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnRows(memorialEntryRows().AddRow(
				11, "Ada Lovelace", "Analytical Engine", MemorialCategoryFounder, MemorialStatusDraft, "<p>Hello</p>",
				nil, nil, nil,
				"", "memorial/entry-11/portrait/generated.jpg", "gs://drive-bucket/memorial/entry-11/portrait/generated.jpg", "", 0,
				7, 7, now, now,
			))
		portraitResp, err := svc.GetMemorialPortraitContent(11)
		if err != nil {
			t.Fatalf("GetMemorialPortraitContent returned error: %v", err)
		}
		if portraitResp.FileName != "generated.jpg" || portraitResp.ContentType != "application/octet-stream" {
			t.Fatalf("unexpected portrait fallback response: %#v", portraitResp)
		}

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_gallery_images" WHERE memorial_entry_id = $1 AND id = $2 ORDER BY "memorial_gallery_images"."id" LIMIT $3`)).
			WillReturnRows(memorialGalleryRows().AddRow(
				31, 11, "", "memorial/entry-11/gallery/generated.png", "gs://drive-bucket/memorial/entry-11/gallery/generated.png",
				"", 2048, 0, 7, now, now,
			))
		galleryResp, err := svc.GetMemorialGalleryImageContent(11, 31)
		if err != nil {
			t.Fatalf("GetMemorialGalleryImageContent returned error: %v", err)
		}
		if galleryResp.FileName != "generated.png" || galleryResp.ContentType != "application/octet-stream" {
			t.Fatalf("unexpected gallery fallback response: %#v", galleryResp)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("helper branches cover image validation and download errors", func(t *testing.T) {
		if err := validateImageUploadInput(MemorialUploadInput{
			FileName: "portrait.webp",
			Content:  []byte("portrait"),
		}); err != nil {
			t.Fatalf("expected extension-based image validation success, got %v", err)
		}
		if err := validateImageUploadInput(MemorialUploadInput{
			FileName: "stored.bin",
			FileURL:  "gs://drive-bucket/memorial/stored.bin",
		}); err != nil {
			t.Fatalf("expected stored file references to be accepted, got %v", err)
		}
		if err := validateImageUploadInput(MemorialUploadInput{
			FileName: "unknown.bin",
			Content:  []byte("raw"),
		}); err == nil || err.Error() != "only image uploads are supported" {
			t.Fatalf("expected unknown binary uploads to be rejected, got %v", err)
		}

		if !isAllowedMemorialStatus(MemorialStatusReview) {
			t.Fatal("expected review status to be allowed")
		}

		previousDownload := downloadGCSObjectHook
		t.Cleanup(func() {
			downloadGCSObjectHook = previousDownload
		})
		downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
			return nil, "", errors.New("download failed")
		}

		svc := &MemorialService{BucketName: "drive-bucket"}
		if _, _, err := svc.downloadStoredObject(storedObjectRef{
			ObjectKey: "memorial/entry-9/gallery/file.jpg",
		}); err == nil || err.Error() != "download failed" {
			t.Fatalf("expected generic download errors to bubble up, got %v", err)
		}
	})

	t.Run("resequence gallery images no-op when items are already ordered", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
		mock.ExpectQuery(`SELECT \* FROM "memorial_gallery_images" WHERE memorial_entry_id = \$1 ORDER BY sort_order ASC,id ASC`).
			WillReturnRows(memorialGalleryRows().
				AddRow(31, 11, "gallery-one.png", "memorial/entry-11/gallery/one.png", "gs://drive-bucket/memorial/entry-11/gallery/one.png", "image/png", 2048, 0, 7, now, now).
				AddRow(32, 11, "gallery-two.png", "memorial/entry-11/gallery/two.png", "gs://drive-bucket/memorial/entry-11/gallery/two.png", "image/png", 4096, 1, 7, now, now))

		svc := &MemorialService{}
		if err := svc.resequenceGalleryImages(db, 11); err != nil {
			t.Fatalf("expected resequenceGalleryImages no-op success, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("resolve stored object references without a configured bucket rejects incomplete references", func(t *testing.T) {
		svc := &MemorialService{}
		if _, _, err := svc.resolveStoredObjectReference("", "memorial/entry-9/portrait/file.jpg"); err == nil || !strings.Contains(err.Error(), "not available from storage") {
			t.Fatalf("expected unavailable storage reference error, got %v", err)
		}
	})

	t.Run("download object not found continues to map correctly", func(t *testing.T) {
		previousDownload := downloadGCSObjectHook
		t.Cleanup(func() {
			downloadGCSObjectHook = previousDownload
		})
		downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
			return nil, "", util.ErrObjectNotFound
		}

		svc := &MemorialService{BucketName: "drive-bucket"}
		if _, _, err := svc.downloadStoredObject(storedObjectRef{
			ObjectKey: "memorial/entry-9/gallery/file.jpg",
		}); !errors.Is(err, ErrMemorialMediaNotFound) {
			t.Fatalf("expected ErrMemorialMediaNotFound, got %v", err)
		}
	})
}
