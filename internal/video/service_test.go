package video

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

func TestVideoServiceStoreUnavailable(t *testing.T) {
	svc := &VideoService{}

	if _, err := svc.CreateVideoPackage(SaveVideoPackageRequest{}, nil); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ErrStoreUnavailable, got %v", err)
	}
	if _, err := svc.UpdateVideoPackage(1, UpdateVideoPackageRequest{Title: "Videos"}, nil); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ErrStoreUnavailable, got %v", err)
	}
	if err := svc.DeleteVideoPackage(1); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ErrStoreUnavailable, got %v", err)
	}
}

func TestCreateUpdateAndManageVideoPackage(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &VideoService{DB: db, BucketName: "drive-bucket"}
	restore := stubVideoHooks()
	defer restore()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "video_packages"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "video_package_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectCommit()

	resp, err := svc.CreateVideoPackage(SaveVideoPackageRequest{
		PackageType: VideoPackageTypeSingle,
		SingleVideo: &VideoInput{
			Title:       "Board Update",
			YouTubeURL:  "https://www.youtube.com/watch?v=abc123",
			FileName:    "teaser.png",
			MimeType:    "image/png",
			DataBase64:  "aGVsbG8=",
			Description: "Community update",
		},
	}, intPtr(7))
	if err != nil {
		t.Fatalf("CreateVideoPackage returned error: %v", err)
	}
	if resp.ID != 5 || resp.Title != "Board Update" || resp.PackageType != VideoPackageTypeSingle {
		t.Fatalf("unexpected create response: %#v", resp)
	}

	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "video_packages" WHERE "video_packages"."id" = $1 ORDER BY "video_packages"."id" LIMIT $2`)).
		WithArgs(5, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "package_type", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(5, "Board Update", VideoPackageTypeSingle, 7, 7, now, now))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "video_packages" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "video_package_items" WHERE video_package_id = $1 ORDER BY sort_order ASC,id ASC`)).
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "video_package_id", "title", "youtube_url", "description", "teaser_image_url", "teaser_image_object_key", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(11, 5, "Board Update", "https://www.youtube.com/watch?v=abc123", "Community update", "gs://drive-bucket/videos/5/items/teaser.png", "videos/5/items/teaser.png", 0, 7, 7, now, now))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "video_package_items" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	resp, err = svc.UpdateVideoPackage(5, UpdateVideoPackageRequest{Title: "Board Update Revised"}, intPtr(9))
	if err != nil {
		t.Fatalf("UpdateVideoPackage returned error: %v", err)
	}
	if resp.Title != "Board Update Revised" {
		t.Fatalf("expected updated package title, got %#v", resp)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "video_packages" WHERE "video_packages"."id" = $1 ORDER BY "video_packages"."id" LIMIT $2`)).
		WithArgs(7, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "package_type", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(7, "Community Videos", VideoPackageTypeCollection, 7, 7, now, now))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(MAX(sort_order), -1) AS max_sort_order FROM "video_package_items" WHERE video_package_id = $1`)).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{"max_sort_order"}).AddRow(-1))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "video_package_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(21))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "video_packages" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	addResp, err := svc.AddVideoItems(7, AddVideoItemsRequest{
		Videos: []VideoInput{{
			Title:      "Chapter One",
			YouTubeURL: "https://youtu.be/example123",
			FileName:   "cover.png",
			MimeType:   "image/png",
			DataBase64: "aGVsbG8=",
		}},
	}, intPtr(7))
	if err != nil || addResp.UploadedCount != 1 {
		t.Fatalf("unexpected add items result: resp=%#v err=%v", addResp, err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "video_packages" WHERE "video_packages"."id" = $1 ORDER BY "video_packages"."id" LIMIT $2`)).
		WithArgs(7, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "package_type", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(7, "Community Videos", VideoPackageTypeCollection, 7, 7, now, now))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "video_package_items" WHERE video_package_id = $1 AND id = $2 ORDER BY "video_package_items"."id" LIMIT $3`)).
		WithArgs(7, 21, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "video_package_id", "title", "youtube_url", "description", "teaser_image_url", "teaser_image_object_key", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(21, 7, "Chapter One", "https://youtu.be/example123", "", "gs://drive-bucket/videos/7/items/cover.png", "videos/7/items/cover.png", 0, 7, 7, now, now))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "video_package_items" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "video_packages" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	itemResp, err := svc.UpdateVideoItem(7, 21, UpdateVideoItemRequest{
		Title:       "Chapter One Revised",
		YouTubeURL:  "https://youtu.be/example123",
		Description: "New teaser copy",
	}, intPtr(9))
	if err != nil || itemResp.Title != "Chapter One Revised" {
		t.Fatalf("unexpected update item result: resp=%#v err=%v", itemResp, err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "video_packages" WHERE "video_packages"."id" = $1 ORDER BY "video_packages"."id" LIMIT $2`)).
		WithArgs(7, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "package_type", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(7, "Community Videos", VideoPackageTypeCollection, 7, 9, now, now))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "video_package_items" WHERE video_package_id = $1 AND id = $2 ORDER BY "video_package_items"."id" LIMIT $3`)).
		WithArgs(7, 21, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "video_package_id", "title", "youtube_url", "description", "teaser_image_url", "teaser_image_object_key", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(21, 7, "Chapter One Revised", "https://youtu.be/example123", "New teaser copy", "gs://drive-bucket/videos/7/items/cover.png", "videos/7/items/cover.png", 0, 7, 9, now, now))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "video_package_items" WHERE "video_package_items"."id" = $1`)).
		WithArgs(21).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "video_package_items" WHERE video_package_id = $1 ORDER BY sort_order ASC,id ASC`)).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "video_package_id", "title", "youtube_url", "description", "teaser_image_url", "teaser_image_object_key", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "video_packages" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	delResp, err := svc.DeleteVideoItem(7, 21)
	if err != nil || delResp.DeletedCount != 1 {
		t.Fatalf("unexpected delete item result: resp=%#v err=%v", delResp, err)
	}
}

func TestVideoValidationAndDetailMapping(t *testing.T) {
	if _, _, err := normalizeCreateVideoPackageRequest(SaveVideoPackageRequest{PackageType: VideoPackageTypeSingle}); err == nil {
		t.Fatal("expected create validation error for missing single_video")
	}
	if _, err := normalizeAddVideoItemsRequest(AddVideoItemsRequest{
		Videos: []VideoInput{{
			Title:      "Bad Item",
			YouTubeURL: "https://example.com/video",
			FileName:   "cover.png",
			MimeType:   "image/png",
			DataBase64: "aGVsbG8=",
		}},
	}); err == nil || err.Error() != "youtube_url must be a valid YouTube URL" {
		t.Fatalf("expected youtube validation error, got %v", err)
	}

	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &VideoService{DB: db, BucketName: "drive-bucket"}
	restore := stubVideoHooks()
	defer restore()
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "video_packages" WHERE "video_packages"."id" = $1 ORDER BY "video_packages"."id" LIMIT $2`)).
		WithArgs(7, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "package_type", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(7, "Community Videos", VideoPackageTypeCollection, 7, 9, now, now))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "video_package_items" WHERE video_package_id = $1 ORDER BY sort_order ASC,id ASC`)).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "video_package_id", "title", "youtube_url", "description", "teaser_image_url", "teaser_image_object_key", "sort_order", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(21, 7, "Chapter One", "https://youtu.be/example123", "New teaser copy", "gs://drive-bucket/videos/7/items/cover.png", "videos/7/items/cover.png", 0, 7, 9, now, now))

	resp, err := svc.GetVideoPackage(7)
	if err != nil {
		t.Fatalf("GetVideoPackage returned error: %v", err)
	}
	if resp.VideoCount != 1 || len(resp.Videos) != 1 || resp.Videos[0].TeaserImageURL != "/api/videos/7/items/21/teaser/content" {
		t.Fatalf("unexpected detail response: %#v", resp)
	}
}

func stubVideoHooks() func() {
	prevUpload := uploadBase64ToGCSHook
	prevUploadBytes := uploadBytesToGCSHook
	prevDownload := downloadGCSObjectHook
	prevDelete := deleteGCSObjectHook
	prevNow := videoNowFunc

	uploadBase64ToGCSHook = func(base64Data, bucketName, objectName, contentType string) (string, int64, error) {
		return "gs://" + bucketName + "/" + objectName, int64(len(base64Data)), nil
	}
	uploadBytesToGCSHook = func(data []byte, bucketName, objectName, contentType string) (string, int64, error) {
		return "gs://" + bucketName + "/" + objectName, int64(len(data)), nil
	}
	downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
		return []byte("file"), "image/png", nil
	}
	deleteGCSObjectHook = func(bucketName, objectName string) error { return nil }
	videoNowFunc = func() time.Time { return time.Date(2026, 5, 11, 14, 25, 21, 0, time.UTC) }

	return func() {
		uploadBase64ToGCSHook = prevUpload
		uploadBytesToGCSHook = prevUploadBytes
		downloadGCSObjectHook = prevDownload
		deleteGCSObjectHook = prevDelete
		videoNowFunc = prevNow
	}
}

func intPtr(v int) *int { return &v }
