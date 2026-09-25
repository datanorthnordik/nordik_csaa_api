package latestcontent

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestListLatestContentReturnsPublicationOrderAndSafeDetailPaths(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	publishedAt := time.Date(2026, time.September, 25, 10, 0, 0, 0, time.UTC)
	displayDate := time.Date(2026, time.October, 24, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "latest_content" ORDER BY published_at DESC,id DESC LIMIT $1`,
	)).WithArgs(3).WillReturnRows(
		sqlmock.NewRows([]string{
			"id", "source_type", "source_id", "title", "description",
			"display_date", "published_at", "created_at", "updated_at",
		}).AddRow(
			1, SourceTypeEvent, 42, "Traditional Storytelling Circle", "Join us.",
			displayDate, publishedAt, publishedAt, publishedAt,
		),
	)

	response, err := (&Service{DB: db}).ListLatestContent(3)
	if err != nil {
		t.Fatalf("ListLatestContent: %v", err)
	}
	if len(response.Items) != 1 {
		t.Fatalf("expected one item, got %d", len(response.Items))
	}
	if response.Items[0].DetailPath != "/events/42" {
		t.Fatalf("unexpected detail path %q", response.Items[0].DetailPath)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestLatestContentDetailPathsAndLimitBounds(t *testing.T) {
	if normalizeLimit(0) != 3 || normalizeLimit(99) != MaxItems {
		t.Fatal("latest content limits were not normalized")
	}

	tests := map[string]string{
		SourceTypeEvent:      "/events/7",
		SourceTypeNewsletter: "/news-media/digital-newsletter/7",
		SourceTypePress:      "/news-media/press-archive/7",
		"unknown":            "",
	}
	for sourceType, expected := range tests {
		if actual := detailPath(sourceType, 7); actual != expected {
			t.Fatalf("detailPath(%q) = %q, want %q", sourceType, actual, expected)
		}
	}
}
