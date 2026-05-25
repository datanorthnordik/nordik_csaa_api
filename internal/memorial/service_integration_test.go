package memorial

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"nordikcsaaapi/internal/util"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupMemorialMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
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

func memorialEntryRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "full_name", "affiliation", "category", "status", "biography",
		"date_of_birth", "date_of_passing", "published_at",
		"portrait_file_name", "portrait_gcp_object_key", "portrait_file_url", "portrait_mime_type", "portrait_file_size",
		"created_by", "updated_by", "created_at", "updated_at",
	})
}

func memorialGalleryRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "memorial_entry_id", "file_name", "gcp_object_key", "file_url",
		"mime_type", "file_size", "sort_order", "uploaded_by", "created_at", "updated_at",
	})
}

func containsMemorialString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestMemorialServiceListMemorials(t *testing.T) {
	db, mock, cleanup := setupMemorialMockDB(t)
	defer cleanup()

	svc := &MemorialService{DB: db}
	createdAt := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(2 * time.Hour)

	mock.ExpectQuery(`SELECT category, COUNT\(\*\) AS count FROM "memorial_entries" GROUP BY "category"`).
		WillReturnRows(sqlmock.NewRows([]string{"category", "count"}).
			AddRow(MemorialCategoryFriend, 1))
	mock.ExpectQuery(`SELECT status, COUNT\(\*\) AS count FROM "memorial_entries" GROUP BY "category","status"`).
		WillReturnRows(sqlmock.NewRows([]string{"status", "count"}).
			AddRow(MemorialStatusPublished, 1))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "memorial_entries"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT \* FROM "memorial_entries" ORDER BY "updated_at" DESC,"id" DESC LIMIT`).
		WillReturnRows(memorialEntryRows().AddRow(
			11,
			"Ada Lovelace",
			"Analytical Engine",
			MemorialCategoryFriend,
			MemorialStatusPublished,
			"<p>Remembered</p>",
			time.Date(1815, 12, 10, 0, 0, 0, 0, time.UTC),
			time.Date(1852, 11, 27, 0, 0, 0, 0, time.UTC),
			updatedAt,
			"portrait.jpg",
			"memorial/entry-11/portrait/file.jpg",
			"gs://drive-bucket/memorial/entry-11/portrait/file.jpg",
			"image/jpeg",
			1024,
			7,
			7,
			createdAt,
			updatedAt,
		))

	resp, err := svc.ListMemorials(ListMemorialsFilter{
		Page:     1,
		PageSize: 10,
		Status:   "all",
	})
	if err != nil {
		t.Fatalf("ListMemorials returned error: %v", err)
	}

	if len(resp.Items) != 1 || resp.Items[0].FullName != "Ada Lovelace" {
		t.Fatalf("unexpected list response: %#v", resp)
	}
	if resp.Items[0].PortraitContentURL != buildMemorialPortraitContentURL(11) {
		t.Fatalf("expected portrait content url, got %#v", resp.Items[0])
	}
	if resp.Pagination.TotalItems != 1 || resp.Pagination.HasNext || resp.Pagination.TotalPages != 1 {
		t.Fatalf("unexpected pagination: %#v", resp.Pagination)
	}
	if resp.Summary.CategoryCounts[3].Count != 1 || resp.Summary.StatusCounts[0].Count != 1 {
		t.Fatalf("unexpected summary counts: %#v", resp.Summary)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestMemorialServiceListMemorialsWithFilters(t *testing.T) {
	db, mock, cleanup := setupMemorialMockDB(t)
	defer cleanup()

	svc := &MemorialService{DB: db}

	mock.ExpectQuery(`LOWER\(COALESCE\(full_name, ''\)\) LIKE \$1 OR LOWER\(COALESCE\(affiliation, ''\)\) LIKE \$2 OR LOWER\(COALESCE\(biography, ''\)\) LIKE \$3`).
		WillReturnRows(sqlmock.NewRows([]string{"category", "count"}).
			AddRow(MemorialCategoryFounder, 1))
	mock.ExpectQuery(`GROUP BY "category","status"`).
		WillReturnRows(sqlmock.NewRows([]string{"status", "count"}).
			AddRow(MemorialStatusReview, 1))
	mock.ExpectQuery(`WHERE \(LOWER\(COALESCE\(full_name, ''\)\) LIKE \$1 OR LOWER\(COALESCE\(affiliation, ''\)\) LIKE \$2 OR LOWER\(COALESCE\(biography, ''\)\) LIKE \$3\) AND status = \$4 AND category = \$5`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`WHERE \(LOWER\(COALESCE\(full_name, ''\)\) LIKE \$1 OR LOWER\(COALESCE\(affiliation, ''\)\) LIKE \$2 OR LOWER\(COALESCE\(biography, ''\)\) LIKE \$3\) AND status = \$4 AND category = \$5 ORDER BY "updated_at" DESC,"id" DESC LIMIT`).
		WillReturnRows(memorialEntryRows().AddRow(
			12, "Grace Hopper", "Navy", MemorialCategoryFounder, MemorialStatusReview, "<p>COBOL pioneer</p>",
			nil, nil, nil,
			"", "", "", "", 0,
			7, 7, time.Now(), time.Now(),
		))

	resp, err := svc.ListMemorials(ListMemorialsFilter{
		Page:       1,
		PageSize:   5,
		SearchTerm: "grace",
		Status:     MemorialStatusReview,
		Category:   MemorialCategoryFounder,
	})
	if err != nil {
		t.Fatalf("ListMemorials with filters returned error: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].FullName != "Grace Hopper" {
		t.Fatalf("unexpected filtered list response: %#v", resp)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestMemorialServiceGetAndMediaContent(t *testing.T) {
	t.Run("get memorial detail", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		createdAt := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
		updatedAt := createdAt.Add(2 * time.Hour)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnRows(memorialEntryRows().AddRow(
				11, "Ada Lovelace", "Analytical Engine", MemorialCategoryFounder, MemorialStatusDraft, "<p>Hello</p>",
				time.Date(1815, 12, 10, 0, 0, 0, 0, time.UTC), nil, nil,
				"portrait.jpg", "memorial/entry-11/portrait/file.jpg", "gs://drive-bucket/memorial/entry-11/portrait/file.jpg", "image/jpeg", 1024,
				7, 7, createdAt, updatedAt,
			))
		mock.ExpectQuery(`SELECT \* FROM "memorial_gallery_images" WHERE memorial_entry_id = \$1 ORDER BY sort_order ASC,id ASC`).
			WillReturnRows(memorialGalleryRows().
				AddRow(31, 11, "gallery-one.png", "memorial/entry-11/gallery/one.png", "gs://drive-bucket/memorial/entry-11/gallery/one.png", "image/png", 2048, 0, 7, createdAt, updatedAt).
				AddRow(32, 11, "gallery-two.png", "memorial/entry-11/gallery/two.png", "gs://drive-bucket/memorial/entry-11/gallery/two.png", "image/png", 4096, 1, 7, createdAt, updatedAt))

		resp, err := svc.GetMemorial(11)
		if err != nil {
			t.Fatalf("GetMemorial returned error: %v", err)
		}
		if resp.Portrait == nil || len(resp.GalleryImages) != 2 {
			t.Fatalf("expected portrait and gallery images, got %#v", resp)
		}
		if resp.GalleryImages[0].ContentURL != buildMemorialGalleryImageContentURL(11, 31) {
			t.Fatalf("unexpected gallery content url: %#v", resp.GalleryImages[0])
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("portrait content success and not found", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db, BucketName: "drive-bucket"}
		createdAt := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
		updatedAt := createdAt.Add(2 * time.Hour)

		previousDownload := downloadGCSObjectHook
		t.Cleanup(func() {
			downloadGCSObjectHook = previousDownload
		})
		downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
			if objectName == "missing" {
				return nil, "", util.ErrObjectNotFound
			}
			return []byte("portrait-bits"), "", nil
		}

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnRows(memorialEntryRows().AddRow(
				11, "Ada Lovelace", "Analytical Engine", MemorialCategoryFounder, MemorialStatusDraft, "<p>Hello</p>",
				nil, nil, nil,
				"portrait.jpg", "memorial/entry-11/portrait/file.jpg", "gs://drive-bucket/memorial/entry-11/portrait/file.jpg", "image/jpeg", 1024,
				7, 7, createdAt, updatedAt,
			))

		resp, err := svc.GetMemorialPortraitContent(11)
		if err != nil {
			t.Fatalf("GetMemorialPortraitContent returned error: %v", err)
		}
		if string(resp.Content) != "portrait-bits" || resp.ContentType != "image/jpeg" || resp.FileName != "portrait.jpg" {
			t.Fatalf("unexpected portrait content: %#v", resp)
		}

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnRows(memorialEntryRows().AddRow(
				12, "Ada Lovelace", "Analytical Engine", MemorialCategoryFounder, MemorialStatusDraft, "<p>Hello</p>",
				nil, nil, nil,
				"", "", "", "", 0,
				7, 7, createdAt, updatedAt,
			))
		if _, err := svc.GetMemorialPortraitContent(12); !errors.Is(err, ErrMemorialMediaNotFound) {
			t.Fatalf("expected ErrMemorialMediaNotFound, got %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("gallery content success and not found", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db, BucketName: "drive-bucket"}

		previousDownload := downloadGCSObjectHook
		t.Cleanup(func() {
			downloadGCSObjectHook = previousDownload
		})
		downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
			return []byte("gallery-bits"), "image/png", nil
		}

		now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_gallery_images" WHERE memorial_entry_id = $1 AND id = $2 ORDER BY "memorial_gallery_images"."id" LIMIT $3`)).
			WillReturnRows(memorialGalleryRows().AddRow(
				31, 11, "gallery.png", "memorial/entry-11/gallery/file.png", "gs://drive-bucket/memorial/entry-11/gallery/file.png",
				"image/png", 2048, 0, 7, now, now,
			))

		resp, err := svc.GetMemorialGalleryImageContent(11, 31)
		if err != nil {
			t.Fatalf("GetMemorialGalleryImageContent returned error: %v", err)
		}
		if string(resp.Content) != "gallery-bits" || resp.FileName != "gallery.png" {
			t.Fatalf("unexpected gallery content: %#v", resp)
		}

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_gallery_images" WHERE memorial_entry_id = $1 AND id = $2 ORDER BY "memorial_gallery_images"."id" LIMIT $3`)).
			WillReturnRows(memorialGalleryRows())
		if _, err := svc.GetMemorialGalleryImageContent(11, 99); !errors.Is(err, ErrMemorialMediaNotFound) {
			t.Fatalf("expected ErrMemorialMediaNotFound, got %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})
}

func TestMemorialServiceCreateUpdateAndDelete(t *testing.T) {
	t.Run("create memorial", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		baseTime := time.Date(2026, 5, 25, 10, 30, 0, 0, time.UTC)
		previousNow := memorialNowFunc
		previousUpload := uploadBytesToGCSHook
		t.Cleanup(func() {
			memorialNowFunc = previousNow
			uploadBytesToGCSHook = previousUpload
		})
		memorialNowFunc = func() time.Time { return baseTime }

		var uploadedObjectKey string
		uploadBytesToGCSHook = func(data []byte, bucketName, objectName, contentType string) (string, int64, error) {
			uploadedObjectKey = objectName
			return "gs://" + bucketName + "/" + objectName, int64(len(data)), nil
		}

		svc := &MemorialService{DB: db, BucketName: "drive-bucket", BucketPrefix: "cms"}
		userID := 7

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "memorial_entries"`)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "memorial_entries" SET`)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		resp, err := svc.CreateMemorial(SaveMemorialRequest{
			FullName: "Ada Lovelace",
			Category: MemorialCategoryFounder,
			Status:   MemorialStatusDraft,
			Portrait: &MemorialUploadInput{
				FileName: "Portrait Final.JPG",
				MimeType: "image/jpeg",
				Content:  []byte("portrait-v1"),
			},
		}, &userID)
		if err != nil {
			t.Fatalf("CreateMemorial returned error: %v", err)
		}
		if resp.ID != 11 || resp.Status != MemorialStatusDraft {
			t.Fatalf("unexpected create response: %#v", resp)
		}
		if !strings.Contains(uploadedObjectKey, "cms/memorial/entry-11/portrait/") {
			t.Fatalf("expected uploaded portrait object key, got %q", uploadedObjectKey)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("create memorial with gallery uploads", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		previousNow := memorialNowFunc
		previousUpload := uploadBytesToGCSHook
		t.Cleanup(func() {
			memorialNowFunc = previousNow
			uploadBytesToGCSHook = previousUpload
		})

		nowBase := time.Date(2026, 5, 25, 10, 30, 0, 0, time.UTC)
		nowCall := 0
		memorialNowFunc = func() time.Time {
			current := nowBase.Add(time.Duration(nowCall) * time.Second)
			nowCall++
			return current
		}
		uploadBytesToGCSHook = func(data []byte, bucketName, objectName, contentType string) (string, int64, error) {
			return "gs://" + bucketName + "/" + objectName, int64(len(data)), nil
		}

		svc := &MemorialService{DB: db, BucketName: "drive-bucket", BucketPrefix: "cms"}
		userID := 7

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "memorial_entries"`)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(12))
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "memorial_gallery_images"`)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(41))
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "memorial_gallery_images"`)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "memorial_entries" SET`)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		resp, err := svc.CreateMemorial(SaveMemorialRequest{
			FullName: "Grace Hopper",
			Category: MemorialCategoryFounder,
			Status:   MemorialStatusDraft,
			GalleryImages: []MemorialUploadInput{
				{FileName: "gallery-one.png", MimeType: "image/png", Content: []byte("one")},
				{FileName: "gallery-two.png", MimeType: "image/png", Content: []byte("two")},
			},
		}, &userID)
		if err != nil {
			t.Fatalf("CreateMemorial with gallery uploads returned error: %v", err)
		}
		if resp.ID != 12 {
			t.Fatalf("unexpected create response: %#v", resp)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("update memorial with media mutations", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		baseTime := time.Date(2026, 5, 25, 10, 30, 0, 0, time.UTC)
		nowCall := 0
		previousNow := memorialNowFunc
		previousUpload := uploadBytesToGCSHook
		previousDelete := deleteGCSObjectHook
		t.Cleanup(func() {
			memorialNowFunc = previousNow
			uploadBytesToGCSHook = previousUpload
			deleteGCSObjectHook = previousDelete
		})
		memorialNowFunc = func() time.Time {
			current := baseTime.Add(time.Duration(nowCall) * time.Second)
			nowCall++
			return current
		}

		deletedKeys := make([]string, 0)
		uploadBytesToGCSHook = func(data []byte, bucketName, objectName, contentType string) (string, int64, error) {
			return "gs://" + bucketName + "/" + objectName, int64(len(data)), nil
		}
		deleteGCSObjectHook = func(bucketName, objectName string) error {
			deletedKeys = append(deletedKeys, bucketName+":"+objectName)
			return nil
		}

		svc := &MemorialService{DB: db, BucketName: "drive-bucket", BucketPrefix: "cms"}
		userID := 7
		createdAt := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
		updatedAt := createdAt.Add(2 * time.Hour)

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnRows(memorialEntryRows().AddRow(
				11, "Ada Lovelace", "Analytical Engine", MemorialCategoryFounder, MemorialStatusDraft, "<p>Hello</p>",
				time.Date(1815, 12, 10, 0, 0, 0, 0, time.UTC), nil, nil,
				"portrait-v1.jpg", "memorial/entry-11/portrait/old.jpg", "gs://drive-bucket/memorial/entry-11/portrait/old.jpg", "image/jpeg", 1024,
				7, 7, createdAt, updatedAt,
			))
		mock.ExpectQuery(`SELECT \* FROM "memorial_gallery_images" WHERE memorial_entry_id = \$1 AND id IN \(\$2\)`).
			WillReturnRows(memorialGalleryRows().AddRow(
				31, 11, "gallery-old.png", "memorial/entry-11/gallery/old.png", "gs://drive-bucket/memorial/entry-11/gallery/old.png",
				"image/png", 2048, 0, 7, createdAt, updatedAt,
			))
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "memorial_gallery_images" WHERE "memorial_gallery_images"."id" = $1`)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(`SELECT \* FROM "memorial_gallery_images" WHERE memorial_entry_id = \$1 ORDER BY sort_order ASC,id ASC`).
			WillReturnRows(memorialGalleryRows().AddRow(
				32, 11, "gallery-keep.png", "memorial/entry-11/gallery/keep.png", "gs://drive-bucket/memorial/entry-11/gallery/keep.png",
				"image/png", 4096, 3, 7, createdAt, updatedAt,
			))
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "memorial_gallery_images" SET`)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(sort_order\), -1\) AS max_sort_order FROM "memorial_gallery_images" WHERE memorial_entry_id = \$1`).
			WillReturnRows(sqlmock.NewRows([]string{"max_sort_order"}).AddRow(0))
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "memorial_gallery_images"`)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(40))
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "memorial_entries" SET`)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		resp, err := svc.UpdateMemorial(11, SaveMemorialRequest{
			FullName:       "Ada Lovelace Updated",
			Affiliation:    "The Analytical Engine",
			Category:       MemorialCategoryFriend,
			Status:         MemorialStatusPublished,
			Biography:      "<p>Updated biography</p>",
			DateOfBirth:    "1815-12-10",
			DateOfPassing:  "1852-11-27",
			RemovePortrait: true,
			Portrait: &MemorialUploadInput{
				FileName: "portrait-v2.png",
				MimeType: "image/png",
				Content:  []byte("portrait-v2"),
			},
			GalleryImages: []MemorialUploadInput{
				{FileName: "gallery-new.gif", MimeType: "image/gif", Content: []byte("gallery-new")},
			},
			RemoveGalleryImageIDs: []int{31},
		}, &userID)
		if err != nil {
			t.Fatalf("UpdateMemorial returned error: %v", err)
		}
		if resp.ID != 11 || resp.Status != MemorialStatusPublished {
			t.Fatalf("unexpected update response: %#v", resp)
		}
		if !containsMemorialString(deletedKeys, "drive-bucket:memorial/entry-11/portrait/old.jpg") ||
			!containsMemorialString(deletedKeys, "drive-bucket:memorial/entry-11/gallery/old.png") {
			t.Fatalf("expected old objects to be cleaned up, got %#v", deletedKeys)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("update memorial not found", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnRows(memorialEntryRows())
		mock.ExpectRollback()

		if _, err := svc.UpdateMemorial(99, SaveMemorialRequest{
			FullName: "Missing",
			Category: MemorialCategoryFounder,
			Status:   MemorialStatusDraft,
		}, nil); !errors.Is(err, ErrMemorialNotFound) {
			t.Fatalf("expected ErrMemorialNotFound, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("create memorial upload failure rolls back", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		previousUpload := uploadBytesToGCSHook
		t.Cleanup(func() {
			uploadBytesToGCSHook = previousUpload
		})
		uploadBytesToGCSHook = func(data []byte, bucketName, objectName, contentType string) (string, int64, error) {
			return "", 0, errors.New("upload failed")
		}

		svc := &MemorialService{DB: db, BucketName: "drive-bucket"}
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "memorial_entries"`)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(13))
		mock.ExpectRollback()

		if _, err := svc.CreateMemorial(SaveMemorialRequest{
			FullName: "Upload Failure",
			Category: MemorialCategoryFounder,
			Status:   MemorialStatusDraft,
			Portrait: &MemorialUploadInput{
				FileName: "portrait.jpg",
				MimeType: "image/jpeg",
				Content:  []byte("portrait"),
			},
		}, nil); err == nil || err.Error() != "upload failed" {
			t.Fatalf("expected upload failure, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("update memorial insert failure cleans uploaded object", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		previousNow := memorialNowFunc
		previousUpload := uploadBytesToGCSHook
		previousDelete := deleteGCSObjectHook
		t.Cleanup(func() {
			memorialNowFunc = previousNow
			uploadBytesToGCSHook = previousUpload
			deleteGCSObjectHook = previousDelete
		})
		memorialNowFunc = func() time.Time { return time.Date(2026, 5, 25, 10, 30, 0, 0, time.UTC) }

		deletedKeys := make([]string, 0)
		uploadBytesToGCSHook = func(data []byte, bucketName, objectName, contentType string) (string, int64, error) {
			return "gs://" + bucketName + "/" + objectName, int64(len(data)), nil
		}
		deleteGCSObjectHook = func(bucketName, objectName string) error {
			deletedKeys = append(deletedKeys, bucketName+":"+objectName)
			return nil
		}

		svc := &MemorialService{DB: db, BucketName: "drive-bucket"}
		now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnRows(memorialEntryRows().AddRow(
				11, "Ada Lovelace", "Analytical Engine", MemorialCategoryFounder, MemorialStatusDraft, "<p>Hello</p>",
				nil, nil, nil, "", "", "", "", 0, 7, 7, now, now,
			))
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(sort_order\), -1\) AS max_sort_order FROM "memorial_gallery_images" WHERE memorial_entry_id = \$1`).
			WillReturnRows(sqlmock.NewRows([]string{"max_sort_order"}).AddRow(-1))
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "memorial_gallery_images"`)).
			WillReturnError(errors.New("insert failed"))
		mock.ExpectRollback()

		if _, err := svc.UpdateMemorial(11, SaveMemorialRequest{
			FullName: "Ada Lovelace",
			Category: MemorialCategoryFounder,
			Status:   MemorialStatusDraft,
			GalleryImages: []MemorialUploadInput{
				{FileName: "gallery-new.png", MimeType: "image/png", Content: []byte("gallery")},
			},
		}, nil); err == nil || err.Error() != "insert failed" {
			t.Fatalf("expected gallery insert failure, got %v", err)
		}
		if len(deletedKeys) != 1 || !strings.Contains(deletedKeys[0], "memorial/entry-11/gallery/") {
			t.Fatalf("expected uploaded gallery object cleanup, got %#v", deletedKeys)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("delete memorial", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		deletedKeys := make([]string, 0)
		previousDelete := deleteGCSObjectHook
		t.Cleanup(func() {
			deleteGCSObjectHook = previousDelete
		})
		deleteGCSObjectHook = func(bucketName, objectName string) error {
			deletedKeys = append(deletedKeys, bucketName+":"+objectName)
			return nil
		}

		svc := &MemorialService{DB: db, BucketName: "drive-bucket"}
		now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnRows(memorialEntryRows().AddRow(
				11, "Ada Lovelace", "Analytical Engine", MemorialCategoryFounder, MemorialStatusDraft, "<p>Hello</p>",
				nil, nil, nil,
				"portrait.jpg", "memorial/entry-11/portrait/file.jpg", "gs://drive-bucket/memorial/entry-11/portrait/file.jpg", "image/jpeg", 1024,
				7, 7, now, now,
			))
		mock.ExpectQuery(`SELECT \* FROM "memorial_gallery_images" WHERE memorial_entry_id = \$1`).
			WillReturnRows(memorialGalleryRows().
				AddRow(31, 11, "gallery-one.png", "memorial/entry-11/gallery/one.png", "gs://drive-bucket/memorial/entry-11/gallery/one.png", "image/png", 2048, 0, 7, now, now).
				AddRow(32, 11, "gallery-two.png", "memorial/entry-11/gallery/two.png", "gs://drive-bucket/memorial/entry-11/gallery/two.png", "image/png", 4096, 1, 7, now, now))
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "memorial_entries" WHERE "memorial_entries"."id" = $1`)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		if err := svc.DeleteMemorial(11); err != nil {
			t.Fatalf("DeleteMemorial returned error: %v", err)
		}
		if !containsMemorialString(deletedKeys, "drive-bucket:memorial/entry-11/portrait/file.jpg") ||
			!containsMemorialString(deletedKeys, "drive-bucket:memorial/entry-11/gallery/one.png") ||
			!containsMemorialString(deletedKeys, "drive-bucket:memorial/entry-11/gallery/two.png") {
			t.Fatalf("expected cleanup of stored objects, got %#v", deletedKeys)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})

	t.Run("delete memorial not found", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnRows(memorialEntryRows())
		mock.ExpectRollback()

		if err := svc.DeleteMemorial(99); !errors.Is(err, ErrMemorialNotFound) {
			t.Fatalf("expected ErrMemorialNotFound, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})
}

func TestMemorialServiceHelpersAndErrorBranches(t *testing.T) {
	t.Run("store unavailable guards", func(t *testing.T) {
		svc := &MemorialService{}
		if _, err := svc.ListMemorials(ListMemorialsFilter{}); !errors.Is(err, ErrStoreUnavailable) {
			t.Fatalf("expected ErrStoreUnavailable from ListMemorials, got %v", err)
		}
		if _, err := svc.GetMemorial(1); !errors.Is(err, ErrStoreUnavailable) {
			t.Fatalf("expected ErrStoreUnavailable from GetMemorial, got %v", err)
		}
		if _, err := svc.GetMemorialPortraitContent(1); !errors.Is(err, ErrStoreUnavailable) {
			t.Fatalf("expected ErrStoreUnavailable from GetMemorialPortraitContent, got %v", err)
		}
		if _, err := svc.GetMemorialGalleryImageContent(1, 2); !errors.Is(err, ErrStoreUnavailable) {
			t.Fatalf("expected ErrStoreUnavailable from GetMemorialGalleryImageContent, got %v", err)
		}
		if _, err := svc.CreateMemorial(SaveMemorialRequest{}, nil); !errors.Is(err, ErrStoreUnavailable) {
			t.Fatalf("expected ErrStoreUnavailable from CreateMemorial, got %v", err)
		}
		if _, err := svc.UpdateMemorial(1, SaveMemorialRequest{}, nil); !errors.Is(err, ErrStoreUnavailable) {
			t.Fatalf("expected ErrStoreUnavailable from UpdateMemorial, got %v", err)
		}
		if err := svc.DeleteMemorial(1); !errors.Is(err, ErrStoreUnavailable) {
			t.Fatalf("expected ErrStoreUnavailable from DeleteMemorial, got %v", err)
		}
	})

	t.Run("filter and validation helpers", func(t *testing.T) {
		filter, err := normalizeListMemorialsFilter(ListMemorialsFilter{Page: 0, PageSize: 200})
		if err != nil {
			t.Fatalf("normalizeListMemorialsFilter returned error: %v", err)
		}
		if filter.Page != 1 || filter.PageSize != 10 || filter.Status != "all" {
			t.Fatalf("unexpected normalized filter: %#v", filter)
		}
		if _, err := normalizeListMemorialsFilter(ListMemorialsFilter{Status: "bad"}); err == nil {
			t.Fatal("expected invalid status filter to fail")
		}
		if _, err := normalizeListMemorialsFilter(ListMemorialsFilter{Category: "bad"}); err == nil {
			t.Fatal("expected invalid category filter to fail")
		}

		if err := validateImageUploadInput(MemorialUploadInput{}); err == nil || err.Error() != "image file is required" {
			t.Fatalf("expected missing image file validation error, got %v", err)
		}
		if err := validateImageUploadInput(MemorialUploadInput{MimeType: "application/pdf", Content: []byte("pdf")}); err == nil || err.Error() != "only image uploads are supported" {
			t.Fatalf("expected image mime validation error, got %v", err)
		}

		if got := uniquePositiveInts([]int{4, 2, 4, 0, -1, 3}); fmt.Sprint(got) != "[2 3 4]" {
			t.Fatalf("unexpected unique ids: %#v", got)
		}
		if got := memorialCategoryLabel("veteran"); got != "Veteran" {
			t.Fatalf("unexpected category label: %q", got)
		}
		if got := memorialCategoryLabel("unknown"); got != "Memorial" {
			t.Fatalf("unexpected fallback category label: %q", got)
		}
		if got := memorialStatusLabel("review"); got != "Under Review" {
			t.Fatalf("unexpected status label: %q", got)
		}
		if got := memorialStatusLabel("unknown"); got != "Draft" {
			t.Fatalf("unexpected fallback status label: %q", got)
		}
	})

	t.Run("storage reference helpers", func(t *testing.T) {
		svc := &MemorialService{BucketName: "drive-bucket"}
		fileURL, objectKey, fileName, mimeType, fileSize, uploadedKey, err := svc.storeMemorialImage(7, "portrait", 0, MemorialUploadInput{
			FileURL:  "gs://drive-bucket/memorial/entry-7/portrait/file.jpg",
			MimeType: "image/jpeg",
			FileSize: 123,
		})
		if err != nil {
			t.Fatalf("storeMemorialImage without content returned error: %v", err)
		}
		if fileURL == "" || objectKey != "memorial/entry-7/portrait/file.jpg" || fileName != "memorial-image" || mimeType != "image/jpeg" || fileSize != 123 || uploadedKey != "" {
			t.Fatalf("unexpected stored image passthrough: url=%q objectKey=%q fileName=%q mimeType=%q fileSize=%d uploadedKey=%q", fileURL, objectKey, fileName, mimeType, fileSize, uploadedKey)
		}

		svc = &MemorialService{}
		if _, _, _, _, _, _, err := svc.storeMemorialImage(7, "portrait", 0, MemorialUploadInput{
			FileName: "portrait.jpg",
			Content:  []byte("portrait"),
		}); !errors.Is(err, ErrMediaBucketNotConfigured) {
			t.Fatalf("expected ErrMediaBucketNotConfigured, got %v", err)
		}

		svc = &MemorialService{BucketName: "drive-bucket"}
		if bucket, key, err := svc.resolveStoredObjectReference("memorial/entry-9/portrait/file.jpg", ""); err != nil || bucket != "drive-bucket" || key != "memorial/entry-9/portrait/file.jpg" {
			t.Fatalf("expected direct object key resolution, got bucket=%q key=%q err=%v", bucket, key, err)
		}
		svc = &MemorialService{}
		if _, _, err := svc.resolveStoredObjectReference("memorial/entry-9/portrait/file.jpg", ""); !errors.Is(err, ErrMediaBucketNotConfigured) {
			t.Fatalf("expected ErrMediaBucketNotConfigured, got %v", err)
		}
		svc = &MemorialService{}
		if _, _, err := svc.resolveStoredObjectReference("", "gs://drive-bucket"); err == nil || !strings.Contains(err.Error(), "not available from storage") {
			t.Fatalf("expected missing object name error, got %v", err)
		}
		svc = &MemorialService{BucketName: "drive-bucket"}
		if _, _, err := svc.resolveStoredObjectReference("", "https://example.com/file.jpg"); err == nil || !strings.Contains(err.Error(), "not available from storage") {
			t.Fatalf("expected unsupported storage reference error, got %v", err)
		}
		if bucket, key, err := svc.resolveStoredObjectReference("", "memorial/entry-9/portrait/file.jpg"); err != nil || bucket != "drive-bucket" || key != "memorial/entry-9/portrait/file.jpg" {
			t.Fatalf("unexpected resolved object reference: bucket=%q key=%q err=%v", bucket, key, err)
		}
	})

	t.Run("download and cleanup helpers", func(t *testing.T) {
		previousDownload := downloadGCSObjectHook
		previousDelete := deleteGCSObjectHook
		t.Cleanup(func() {
			downloadGCSObjectHook = previousDownload
			deleteGCSObjectHook = previousDelete
		})

		downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
			return nil, "", util.ErrObjectNotFound
		}

		svc := &MemorialService{BucketName: "drive-bucket"}
		if _, _, err := svc.downloadStoredObject(storedObjectRef{
			ObjectKey: "memorial/entry-9/portrait/file.jpg",
		}); !errors.Is(err, ErrMemorialMediaNotFound) {
			t.Fatalf("expected ErrMemorialMediaNotFound, got %v", err)
		}

		deleted := make([]string, 0)
		deleteGCSObjectHook = func(bucketName, objectName string) error {
			deleted = append(deleted, bucketName+":"+objectName)
			return nil
		}

		svc.cleanupObjects([]string{" object-a ", "", "object-b"})
		svc.cleanupStoredObjectsBestEffort([]storedObjectRef{
			{ObjectKey: "memorial/entry-9/gallery/one.jpg"},
			{FileURL: "gs://drive-bucket/memorial/entry-9/gallery/two.jpg"},
			{FileURL: "https://example.com/nope.jpg"},
		})
		if !containsMemorialString(deleted, "drive-bucket:object-a") ||
			!containsMemorialString(deleted, "drive-bucket:object-b") ||
			!containsMemorialString(deleted, "drive-bucket:memorial/entry-9/gallery/one.jpg") ||
			!containsMemorialString(deleted, "drive-bucket:memorial/entry-9/gallery/two.jpg") {
			t.Fatalf("unexpected cleanup calls: %#v", deleted)
		}
	})

	t.Run("date and filename helpers", func(t *testing.T) {
		if got := formatOptionalDate(nil); got != "" {
			t.Fatalf("expected empty formatted nil date, got %q", got)
		}
		parsed, err := parseOptionalISODate("2026-05-25")
		if err != nil || parsed == nil || parsed.Format("2006-01-02") != "2026-05-25" {
			t.Fatalf("unexpected parsed date: %#v err=%v", parsed, err)
		}
		if _, err := parseOptionalISODate("2026-99-99"); err == nil {
			t.Fatal("expected invalid date to fail")
		}
		if got := buildStoredFileName("", "fallback"); got != "fallback" {
			t.Fatalf("expected fallback filename, got %q", got)
		}
		if got := buildStoredFileName("memorial/entry-7/portrait/file.jpg", "fallback"); got != "file.jpg" {
			t.Fatalf("expected base filename, got %q", got)
		}
		if !looksLikeGCSReference("gs://drive-bucket/file.jpg") || !looksLikeGCSReference("memorial/entry-7/file.jpg") || looksLikeGCSReference("https://example.com/file.jpg") {
			t.Fatal("unexpected GCS reference detection result")
		}
		if !looksLikeGCSReference("https://storage.googleapis.com/drive-bucket/file.jpg") {
			t.Fatal("expected storage.googleapis.com urls to be treated as GCS references")
		}

		current := time.Date(2026, 5, 25, 10, 30, 0, 0, time.UTC)
		if nextPublishedAt(&current, MemorialStatusDraft) != nil {
			t.Fatal("expected draft status to clear published date")
		}
		if cloned := nextPublishedAt(&current, MemorialStatusPublished); cloned == nil || !cloned.Equal(current) || cloned == &current {
			t.Fatalf("expected published date to be cloned, got %#v", cloned)
		}
		previousNow := memorialNowFunc
		t.Cleanup(func() {
			memorialNowFunc = previousNow
		})
		memorialNowFunc = func() time.Time { return current }
		if generated := nextPublishedAt(nil, MemorialStatusPublished); generated == nil || !generated.Equal(current) {
			t.Fatalf("expected publish time to be generated, got %#v", generated)
		}

		sanitized := sanitizeUploadInput(MemorialUploadInput{
			FileName:     " portrait.jpg ",
			MimeType:     " image/jpeg ",
			FileURL:      " gs://drive-bucket/file.jpg ",
			GCPObjectKey: " memorial/file.jpg ",
		})
		if sanitized.FileName != "portrait.jpg" || sanitized.MimeType != "image/jpeg" || sanitized.FileURL != "gs://drive-bucket/file.jpg" || sanitized.GCPObjectKey != "memorial/file.jpg" {
			t.Fatalf("unexpected sanitized upload input: %#v", sanitized)
		}

		if got := buildMemorialPortraitContentURL(7); got != "/api/memorial/7/portrait/content" {
			t.Fatalf("unexpected portrait content url: %q", got)
		}
		if got := buildMemorialGalleryImageContentURL(7, 3); got != "/api/memorial/7/gallery/3/content" {
			t.Fatalf("unexpected gallery content url: %q", got)
		}
	})

	t.Run("table names", func(t *testing.T) {
		if got := (MemorialEntry{}).TableName(); got != "memorial_entries" {
			t.Fatalf("unexpected memorial entry table name: %q", got)
		}
		if got := (MemorialGalleryImage{}).TableName(); got != "memorial_gallery_images" {
			t.Fatalf("unexpected memorial gallery table name: %q", got)
		}
	})

	t.Run("detect uploaded content type fallback", func(t *testing.T) {
		if got := detectUploadedContentType("", []byte("hello")); got != http.DetectContentType([]byte("hello")) {
			t.Fatalf("expected detected content type, got %q", got)
		}
	})

	t.Run("rollback on panic helper", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		mock.ExpectBegin()
		tx := db.Begin()
		if tx.Error != nil {
			t.Fatalf("db.Begin returned error: %v", tx.Error)
		}
		mock.ExpectRollback()

		defer func() {
			if recovered := recover(); recovered != "transaction panic" {
				t.Fatalf("expected transaction panic, got %#v", recovered)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet sqlmock expectations: %v", err)
			}
		}()

		func() {
			defer rollbackOnPanic(tx)
			panic("boom")
		}()
	})

	t.Run("model lookup helper", func(t *testing.T) {
		db, mock, cleanup := setupMemorialMockDB(t)
		defer cleanup()

		svc := &MemorialService{DB: db}
		now := time.Now()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnRows(memorialEntryRows().AddRow(
				7, "Ada Lovelace", "Analytical Engine", MemorialCategoryFounder, MemorialStatusDraft, "<p>Hello</p>",
				nil, nil, nil, "", "", "", "", 0, 7, 7, now, now,
			))
		if _, err := svc.getMemorialEntryModel(7); err != nil {
			t.Fatalf("expected memorial model lookup success, got %v", err)
		}

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "memorial_entries" WHERE "memorial_entries"."id" = $1 ORDER BY "memorial_entries"."id" LIMIT $2`)).
			WillReturnRows(memorialEntryRows())
		if _, err := svc.getMemorialEntryModel(8); !errors.Is(err, ErrMemorialNotFound) {
			t.Fatalf("expected ErrMemorialNotFound, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	})
}
