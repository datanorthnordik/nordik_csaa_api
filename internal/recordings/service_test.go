package recordings

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

func stubRecordingHooks() func() {
	originalNow := recordingNowFunc
	originalUploadBase64 := uploadBase64ToGCSHook
	originalUploadBytes := uploadBytesToGCSHook
	originalDownload := downloadGCSObjectHook
	originalDelete := deleteGCSObjectHook

	recordingNowFunc = func() time.Time { return time.Unix(0, 0).UTC() }
	uploadBase64ToGCSHook = func(_, _, objectName, _ string) (string, int64, error) {
		return "https://storage.example.com/" + objectName, 5, nil
	}
	uploadBytesToGCSHook = func(_ []byte, _, objectName, _ string) (string, int64, error) {
		return "https://storage.example.com/" + objectName, 5, nil
	}
	downloadGCSObjectHook = func(_, _ string) ([]byte, string, error) {
		return []byte("audio-bytes"), "audio/mpeg", nil
	}
	deleteGCSObjectHook = func(_, _ string) error { return nil }

	return func() {
		recordingNowFunc = originalNow
		uploadBase64ToGCSHook = originalUploadBase64
		uploadBytesToGCSHook = originalUploadBytes
		downloadGCSObjectHook = originalDownload
		deleteGCSObjectHook = originalDelete
	}
}

func intPtr(v int) *int { return &v }

func TestRecordingServiceStoreUnavailable(t *testing.T) {
	svc := &RecordingService{}

	if _, err := svc.ListRecordingCollections(); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ErrStoreUnavailable from ListRecordingCollections, got %v", err)
	}
	if _, err := svc.GetRecordingCollection(1); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ErrStoreUnavailable from GetRecordingCollection, got %v", err)
	}
	if _, err := svc.GetRecordingCollectionByPlacementKey("living-history-recordings"); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ErrStoreUnavailable from GetRecordingCollectionByPlacementKey, got %v", err)
	}
	if _, err := svc.CreateRecordingCollection(SaveRecordingCollectionRequest{Name: "X"}, nil); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ErrStoreUnavailable from CreateRecordingCollection, got %v", err)
	}
	if err := svc.DeleteRecordingCollection(1); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ErrStoreUnavailable from DeleteRecordingCollection, got %v", err)
	}
}

func TestGetRecordingCollectionByPlacementKey(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &RecordingService{DB: db}
	now := time.Date(2026, time.September, 22, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "recording_collections" WHERE placement_key = $1`)).
		WithArgs("living-history-recordings", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "placement_key", "created_at", "updated_at"}).
			AddRow(8, "Living History Recordings", "living-history-recordings", now, now))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "recording_collection_items" WHERE recording_collection_id = $1`)).
		WithArgs(8).
		WillReturnRows(sqlmock.NewRows([]string{"id", "recording_collection_id", "title", "sort_order", "created_at", "updated_at"}))

	resp, err := svc.GetRecordingCollectionByPlacementKey(" living-history-recordings ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != 8 || resp.Name != "Living History Recordings" || resp.PlacementKey != "living-history-recordings" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if len(resp.Items) != 0 {
		t.Fatalf("expected seeded collection to be empty, got %+v", resp.Items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCreateRecordingCollectionRequiresName(t *testing.T) {
	db, _, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &RecordingService{DB: db}

	if _, err := svc.CreateRecordingCollection(SaveRecordingCollectionRequest{Name: "   "}, nil); err == nil {
		t.Fatal("expected an error for a blank collection name")
	}
}

func TestCreateRecordingCollectionAllowsEmptyCollection(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &RecordingService{DB: db}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "recording_collections"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
	mock.ExpectCommit()

	resp, err := svc.CreateRecordingCollection(SaveRecordingCollectionRequest{
		Name: "Living History Recordings",
	}, intPtr(7))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.ID != 5 || resp.Name != "Living History Recordings" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCreateRecordingCollectionRequiresRecordingForEachItem(t *testing.T) {
	db, _, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &RecordingService{DB: db}
	_, err := svc.CreateRecordingCollection(SaveRecordingCollectionRequest{
		Name: "Oral Histories",
		Items: []RecordingItemInput{
			{Title: "Episode 1", Description: "A story told at the gathering."},
		},
	}, intPtr(7))
	if err == nil || err.Error() != "recording file is required" {
		t.Fatalf("expected recording requirement error, got %v", err)
	}
}

func TestCreateRecordingCollectionWithRecording(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &RecordingService{DB: db, BucketName: "drive-bucket"}
	restore := stubRecordingHooks()
	defer restore()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "recording_collections"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "recording_collection_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectCommit()

	resp, err := svc.CreateRecordingCollection(SaveRecordingCollectionRequest{
		Name: "Oral Histories",
		Items: []RecordingItemInput{
			{
				Title:       "Episode 1",
				Description: "A story told at the gathering.",
				FileName:    "episode-1.mp3",
				MimeType:    "audio/mpeg",
				Content:     []byte("audio-bytes"),
			},
		},
	}, intPtr(7))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.ID != 5 || resp.Name != "Oral Histories" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAddRecordingItemsCollectionNotFound(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &RecordingService{DB: db, BucketName: "drive-bucket"}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "recording_collections"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
	mock.ExpectRollback()

	_, err := svc.AddRecordingItems(99, AddRecordingItemsRequest{
		Items: []RecordingItemInput{{Title: "Episode 1", FileURL: "gs://drive-bucket/episode-1.mp3"}},
	}, nil)
	if !errors.Is(err, ErrRecordingCollectionNotFound) {
		t.Fatalf("expected ErrRecordingCollectionNotFound, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDeleteRecordingCollectionNotFound(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &RecordingService{DB: db}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "recording_collections"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
	mock.ExpectRollback()

	if err := svc.DeleteRecordingCollection(42); !errors.Is(err, ErrRecordingCollectionNotFound) {
		t.Fatalf("expected ErrRecordingCollectionNotFound, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
