package knowledgecenter

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupKnowledgeCenterMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
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

func TestKnowledgeCenterListSubmissionsUsesIndependentQueries(t *testing.T) {
	db, mock, cleanup := setupKnowledgeCenterMockDB(t)
	defer cleanup()

	service := &KnowledgeCenterService{DB: db}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT status, COUNT(*) AS count FROM "knowledge_center_submissions" GROUP BY "status"`)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "count"}).
			AddRow(KnowledgeCenterSubmissionStatusOpen, 1).
			AddRow(KnowledgeCenterSubmissionStatusCompleted, 0))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "knowledge_center_submissions" WHERE status = $1`)).
		WithArgs(KnowledgeCenterSubmissionStatusOpen).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	createdAt := time.Date(2026, 6, 19, 9, 30, 0, 0, time.UTC)
	updatedAt := createdAt.Add(15 * time.Minute)

	mock.ExpectQuery(`SELECT \* FROM "knowledge_center_submissions" WHERE status = \$1 ORDER BY .*created_at.*id.* LIMIT \$2`).
		WithArgs(KnowledgeCenterSubmissionStatusOpen, 10).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"submitter_name",
			"submitter_email",
			"submitter_phone",
			"submission_type",
			"message",
			"status",
			"completion_notes",
			"completed_by_user_id",
			"completed_by_name",
			"completed_by_email",
			"completed_at",
			"created_at",
			"updated_at",
		}).AddRow(
			7,
			"Alice Walker",
			"alice@example.com",
			"555-0100",
			KnowledgeCenterSubmissionTypePost,
			"A written story",
			KnowledgeCenterSubmissionStatusOpen,
			"",
			nil,
			"",
			"",
			nil,
			createdAt,
			updatedAt,
		))

	resp, err := service.ListSubmissions(ListKnowledgeCenterSubmissionsFilter{
		Status: KnowledgeCenterSubmissionStatusOpen,
	})
	if err != nil {
		t.Fatalf("ListSubmissions returned error: %v", err)
	}

	if len(resp.Items) != 1 || resp.Items[0].ID != 7 {
		t.Fatalf("unexpected items payload: %#v", resp.Items)
	}
	if resp.Summary.OpenCount != 1 || resp.Summary.CompletedCount != 0 {
		t.Fatalf("unexpected summary: %#v", resp.Summary)
	}
	if resp.Pagination.TotalItems != 1 || resp.Pagination.TotalPages != 1 {
		t.Fatalf("unexpected pagination: %#v", resp.Pagination)
	}
	if resp.Applied.Status != KnowledgeCenterSubmissionStatusOpen {
		t.Fatalf("unexpected applied filters: %#v", resp.Applied)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}
