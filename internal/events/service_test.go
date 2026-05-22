package events

import (
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
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

func TestEventServiceReturnsStoreUnavailableWithoutDB(t *testing.T) {
	service := &EventService{}
	req := validSaveEventRequest()

	if _, err := service.ListEvents(ListEventsFilter{}); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ListEvents to return ErrStoreUnavailable, got %v", err)
	}
	if _, err := service.GetEvent(1); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected GetEvent to return ErrStoreUnavailable, got %v", err)
	}
	if _, err := service.ListSavedLocations(); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ListSavedLocations to return ErrStoreUnavailable, got %v", err)
	}
	if _, err := service.ListGalleries(); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ListGalleries to return ErrStoreUnavailable, got %v", err)
	}
	if _, err := service.CreateEvent(req); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected CreateEvent to return ErrStoreUnavailable, got %v", err)
	}
	if _, err := service.UpdateEvent(1, req); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected UpdateEvent to return ErrStoreUnavailable, got %v", err)
	}
	if err := service.DeleteEvent(1); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected DeleteEvent to return ErrStoreUnavailable, got %v", err)
	}
	if err := service.DeleteEventDocument(1, "gs://drive-bucket/events/1/file.pdf"); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected DeleteEventDocument to return ErrStoreUnavailable, got %v", err)
	}
	if _, err := service.DeleteAllEventDocuments(1, nil); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected DeleteAllEventDocuments to return ErrStoreUnavailable, got %v", err)
	}
	if err := service.DeleteEventPhoto(1, "gs://drive-bucket/events/1/file.png"); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected DeleteEventPhoto to return ErrStoreUnavailable, got %v", err)
	}
}

func TestListEventsSuccessAndValidation(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &EventService{DB: db}
	restore := stubMediaHooks()
	defer restore()

	nowFunc = func() time.Time {
		return time.Date(2026, 5, 7, 9, 0, 0, 0, time.UTC)
	}
	defer func() {
		nowFunc = time.Now
	}()

	filter := ListEventsFilter{
		Page:       2,
		PageSize:   5,
		SearchTerm: "spring",
		Statuses:   []string{"published"},
		DateRange:  EventDateRangeNext30Days,
		SortBy:     "title",
		SortOrder:  "asc",
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "events" WHERE LOWER(title) LIKE $1 AND published = $2 AND start_at >= $3 AND start_at < $4`)).
		WithArgs("%spring%", true, time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC), time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(11))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "events" WHERE LOWER(title) LIKE $1 AND published = $2 AND start_at >= $3 AND start_at < $4 ORDER BY title ASC LIMIT $5 OFFSET $6`)).
		WithArgs("%spring%", true, time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC), time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC), 5, 5).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "show_title", "categories", "event_type", "start_at", "end_at", "privacy_type", "private_audiences",
			"published", "request_review", "review_email_list", "teaser", "description_html", "contact_name", "contact_email",
			"contact_phone", "contact_ext", "contact_fax", "location_mode", "address_id", "show_display_image_when_viewing",
			"gallery_id", "registration_enabled", "registration_start_at", "registration_end_at", "registration_url",
			"repeat_enabled", "recurrence_type", "recurrence_frequency", "recurrence_interval", "recurrence_until",
			"recurrence_rule", "created_by", "created_at", "updated_at",
		}).AddRow(
			9, "Spring Fair", true, pq.Array([]string{"Events"}), "single_day_all_day", time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC), nil, "public", pq.Array([]string{}),
			true, false, pq.Array([]string{}), "Teaser", "<p>Hello</p>", "", "", "", "", "", "address", 7, true,
			nil, true, time.Date(2026, 5, 8, 9, 0, 0, 0, time.UTC), time.Date(2026, 5, 9, 17, 0, 0, 0, time.UTC), "https://example.com/register", true, "scheduled", "", 1, nil, []byte(`null`), nil, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
		))
	mock.ExpectQuery(`SELECT \* FROM "addresses" WHERE id IN \(\$1\)`).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "address_line_1", "address_line_2", "city", "province_state", "postal_code", "country", "is_saved", "created_at", "updated_at",
		}).AddRow(
			7, "Community Hall", "1 Main", "", "Toronto", "Ontario", "A1A 1A1", "Canada", true, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
		))
	mock.ExpectQuery(`SELECT \* FROM "event_media" WHERE event_id IN \(\$1\) ORDER BY event_id ASC,sort_order ASC,id ASC`).
		WithArgs(9).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "event_id", "media_role", "display_name", "gcp_object_key", "file_url", "mime_type", "file_size", "sort_order", "created_at", "updated_at",
		}).
			AddRow(3, 9, MediaRoleDisplayImage, "Banner", "events/9/banner.png", "gs://drive-bucket/events/9/banner.png", "image/png", 10, 0, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)).
			AddRow(4, 9, MediaRoleAttachment, "Agenda", "events/9/agenda.pdf", "gs://drive-bucket/events/9/agenda.pdf", "application/pdf", 20, 1, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)))
	mock.ExpectQuery(`SELECT \* FROM "event_occurrences" WHERE event_id IN \(\$1\) ORDER BY event_id ASC,occurrence_start_at ASC,id ASC`).
		WithArgs(9).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "event_id", "occurrence_start_at", "occurrence_end_at", "occurrence_kind", "created_at", "updated_at",
		}).AddRow(
			5, 9, time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC), time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC), "scheduled", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		))

	resp, err := service.ListEvents(filter)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if resp.Pagination.TotalItems != 11 || len(resp.Items) != 1 || resp.Items[0].Status != EventStatusPublished {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if resp.Items[0].DateDisplay != "2026-05-10" {
		t.Fatalf("unexpected date display: %q", resp.Items[0].DateDisplay)
	}
	if resp.Items[0].Address == nil || resp.Items[0].Address.Name != "Community Hall" {
		t.Fatalf("expected list address details, got %#v", resp.Items[0].Address)
	}
	if resp.Items[0].DisplayImage == nil || resp.Items[0].DisplayImage.FileURL != "/api/events/9/media/3/content" {
		t.Fatalf("expected list display image, got %#v", resp.Items[0].DisplayImage)
	}
	if len(resp.Items[0].Attachments) != 1 || resp.Items[0].Attachments[0].FileURL != "/api/events/9/media/4/content" {
		t.Fatalf("expected list attachments, got %#v", resp.Items[0].Attachments)
	}
	if len(resp.Items[0].Occurrences) != 1 || !resp.Items[0].RegistrationEnabled {
		t.Fatalf("expected list occurrences and registration fields, got %#v", resp.Items[0])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}

	if _, err := normalizeListEventsFilter(ListEventsFilter{Statuses: []string{"bad"}}); err == nil {
		t.Fatal("expected invalid status validation error")
	}
	if _, err := normalizeListEventsFilter(ListEventsFilter{DateRange: "bad"}); err == nil {
		t.Fatal("expected invalid date_range validation error")
	}
	if _, err := normalizeListEventsFilter(ListEventsFilter{SortBy: "drop table"}); err == nil {
		t.Fatal("expected invalid sort_by validation error")
	}
	if _, err := normalizeListEventsFilter(ListEventsFilter{SortOrder: "sideways"}); err == nil {
		t.Fatal("expected invalid sort_order validation error")
	}
}

func TestListSavedLocationsAndGalleries(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &EventService{DB: db}

	mock.ExpectQuery(`SELECT \* FROM "addresses"`).
		WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "address_line_1", "address_line_2", "city", "province_state", "postal_code", "country", "is_saved", "created_at", "updated_at",
		}).AddRow(
			7, "Community Hall", "1 Main", "", "Toronto", "Ontario", "A1A 1A1", "Canada", true, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
		))
	mock.ExpectQuery(`SELECT \* FROM "galleries"`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "created_at", "updated_at",
		}).AddRow(
			9, "Homepage Gallery", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
		))

	locationsResp, err := service.ListSavedLocations()
	if err != nil {
		t.Fatalf("ListSavedLocations returned error: %v", err)
	}
	if len(locationsResp.Items) != 1 || locationsResp.Items[0].ID != 7 || !locationsResp.Items[0].IsSaved {
		t.Fatalf("unexpected locations response: %#v", locationsResp)
	}

	galleriesResp, err := service.ListGalleries()
	if err != nil {
		t.Fatalf("ListGalleries returned error: %v", err)
	}
	if len(galleriesResp.Items) != 1 || galleriesResp.Items[0].ID != 9 {
		t.Fatalf("unexpected galleries response: %#v", galleriesResp)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetEventSuccessAndNotFound(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &EventService{DB: db}
	startAt := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	endAt := startAt.Add(2 * time.Hour)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "events" WHERE "events"."id" = $1 ORDER BY "events"."id" LIMIT $2`)).
		WithArgs(12, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "show_title", "categories", "event_type", "start_at", "end_at", "privacy_type", "private_audiences",
			"published", "request_review", "review_email_list", "teaser", "description_html", "contact_name", "contact_email",
			"contact_phone", "contact_ext", "contact_fax", "location_mode", "address_id", "show_display_image_when_viewing",
			"gallery_id", "registration_enabled", "registration_start_at", "registration_end_at", "registration_url",
			"repeat_enabled", "recurrence_type", "recurrence_frequency", "recurrence_interval", "recurrence_until",
			"recurrence_rule", "created_by", "created_at", "updated_at",
		}).AddRow(
			12, "Spring Fair", true, pq.Array([]string{"Events"}), "single_day_partial", startAt, endAt, "private", pq.Array([]string{"Members"}),
			false, true, pq.Array([]string{"review@example.com"}), "Teaser", "<p>Hello</p>", "Jane", "jane@example.com",
			"555-555-5555", "123", "555-555-9999", "address", 7, true,
			nil, false, nil, nil, "", true, "scheduled", "", 1, nil, []byte(`null`), nil, startAt, startAt,
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "addresses" WHERE "addresses"."id" = $1 ORDER BY "addresses"."id" LIMIT $2`)).
		WithArgs(7, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "address_line_1", "address_line_2", "city", "province_state", "postal_code", "country", "is_saved", "created_at", "updated_at",
		}).AddRow(
			7, "Community Hall", "1 Main", "", "Toronto", "Ontario", "A1A 1A1", "Canada", true, startAt, startAt,
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_media" WHERE event_id = $1 ORDER BY sort_order ASC,id ASC`)).
		WithArgs(12).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "event_id", "media_role", "display_name", "gcp_object_key", "file_url", "mime_type", "file_size", "sort_order", "created_at", "updated_at",
		}).
			AddRow(3, 12, MediaRoleDisplayImage, "Banner", "events/12/banner.png", "gs://drive-bucket/events/12/banner.png", "image/png", 10, 0, startAt, startAt).
			AddRow(4, 12, MediaRoleAttachment, "Agenda", "events/12/agenda.pdf", "gs://drive-bucket/events/12/agenda.pdf", "application/pdf", 20, 1, startAt, startAt))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_occurrences" WHERE event_id = $1 ORDER BY occurrence_start_at ASC,id ASC`)).
		WithArgs(12).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "event_id", "occurrence_start_at", "occurrence_end_at", "occurrence_kind", "created_at", "updated_at",
		}).AddRow(
			5, 12, startAt, endAt, "scheduled", startAt, startAt,
		))

	resp, err := service.GetEvent(12)
	if err != nil {
		t.Fatalf("GetEvent returned error: %v", err)
	}
	if resp.ID != 12 || resp.Address == nil || resp.DisplayImage == nil || len(resp.Attachments) != 1 || len(resp.Occurrences) != 1 {
		t.Fatalf("unexpected detail response: %#v", resp)
	}
	if resp.DateDisplay != "2026-05-01 10:00 - 2026-05-01 12:00" {
		t.Fatalf("unexpected date display: %q", resp.DateDisplay)
	}
	if resp.DisplayImage.FileURL != "/api/events/12/media/3/content" {
		t.Fatalf("unexpected display image file url: %q", resp.DisplayImage.FileURL)
	}
	if resp.DisplayImage.FetchURL != "/api/events/12/media/3/content" {
		t.Fatalf("unexpected display image fetch url: %q", resp.DisplayImage.FetchURL)
	}
	if resp.DisplayImage.StorageURI != "gs://drive-bucket/events/12/banner.png" {
		t.Fatalf("unexpected display image storage uri: %q", resp.DisplayImage.StorageURI)
	}
	if resp.Attachments[0].FileURL != "/api/events/12/media/4/content" {
		t.Fatalf("unexpected attachment file url: %q", resp.Attachments[0].FileURL)
	}
	if resp.Attachments[0].FetchURL != "/api/events/12/media/4/content" {
		t.Fatalf("unexpected attachment fetch url: %q", resp.Attachments[0].FetchURL)
	}
	if resp.Attachments[0].StorageURI != "gs://drive-bucket/events/12/agenda.pdf" {
		t.Fatalf("unexpected attachment storage uri: %q", resp.Attachments[0].StorageURI)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "events" WHERE "events"."id" = $1 ORDER BY "events"."id" LIMIT $2`)).
		WithArgs(99, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	if _, err := service.GetEvent(99); !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("expected ErrEventNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetEventMediaContent(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &EventService{DB: db, BucketName: "drive-bucket"}
	restore := stubMediaHooks()
	defer restore()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_media" WHERE event_id = $1 AND id = $2 ORDER BY "event_media"."id" LIMIT $3`)).
		WithArgs(12, 4, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "event_id", "media_role", "display_name", "gcp_object_key", "file_url", "mime_type", "file_size", "sort_order", "created_at", "updated_at",
		}).AddRow(
			4, 12, MediaRoleAttachment, "Agenda", "events/12/agenda.pdf", "gs://drive-bucket/events/12/agenda.pdf", "application/pdf", 20, 1, time.Now(), time.Now(),
		))

	resp, err := service.GetEventMediaContent(12, 4)
	if err != nil {
		t.Fatalf("GetEventMediaContent returned error: %v", err)
	}
	if string(resp.Content) != "downloaded:drive-bucket/events/12/agenda.pdf" {
		t.Fatalf("unexpected content: %q", string(resp.Content))
	}
	if resp.ContentType != "application/pdf" {
		t.Fatalf("unexpected content type: %q", resp.ContentType)
	}
	if resp.FileName != "Agenda.pdf" {
		t.Fatalf("unexpected file name: %q", resp.FileName)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_media" WHERE event_id = $1 AND id = $2 ORDER BY "event_media"."id" LIMIT $3`)).
		WithArgs(99, 4, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	if _, err := service.GetEventMediaContent(99, 4); !errors.Is(err, ErrEventMediaNotFound) {
		t.Fatalf("expected ErrEventMediaNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetEventMediaContentWithBucketPrefix(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &EventService{DB: db, BucketName: "drive-bucket", BucketPrefix: "main-folder"}
	restore := stubMediaHooks()
	defer restore()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_media" WHERE event_id = $1 AND id = $2 ORDER BY "event_media"."id" LIMIT $3`)).
		WithArgs(12, 4, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "event_id", "media_role", "display_name", "gcp_object_key", "file_url", "mime_type", "file_size", "sort_order", "created_at", "updated_at",
		}).AddRow(
			4, 12, MediaRoleAttachment, "Agenda", "events/12/agenda.pdf", "gs://drive-bucket/main-folder/events/12/agenda.pdf", "application/pdf", 20, 1, time.Now(), time.Now(),
		))

	resp, err := service.GetEventMediaContent(12, 4)
	if err != nil {
		t.Fatalf("GetEventMediaContent returned error: %v", err)
	}
	if string(resp.Content) != "downloaded:drive-bucket/main-folder/events/12/agenda.pdf" {
		t.Fatalf("unexpected prefixed content: %q", string(resp.Content))
	}
}

func TestCreateEventSuccess(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &EventService{DB: db, BucketName: "drive-bucket"}
	restore := stubMediaHooks()
	defer restore()

	startAt := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	endAt := startAt.Add(2 * time.Hour)
	regStart := startAt.Add(-24 * time.Hour)
	regEnd := startAt.Add(-2 * time.Hour)
	req := SaveEventRequest{
		Title:                       "Spring Fair",
		ShowTitle:                   true,
		Categories:                  []string{"Events"},
		EventType:                   "single_day_partial",
		StartAt:                     startAt,
		EndAt:                       &endAt,
		PrivacyType:                 "private",
		PrivateAudiences:            []string{"Members"},
		Published:                   false,
		RequestReview:               true,
		ReviewEmailList:             []string{"review@example.com"},
		Teaser:                      "Welcome!",
		DescriptionHTML:             "<p>Hello</p>",
		LocationMode:                "address",
		Address:                     &EventAddressInput{Name: "Hall", AddressLine1: "1 Main", City: "Toronto", ProvinceState: "ON", PostalCode: "A1A1A1", Country: "Canada", IsSaved: true},
		ShowDisplayImageWhenViewing: true,
		RegistrationEnabled:         true,
		RegistrationStartAt:         &regStart,
		RegistrationEndAt:           &regEnd,
		RegistrationURL:             "https://example.com/register",
		RepeatEnabled:               true,
		RecurrenceType:              "recurring",
		RecurrenceFrequency:         "weekly",
		RecurrenceInterval:          2,
		RecurrenceRule:              json.RawMessage(`{"byDay":["MO"]}`),
		DisplayImage:                &EventUploadInput{FileName: "banner.png", MimeType: "image/png", DataBase64: "aGVsbG8=", DisplayName: "Banner"},
		Attachments: []EventUploadInput{
			{FileName: "agenda.pdf", MimeType: "application/pdf", DataBase64: "aGVsbG8=", DisplayName: "Agenda"},
		},
		Occurrences: []EventOccurrenceInput{
			{OccurrenceStartAt: startAt, OccurrenceEndAt: &endAt, OccurrenceKind: "scheduled"},
		},
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "addresses"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "events"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "event_media"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(21))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "event_media"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(22))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "event_occurrences" WHERE event_id = $1`)).
		WithArgs(11).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "event_occurrences"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(31))
	mock.ExpectCommit()

	resp, err := service.CreateEvent(req)
	if err != nil {
		t.Fatalf("CreateEvent returned error: %v", err)
	}
	if resp.ID != 11 || resp.Title != "Spring Fair" || resp.Published {
		t.Fatalf("unexpected response: %#v", resp)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCreateEventValidationAndUploadErrors(t *testing.T) {
	service := &EventService{BucketName: "drive-bucket"}

	if _, err := normalizeSaveEventRequest(SaveEventRequest{}); err == nil {
		t.Fatal("expected validation error")
	}

	req := validSaveEventRequest()
	req.DisplayImage = &EventUploadInput{FileName: "banner.png", MimeType: "image/png", DataBase64: "aGVsbG8="}
	req.Attachments = nil

	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	service.DB = db

	restore := stubMediaHooksWithError(errors.New("upload failed"), nil)
	defer restore()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "events"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectRollback()

	if _, err := service.CreateEvent(req); err == nil {
		t.Fatal("expected upload error")
	}
}

func TestCreateEventWithExistingAddressAndNoMedia(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &EventService{DB: db, BucketName: "drive-bucket"}
	req := validSaveEventRequest()
	req.LocationMode = "address"
	req.Address = &EventAddressInput{ID: intPtr(3)}
	req.DisplayImage = nil
	req.Attachments = nil
	req.Occurrences = nil

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "addresses" WHERE "addresses"."id" = $1 ORDER BY "addresses"."id" LIMIT $2`)).
		WithArgs(3, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "address_line_1", "address_line_2", "city", "province_state", "postal_code", "country", "is_saved", "created_at", "updated_at"}).
			AddRow(3, "Hall", "1 Main", "", "Toronto", "ON", "A1A1A1", "Canada", true, time.Now(), time.Now()))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "events"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "event_occurrences" WHERE event_id = $1`)).
		WithArgs(11).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	resp, err := service.CreateEvent(req)
	if err != nil {
		t.Fatalf("CreateEvent returned error: %v", err)
	}
	if resp.ID != 11 {
		t.Fatalf("expected id 11, got %d", resp.ID)
	}
}

func TestResolveAddressForUpdateReusesExistingTemporaryAddress(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &EventService{DB: db}
	currentAddressID := 7
	input := &EventAddressInput{
		Name:          "Community Hall",
		AddressLine1:  "1 Main",
		AddressLine2:  "Suite B",
		City:          "Toronto",
		ProvinceState: "ON",
		PostalCode:    "A1A1A1",
		Country:       "Canada",
		IsSaved:       false,
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "addresses" WHERE "addresses"."id" = $1 ORDER BY "addresses"."id" LIMIT $2`)).
		WithArgs(7, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "address_line_1", "address_line_2", "city", "province_state", "postal_code", "country", "is_saved", "created_at", "updated_at"}).
			AddRow(7, "Old Hall", "9 Old", "", "Toronto", "ON", "Z9Z9Z9", "Canada", false, time.Now(), time.Now()))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "addresses" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	addressID, err := service.resolveAddressForUpdate(db.Session(&gorm.Session{SkipDefaultTransaction: true}), &currentAddressID, "address", input)
	if err != nil {
		t.Fatalf("resolveAddressForUpdate returned error: %v", err)
	}
	if addressID == nil || *addressID != currentAddressID {
		t.Fatalf("expected address id %d to be reused, got %#v", currentAddressID, addressID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCleanupUnusedTemporaryAddressDeletesUnreferencedRow(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &EventService{DB: db}
	addressID := 9

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "addresses" WHERE "addresses"."id" = $1 ORDER BY "addresses"."id" LIMIT $2`)).
		WithArgs(9, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "address_line_1", "address_line_2", "city", "province_state", "postal_code", "country", "is_saved", "created_at", "updated_at"}).
			AddRow(9, "Temp Hall", "1 Main", "", "Toronto", "ON", "A1A1A1", "Canada", false, time.Now(), time.Now()))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "events" WHERE address_id = $1`)).
		WithArgs(9).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "addresses" WHERE "addresses"."id" = $1`)).
		WithArgs(9).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := service.cleanupUnusedTemporaryAddress(db.Session(&gorm.Session{SkipDefaultTransaction: true}), &addressID, nil); err != nil {
		t.Fatalf("cleanupUnusedTemporaryAddress returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestUpdateEventSuccessAndNotFound(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &EventService{DB: db, BucketName: "drive-bucket"}
	restore := stubMediaHooks()
	defer restore()

	req := validSaveEventRequest()
	req.DisplayImage = &EventUploadInput{FileName: "new-banner.png", MimeType: "image/png", DataBase64: "aGVsbG8=", DisplayName: "Banner"}
	req.Attachments = []EventUploadInput{{FileURL: "gs://drive-bucket/events/1/agenda.pdf", ObjectKey: "events/1/agenda.pdf"}}
	req.Occurrences = []EventOccurrenceInput{{OccurrenceStartAt: req.StartAt, OccurrenceKind: "generated"}}

	startAt := req.StartAt
	endAt := req.EndAt

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "events" WHERE "events"."id" = $1 ORDER BY "events"."id" LIMIT $2`)).
		WithArgs(5, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "show_title", "categories", "event_type", "start_at", "end_at", "privacy_type", "private_audiences",
			"published", "request_review", "review_email_list", "teaser", "description_html", "contact_name", "contact_email",
			"contact_phone", "contact_ext", "contact_fax", "location_mode", "address_id", "show_display_image_when_viewing",
			"gallery_id", "registration_enabled", "registration_start_at", "registration_end_at", "registration_url",
			"repeat_enabled", "recurrence_type", "recurrence_frequency", "recurrence_interval", "recurrence_until",
			"recurrence_rule", "created_by", "created_at", "updated_at",
		}).AddRow(
			5, "Old Fair", true, pq.Array([]string{"Events"}), "single_day_all_day", startAt, endAt, "public", pq.Array([]string{}),
			false, false, pq.Array([]string{}), "Old teaser", "", "", "", "", "", "", "none", nil, true,
			nil, false, nil, nil, "", false, "", "", 1, nil, []byte(`null`), nil, startAt, startAt,
		))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "events" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_media" WHERE event_id = $1 AND media_role = $2 ORDER BY "event_media"."id" LIMIT $3`)).
		WithArgs(5, MediaRoleDisplayImage, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "event_id", "media_role", "display_name", "gcp_object_key", "file_url", "mime_type", "file_size", "sort_order", "created_at", "updated_at"}).
			AddRow(2, 5, MediaRoleDisplayImage, "Old Banner", "events/5/old-banner.png", "gs://drive-bucket/events/5/old-banner.png", "image/png", 10, 0, startAt, startAt))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "event_media" WHERE "event_media"."id" = $1`)).
		WithArgs(2).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "event_media"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(40))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "event_media"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(41))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "event_occurrences" WHERE event_id = $1`)).
		WithArgs(5).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "event_occurrences"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(51))
	mock.ExpectCommit()

	resp, err := service.UpdateEvent(5, req)
	if err != nil {
		t.Fatalf("UpdateEvent returned error: %v", err)
	}
	if resp.ID != 5 || resp.Title != req.Title {
		t.Fatalf("unexpected response: %#v", resp)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "events" WHERE "events"."id" = $1 ORDER BY "events"."id" LIMIT $2`)).
		WithArgs(99, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectRollback()

	if _, err := service.UpdateEvent(99, req); !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("expected ErrEventNotFound, got %v", err)
	}
}

func TestDeleteEventAndMediaFlows(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &EventService{DB: db, BucketName: "drive-bucket"}
	restore := stubMediaHooks()
	defer restore()

	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_media" WHERE event_id = $1`)).
		WithArgs(12).
		WillReturnRows(sqlmock.NewRows([]string{"id", "event_id", "media_role", "display_name", "gcp_object_key", "file_url", "mime_type", "file_size", "sort_order", "created_at", "updated_at"}).
			AddRow(1, 12, MediaRoleDisplayImage, "Banner", "events/12/banner.png", "gs://drive-bucket/events/12/banner.png", "image/png", 10, 0, now, now).
			AddRow(2, 12, MediaRoleAttachment, "Agenda", "events/12/agenda.pdf", "gs://drive-bucket/events/12/agenda.pdf", "application/pdf", 20, 0, now, now))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "events" WHERE "events"."id" = $1`)).
		WithArgs(12).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := service.DeleteEvent(12); err != nil {
		t.Fatalf("DeleteEvent returned error: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_media" WHERE event_id = $1 AND file_url = $2 ORDER BY "event_media"."id" LIMIT $3`)).
		WithArgs(12, "gs://drive-bucket/events/12/agenda.pdf", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "event_id", "media_role", "display_name", "gcp_object_key", "file_url", "mime_type", "file_size", "sort_order", "created_at", "updated_at"}).
			AddRow(2, 12, MediaRoleAttachment, "Agenda", "events/12/agenda.pdf", "gs://drive-bucket/events/12/agenda.pdf", "application/pdf", 20, 0, now, now))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "event_media" WHERE "event_media"."id" = $1`)).
		WithArgs(2).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := service.DeleteEventDocument(12, "gs://drive-bucket/events/12/agenda.pdf"); err != nil {
		t.Fatalf("DeleteEventDocument returned error: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_media" WHERE event_id = $1 AND media_role = $2`)).
		WithArgs(12, MediaRoleAttachment).
		WillReturnRows(sqlmock.NewRows([]string{"id", "event_id", "media_role", "display_name", "gcp_object_key", "file_url", "mime_type", "file_size", "sort_order", "created_at", "updated_at"}).
			AddRow(2, 12, MediaRoleAttachment, "Agenda", "events/12/agenda.pdf", "gs://drive-bucket/events/12/agenda.pdf", "application/pdf", 20, 0, now, now))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "event_media" WHERE "event_media"."id" = $1`)).
		WithArgs(2).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	resp, err := service.DeleteAllEventDocuments(12, nil)
	if err != nil {
		t.Fatalf("DeleteAllEventDocuments returned error: %v", err)
	}
	if resp.DeletedCount != 1 {
		t.Fatalf("expected deleted count 1, got %d", resp.DeletedCount)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_media" WHERE event_id = $1 AND file_url = $2 ORDER BY "event_media"."id" LIMIT $3`)).
		WithArgs(12, "gs://drive-bucket/events/12/banner.png", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "event_id", "media_role", "display_name", "gcp_object_key", "file_url", "mime_type", "file_size", "sort_order", "created_at", "updated_at"}).
			AddRow(1, 12, MediaRoleDisplayImage, "Banner", "events/12/banner.png", "gs://drive-bucket/events/12/banner.png", "image/png", 10, 0, now, now))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "event_media" WHERE "event_media"."id" = $1`)).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := service.DeleteEventPhoto(12, "gs://drive-bucket/events/12/banner.png"); err != nil {
		t.Fatalf("DeleteEventPhoto returned error: %v", err)
	}
}

func TestDeleteEventErrorsAndHelpers(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &EventService{DB: db, BucketName: "drive-bucket"}
	restore := stubMediaHooksWithError(nil, errors.New("delete failed"))
	defer restore()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_media" WHERE event_id = $1`)).
		WithArgs(55).
		WillReturnRows(sqlmock.NewRows([]string{"id", "event_id", "media_role", "display_name", "gcp_object_key", "file_url", "mime_type", "file_size", "sort_order", "created_at", "updated_at"}))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "events" WHERE "events"."id" = $1`)).
		WithArgs(55).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	if err := service.DeleteEvent(55); !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("expected ErrEventNotFound, got %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_media" WHERE event_id = $1 AND file_url = $2 ORDER BY "event_media"."id" LIMIT $3`)).
		WithArgs(12, "gs://drive-bucket/events/12/agenda.pdf", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "event_id", "media_role", "display_name", "gcp_object_key", "file_url", "mime_type", "file_size", "sort_order", "created_at", "updated_at"}).
			AddRow(2, 12, MediaRoleDisplayImage, "Banner", "events/12/banner.png", "gs://drive-bucket/events/12/banner.png", "image/png", 10, 0, time.Now(), time.Now()))
	mock.ExpectRollback()

	if err := service.DeleteEventDocument(12, "gs://drive-bucket/events/12/agenda.pdf"); !errors.Is(err, ErrEventMediaNotFound) {
		t.Fatalf("expected ErrEventMediaNotFound, got %v", err)
	}

	if err := service.cleanupSingleMediaObject(EventMedia{FileURL: "gs://drive-bucket/events/1/file.pdf"}); err == nil {
		t.Fatal("expected cleanupSingleMediaObject to return delete error")
	}

	if err := service.cleanupSingleMediaObject(EventMedia{}); err != nil {
		t.Fatalf("expected empty media cleanup to succeed, got %v", err)
	}

	service.BucketName = ""
	if err := service.cleanupSingleMediaObject(EventMedia{GCPObjectKey: "events/1/file.pdf"}); !errors.Is(err, ErrMediaBucketNotConfigured) {
		t.Fatalf("expected ErrMediaBucketNotConfigured, got %v", err)
	}

	service.BucketName = "drive-bucket"
	req := validSaveEventRequest()
	req.LocationMode = "address"
	req.Address = &EventAddressInput{ID: intPtr(9)}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "events" WHERE "events"."id" = $1 ORDER BY "events"."id" LIMIT $2`)).
		WithArgs(5, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "show_title", "categories", "event_type", "start_at", "end_at", "privacy_type", "private_audiences",
			"published", "request_review", "review_email_list", "teaser", "description_html", "contact_name", "contact_email",
			"contact_phone", "contact_ext", "contact_fax", "location_mode", "address_id", "show_display_image_when_viewing",
			"gallery_id", "registration_enabled", "registration_start_at", "registration_end_at", "registration_url",
			"repeat_enabled", "recurrence_type", "recurrence_frequency", "recurrence_interval", "recurrence_until",
			"recurrence_rule", "created_by", "created_at", "updated_at",
		}).AddRow(
			5, "Old Fair", true, pq.Array([]string{"Events"}), "single_day_all_day", req.StartAt, req.EndAt, "public", pq.Array([]string{}),
			false, false, pq.Array([]string{}), "Old teaser", "", "", "", "", "", "", "none", nil, true,
			nil, false, nil, nil, "", false, "", "", 1, nil, []byte(`null`), nil, req.StartAt, req.StartAt,
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "addresses" WHERE "addresses"."id" = $1 ORDER BY "addresses"."id" LIMIT $2`)).
		WithArgs(9, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectRollback()

	if _, err := service.UpdateEvent(5, req); err == nil {
		t.Fatal("expected address lookup error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestServiceAdditionalErrorBranches(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &EventService{DB: db, BucketName: "drive-bucket"}
	restore := stubMediaHooks()
	defer restore()

	req := validSaveEventRequest()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "events" WHERE "events"."id" = $1 ORDER BY "events"."id" LIMIT $2`)).
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "show_title", "categories", "event_type", "start_at", "end_at", "privacy_type", "private_audiences",
			"published", "request_review", "review_email_list", "teaser", "description_html", "contact_name", "contact_email",
			"contact_phone", "contact_ext", "contact_fax", "location_mode", "address_id", "show_display_image_when_viewing",
			"gallery_id", "registration_enabled", "registration_start_at", "registration_end_at", "registration_url",
			"repeat_enabled", "recurrence_type", "recurrence_frequency", "recurrence_interval", "recurrence_until",
			"recurrence_rule", "created_by", "created_at", "updated_at",
		}).AddRow(
			1, "Old Fair", true, pq.Array([]string{"Events"}), "single_day_all_day", req.StartAt, req.EndAt, "public", pq.Array([]string{}),
			false, false, pq.Array([]string{}), "Old teaser", "", "", "", "", "", "", "none", nil, true,
			nil, false, nil, nil, "", false, "", "", 1, nil, []byte(`null`), nil, req.StartAt, req.StartAt,
		))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "events" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_media" WHERE event_id = $1 AND media_role = $2 ORDER BY "event_media"."id" LIMIT $3`)).
		WithArgs(1, MediaRoleDisplayImage, 1).
		WillReturnError(errors.New("query failed"))
	mock.ExpectRollback()

	req.DisplayImage = &EventUploadInput{FileName: "banner.png", MimeType: "image/png", DataBase64: "aGVsbG8="}
	if _, err := service.UpdateEvent(1, req); err == nil {
		t.Fatal("expected update display-image query error")
	}

	req = validSaveEventRequest()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_media" WHERE event_id = $1 AND file_url = $2 ORDER BY "event_media"."id" LIMIT $3`)).
		WithArgs(1, "gs://drive-bucket/events/1/banner.png", 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectRollback()
	if err := service.DeleteEventPhoto(1, "gs://drive-bucket/events/1/banner.png"); !errors.Is(err, ErrEventMediaNotFound) {
		t.Fatalf("expected ErrEventMediaNotFound, got %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_media" WHERE event_id = $1 AND media_role = $2`)).
		WithArgs(1, MediaRoleAttachment).
		WillReturnError(errors.New("find failed"))
	mock.ExpectRollback()
	if _, err := service.DeleteAllEventDocuments(1, nil); err == nil {
		t.Fatal("expected delete-all find error")
	}

	req = validSaveEventRequest()
	req.Occurrences = []EventOccurrenceInput{{OccurrenceStartAt: req.StartAt, OccurrenceKind: ""}}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "events"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "event_occurrences" WHERE event_id = $1`)).
		WithArgs(9).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "event_occurrences"`)).
		WillReturnError(errors.New("insert failed"))
	mock.ExpectRollback()
	if _, err := service.CreateEvent(req); err == nil {
		t.Fatal("expected occurrence insert error")
	}
}

func TestNormalizeHelpersAndUtilityBranches(t *testing.T) {
	req := validSaveEventRequest()
	req.Published = false
	req.RequestReview = true
	req.PrivacyType = "private"
	req.Categories = []string{" ", "Events "}
	req.PrivateAudiences = []string{" Members "}
	req.ReviewEmailList = []string{" review@example.com "}
	req.LocationMode = "address"
	req.Address = &EventAddressInput{Name: " Hall ", AddressLine1: " 1 Main ", City: " Toronto ", ProvinceState: " ON ", PostalCode: " A1A1A1 ", Country: " Canada "}
	req.RecurrenceRule = json.RawMessage(`{"ok":true}`)
	req.Occurrences = []EventOccurrenceInput{{OccurrenceStartAt: req.StartAt, OccurrenceKind: "generated"}}

	normalized, err := normalizeSaveEventRequest(req)
	if err != nil {
		t.Fatalf("normalizeSaveEventRequest returned error: %v", err)
	}
	if len(normalized.Categories) != 1 || normalized.Categories[0] != "Events" {
		t.Fatalf("expected trimmed categories, got %#v", normalized.Categories)
	}
	if normalized.Teaser != "Welcome!" {
		t.Fatalf("expected teaser to be preserved, got %q", normalized.Teaser)
	}

	req = validSaveEventRequest()
	req.Teaser = "   "
	normalized, err = normalizeSaveEventRequest(req)
	if err != nil {
		t.Fatalf("expected blank teaser to be allowed, got %v", err)
	}
	if normalized.Teaser != "" {
		t.Fatalf("expected teaser to normalize to empty string, got %q", normalized.Teaser)
	}

	if !isAllowed("public", "public", "private") {
		t.Fatal("expected isAllowed to accept matching value")
	}

	req = validSaveEventRequest()
	req.PrivacyType = "private"
	req.PrivateAudiences = nil
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected private audience validation error")
	}

	req = validSaveEventRequest()
	req.PrivateAudiences = []string{"Members"}
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected public audience validation error")
	}

	req = validSaveEventRequest()
	req.PrivacyType = "secret"
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected invalid privacy validation error")
	}

	req = validSaveEventRequest()
	req.LocationMode = "bad"
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected invalid location validation error")
	}

	req = validSaveEventRequest()
	req.LocationMode = "address"
	req.Address = nil
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected missing address validation error")
	}

	req = validSaveEventRequest()
	req.LocationMode = "address"
	req.Address = &EventAddressInput{}
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected blank new address validation error")
	}

	req = validSaveEventRequest()
	req.RequestReview = true
	req.ReviewEmailList = nil
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected review email validation error")
	}

	req = validSaveEventRequest()
	req.RequestReview = false
	req.ReviewEmailList = []string{"review@example.com"}
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected review_email_list false-state validation error")
	}

	req = validSaveEventRequest()
	req.RequestReview = true
	req.ReviewEmailList = []string{"review@example.com"}
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected published review conflict validation error")
	}

	req = validSaveEventRequest()
	req.RegistrationEnabled = true
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected registration validation error")
	}

	req = validSaveEventRequest()
	req.RegistrationEnabled = true
	start := req.StartAt
	end := start.Add(-time.Hour)
	req.RegistrationStartAt = &start
	req.RegistrationEndAt = &end
	req.RegistrationURL = "https://example.com"
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected registration end validation error")
	}

	req = validSaveEventRequest()
	req.RegistrationEnabled = true
	req.RegistrationStartAt = &start
	req.RegistrationEndAt = req.EndAt
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected registration url validation error")
	}

	req = validSaveEventRequest()
	req.RepeatEnabled = true
	req.RecurrenceType = "bad"
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected recurrence type validation error")
	}

	req = validSaveEventRequest()
	req.RepeatEnabled = true
	req.RecurrenceType = "recurring"
	req.RecurrenceFrequency = "nope"
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected recurrence validation error")
	}

	req = validSaveEventRequest()
	req.RepeatEnabled = true
	req.RecurrenceType = "recurring"
	req.RecurrenceFrequency = "weekly"
	req.RecurrenceInterval = -1
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected recurrence interval validation error")
	}

	req = validSaveEventRequest()
	req.Occurrences = []EventOccurrenceInput{{OccurrenceStartAt: req.StartAt, OccurrenceEndAt: timePtr(req.StartAt.Add(-time.Hour)), OccurrenceKind: "scheduled"}}
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected occurrence validation error")
	}

	req = validSaveEventRequest()
	req.Occurrences = []EventOccurrenceInput{{OccurrenceStartAt: req.StartAt, OccurrenceKind: "bad"}}
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected invalid occurrence kind error")
	}

	req = validSaveEventRequest()
	req.RepeatEnabled = true
	req.RecurrenceType = "scheduled"
	req.RecurrenceRule = json.RawMessage(`{"ok":`)
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected invalid json recurrence rule error")
	}

	req = validSaveEventRequest()
	badEnd := req.StartAt.Add(-time.Hour)
	req.EndAt = &badEnd
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected end before start validation error")
	}

	req = validSaveEventRequest()
	req.EventType = "single_day_all_day"
	req.EndAt = nil
	if _, err := normalizeSaveEventRequest(req); err != nil {
		t.Fatalf("expected single_day_all_day without end_at to be valid, got %v", err)
	}

	req = validSaveEventRequest()
	req.EventType = "single_day_all_day"
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected single_day_all_day end_at validation error")
	}

	req = validSaveEventRequest()
	req.EndAt = nil
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected single_day_partial end_at required validation error")
	}

	req = validSaveEventRequest()
	nextDay := req.StartAt.Add(24 * time.Hour)
	req.EndAt = &nextDay
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected single_day_partial same-day validation error")
	}

	req = validSaveEventRequest()
	req.EventType = "multi_day_all_day"
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected multi_day_all_day later-date validation error")
	}

	req = validSaveEventRequest()
	req.EventType = "multi_day_partial"
	if _, err := normalizeSaveEventRequest(req); err == nil {
		t.Fatal("expected multi_day_partial later-date validation error")
	}

	req = validSaveEventRequest()
	req.EventType = "multi_day_all_day"
	req.EndAt = &nextDay
	if _, err := normalizeSaveEventRequest(req); err != nil {
		t.Fatalf("expected multi_day_all_day to accept later end date, got %v", err)
	}

	req = validSaveEventRequest()
	req.EventType = "multi_day_partial"
	req.EndAt = &nextDay
	if _, err := normalizeSaveEventRequest(req); err != nil {
		t.Fatalf("expected multi_day_partial to accept later end date, got %v", err)
	}

	service := &EventService{BucketName: "drive-bucket"}
	restore := stubMediaHooks()
	defer restore()

	media, uploadedObject, err := service.buildMediaRecord(1, MediaRoleAttachment, 0, EventUploadInput{FileURL: "https://storage.googleapis.com/drive-bucket/events/1/file.pdf"})
	if err != nil {
		t.Fatalf("buildMediaRecord returned error: %v", err)
	}
	if uploadedObject != "" || media.GCPObjectKey != "events/1/file.pdf" {
		t.Fatalf("unexpected buildMediaRecord result: %#v object=%q", media, uploadedObject)
	}

	media, uploadedObject, err = service.buildMediaRecord(1, MediaRoleAttachment, 0, EventUploadInput{
		FileURL:      "/api/events/1/media/9/content",
		StorageURI:   "gs://drive-bucket/events/1/file.pdf",
		GCPObjectKey: "events/1/file.pdf",
	})
	if err != nil {
		t.Fatalf("buildMediaRecord with storage alias returned error: %v", err)
	}
	if uploadedObject != "" || media.FileURL != "gs://drive-bucket/events/1/file.pdf" || media.GCPObjectKey != "events/1/file.pdf" {
		t.Fatalf("unexpected alias buildMediaRecord result: %#v object=%q", media, uploadedObject)
	}

	service.BucketPrefix = "main-folder"
	media, uploadedObject, err = service.buildMediaRecord(1, MediaRoleAttachment, 0, EventUploadInput{
		FileName:   "Agenda.pdf",
		MimeType:   "application/pdf",
		DataBase64: "aGVsbG8=",
	})
	if err != nil {
		t.Fatalf("buildMediaRecord with bucket prefix returned error: %v", err)
	}
	if uploadedObject != "main-folder/events/1/attachment_20260501100000_1_agenda.pdf" {
		t.Fatalf("unexpected uploaded object with prefix: %q", uploadedObject)
	}
	if media.GCPObjectKey != "events/1/attachment_20260501100000_1_agenda.pdf" {
		t.Fatalf("unexpected stored gcp_object_key: %q", media.GCPObjectKey)
	}
	if media.FileURL != "gs://drive-bucket/main-folder/events/1/attachment_20260501100000_1_agenda.pdf" {
		t.Fatalf("unexpected stored file url: %q", media.FileURL)
	}

	service.BucketName = ""
	if _, _, err := service.buildMediaRecord(1, MediaRoleAttachment, 0, EventUploadInput{DataBase64: "aGVsbG8="}); !errors.Is(err, ErrMediaBucketNotConfigured) {
		t.Fatalf("expected ErrMediaBucketNotConfigured, got %v", err)
	}

	service.BucketName = "drive-bucket"
	if got := service.mediaObjectName(1, MediaRoleAttachment, 1, "Agenda.pdf", "application/pdf"); got == "" {
		t.Fatal("expected mediaObjectName to build a value")
	}
	if got := service.mediaObjectName(1, MediaRoleAttachment, 1, "", ""); got == "" {
		t.Fatal("expected fallback mediaObjectName to build a value")
	}

	if err := service.cleanupMediaObjects([]EventMedia{{GCPObjectKey: "events/1/a"}, {GCPObjectKey: "events/1/b"}}); err != nil {
		t.Fatalf("expected cleanupMediaObjects success, got %v", err)
	}
}

func TestModelHelpersAndJSONRawMessage(t *testing.T) {
	if (Event{}).TableName() != "events" {
		t.Fatal("unexpected events table name")
	}
	if (Address{}).TableName() != "addresses" {
		t.Fatal("unexpected addresses table name")
	}
	if (Gallery{}).TableName() != "galleries" {
		t.Fatal("unexpected galleries table name")
	}
	if (EventMedia{}).TableName() != "event_media" {
		t.Fatal("unexpected event_media table name")
	}
	if (EventOccurrence{}).TableName() != "event_occurrences" {
		t.Fatal("unexpected event_occurrences table name")
	}

	var raw JSONRawMessage
	if err := raw.Scan(nil); err != nil {
		t.Fatalf("expected nil scan to succeed, got %v", err)
	}
	if err := raw.Scan("text"); err != nil {
		t.Fatalf("expected string scan to succeed, got %v", err)
	}
	if err := raw.Scan([]byte(`{"ok":true}`)); err != nil {
		t.Fatalf("expected []byte scan to succeed, got %v", err)
	}
	if _, err := raw.Value(); err != nil {
		t.Fatalf("expected value conversion to succeed, got %v", err)
	}
	if err := raw.Scan(123); err == nil {
		t.Fatal("expected unsupported scan type error")
	}
}

func TestApplyEventRequestPreservesCreatedBy(t *testing.T) {
	creatorID := 7
	event := &Event{CreatedBy: &creatorID}
	replacementID := 99
	req := validSaveEventRequest()
	req.CreatedBy = &replacementID

	applyEventRequest(event, req)

	if event.CreatedBy == nil || *event.CreatedBy != creatorID {
		t.Fatalf("expected created_by %d to be preserved, got %#v", creatorID, event.CreatedBy)
	}
}

func TestCreateEventErrorBranches(t *testing.T) {
	req := validSaveEventRequest()

	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	service := &EventService{DB: db, BucketName: "drive-bucket"}
	restore := stubMediaHooks()
	defer restore()

	req.LocationMode = "address"
	req.Address = &EventAddressInput{Name: "Hall", AddressLine1: "1 Main", City: "Toronto", ProvinceState: "ON", PostalCode: "A1A1A1", Country: "Canada"}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "addresses"`)).
		WillReturnError(errors.New("insert address failed"))
	mock.ExpectRollback()
	if _, err := service.CreateEvent(req); err == nil {
		t.Fatal("expected address insert error")
	}

	db, mock, cleanup = setupMockDB(t)
	defer cleanup()
	service = &EventService{DB: db, BucketName: "drive-bucket"}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "events"`)).
		WillReturnError(errors.New("insert event failed"))
	mock.ExpectRollback()
	if _, err := service.CreateEvent(validSaveEventRequest()); err == nil {
		t.Fatal("expected event insert error")
	}

	db, mock, cleanup = setupMockDB(t)
	defer cleanup()
	service = &EventService{DB: db, BucketName: "drive-bucket"}
	req = validSaveEventRequest()
	req.Attachments = []EventUploadInput{{}}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "events"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectRollback()
	if _, err := service.CreateEvent(req); err == nil {
		t.Fatal("expected attachment validation error")
	}

	db, mock, cleanup = setupMockDB(t)
	defer cleanup()
	service = &EventService{DB: db, BucketName: "drive-bucket"}
	req = validSaveEventRequest()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "events"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "event_occurrences" WHERE event_id = $1`)).
		WithArgs(11).
		WillReturnError(errors.New("delete occurrences failed"))
	mock.ExpectRollback()
	if _, err := service.CreateEvent(req); err == nil {
		t.Fatal("expected occurrence delete error")
	}

	db, mock, cleanup = setupMockDB(t)
	defer cleanup()
	service = &EventService{DB: db, BucketName: "drive-bucket"}
	req = validSaveEventRequest()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "events"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "event_occurrences" WHERE event_id = $1`)).
		WithArgs(11).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit().WillReturnError(errors.New("commit failed"))
	if _, err := service.CreateEvent(req); err == nil {
		t.Fatal("expected commit error")
	}
}

func TestUpdateEventErrorBranches(t *testing.T) {
	baseReq := validSaveEventRequest()

	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	service := &EventService{DB: db, BucketName: "drive-bucket"}
	restore := stubMediaHooks()
	defer restore()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "events" WHERE "events"."id" = $1 ORDER BY "events"."id" LIMIT $2`)).
		WithArgs(5, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "show_title", "categories", "event_type", "start_at", "end_at", "privacy_type", "private_audiences",
			"published", "request_review", "review_email_list", "teaser", "description_html", "contact_name", "contact_email",
			"contact_phone", "contact_ext", "contact_fax", "location_mode", "address_id", "show_display_image_when_viewing",
			"gallery_id", "registration_enabled", "registration_start_at", "registration_end_at", "registration_url",
			"repeat_enabled", "recurrence_type", "recurrence_frequency", "recurrence_interval", "recurrence_until",
			"recurrence_rule", "created_by", "created_at", "updated_at",
		}).AddRow(
			5, "Old Fair", true, pq.Array([]string{"Events"}), "single_day_all_day", baseReq.StartAt, baseReq.EndAt, "public", pq.Array([]string{}),
			false, false, pq.Array([]string{}), "Old teaser", "", "", "", "", "", "", "none", nil, true,
			nil, false, nil, nil, "", false, "", "", 1, nil, []byte(`null`), nil, baseReq.StartAt, baseReq.StartAt,
		))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "events" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_media" WHERE event_id = $1 AND media_role = $2 ORDER BY "event_media"."id" LIMIT $3`)).
		WithArgs(5, MediaRoleDisplayImage, 1).
		WillReturnError(errors.New("media lookup failed"))
	mock.ExpectRollback()
	req := validSaveEventRequest()
	req.DisplayImage = &EventUploadInput{FileName: "banner.png", MimeType: "image/png", DataBase64: "aGVsbG8="}
	if _, err := service.UpdateEvent(5, req); err == nil {
		t.Fatal("expected display image lookup error")
	}

	db, mock, cleanup = setupMockDB(t)
	defer cleanup()
	service = &EventService{DB: db, BucketName: "drive-bucket"}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "events" WHERE "events"."id" = $1 ORDER BY "events"."id" LIMIT $2`)).
		WithArgs(5, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "show_title", "categories", "event_type", "start_at", "end_at", "privacy_type", "private_audiences",
			"published", "request_review", "review_email_list", "teaser", "description_html", "contact_name", "contact_email",
			"contact_phone", "contact_ext", "contact_fax", "location_mode", "address_id", "show_display_image_when_viewing",
			"gallery_id", "registration_enabled", "registration_start_at", "registration_end_at", "registration_url",
			"repeat_enabled", "recurrence_type", "recurrence_frequency", "recurrence_interval", "recurrence_until",
			"recurrence_rule", "created_by", "created_at", "updated_at",
		}).AddRow(
			5, "Old Fair", true, pq.Array([]string{"Events"}), "single_day_all_day", baseReq.StartAt, baseReq.EndAt, "public", pq.Array([]string{}),
			false, false, pq.Array([]string{}), "Old teaser", "", "", "", "", "", "", "none", nil, true,
			nil, false, nil, nil, "", false, "", "", 1, nil, []byte(`null`), nil, baseReq.StartAt, baseReq.StartAt,
		))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "events" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "event_occurrences" WHERE event_id = $1`)).
		WithArgs(5).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit().WillReturnError(errors.New("commit failed"))
	if _, err := service.UpdateEvent(5, validSaveEventRequest()); err == nil {
		t.Fatal("expected update commit error")
	}
}

func TestDeleteHelpersAndPanicRollback(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	service := &EventService{DB: db, BucketName: "drive-bucket"}
	restore := stubMediaHooks()
	defer restore()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_media" WHERE event_id = $1 AND media_role = $2`)).
		WithArgs(12, MediaRoleAttachment).
		WillReturnRows(sqlmock.NewRows([]string{"id", "event_id", "media_role", "display_name", "gcp_object_key", "file_url", "mime_type", "file_size", "sort_order", "created_at", "updated_at"}))
	mock.ExpectRollback()
	if _, err := service.DeleteAllEventDocuments(12, nil); err == nil {
		t.Fatal("expected delete all documents error")
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "event_media" WHERE event_id = $1 AND file_url = $2 ORDER BY "event_media"."id" LIMIT $3`)).
		WithArgs(12, "gs://drive-bucket/events/12/banner.png", 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectRollback()
	if err := service.DeleteEventPhoto(12, "gs://drive-bucket/events/12/banner.png"); !errors.Is(err, ErrEventMediaNotFound) {
		t.Fatalf("expected ErrEventMediaNotFound, got %v", err)
	}

	service.cleanupObjects([]string{"events/1/a.pdf", "", "events/1/b.pdf"})

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected rollbackOnPanic to re-panic")
			}
		}()
		tx := db.Begin()
		defer rollbackOnPanic(tx)
		panic("boom")
	}()
}

func validSaveEventRequest() SaveEventRequest {
	startAt := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	endAt := startAt.Add(2 * time.Hour)
	return SaveEventRequest{
		Title:                       "Spring Fair",
		ShowTitle:                   true,
		Categories:                  []string{"Events"},
		EventType:                   "single_day_partial",
		StartAt:                     startAt,
		EndAt:                       &endAt,
		PrivacyType:                 "public",
		Published:                   true,
		Teaser:                      "Welcome!",
		LocationMode:                "none",
		ShowDisplayImageWhenViewing: true,
	}
}

func TestBuildMediaRecordFromMultipartContent(t *testing.T) {
	svc := &EventService{BucketName: "drive-bucket", BucketPrefix: "main-folder"}
	restore := stubMediaHooks()
	defer restore()

	media, uploadedObject, err := svc.buildMediaRecord(12, MediaRoleDisplayImage, 0, EventUploadInput{
		DisplayName: "Poster",
		FileName:    "poster.png",
		MimeType:    "image/png",
		Content:     []byte("hello"),
	})
	if err != nil {
		t.Fatalf("buildMediaRecord returned error: %v", err)
	}
	if uploadedObject != "main-folder/events/12/display_image_20260501100000_1_poster.png" {
		t.Fatalf("unexpected uploaded object: %q", uploadedObject)
	}
	if media.GCPObjectKey != "events/12/display_image_20260501100000_1_poster.png" {
		t.Fatalf("unexpected object key: %q", media.GCPObjectKey)
	}
	if media.FileURL != "gs://drive-bucket/main-folder/events/12/display_image_20260501100000_1_poster.png" {
		t.Fatalf("unexpected file url: %q", media.FileURL)
	}
	if media.FileSize != 5 {
		t.Fatalf("expected file size 5, got %d", media.FileSize)
	}
}

func stubMediaHooks() func() {
	return stubMediaHooksWithError(nil, nil)
}

func stubMediaHooksWithError(uploadErr, deleteErr error) func() {
	prevUpload := uploadBase64ToGCSHook
	prevUploadBytes := uploadBytesToGCSHook
	prevDownload := downloadGCSObjectHook
	prevDelete := deleteGCSObjectHook
	prevNow := nowFunc
	uploadBase64ToGCSHook = func(base64Data, bucketName, objectName, contentType string) (string, int64, error) {
		if uploadErr != nil {
			return "", 0, uploadErr
		}
		return "gs://" + bucketName + "/" + objectName, int64(len(base64Data)), nil
	}
	uploadBytesToGCSHook = func(data []byte, bucketName, objectName, contentType string) (string, int64, error) {
		if uploadErr != nil {
			return "", 0, uploadErr
		}
		return "gs://" + bucketName + "/" + objectName, int64(len(data)), nil
	}
	downloadGCSObjectHook = func(bucketName, objectName string) ([]byte, string, error) {
		return []byte("downloaded:" + bucketName + "/" + objectName), "application/pdf", nil
	}
	deleteGCSObjectHook = func(bucketName, objectName string) error {
		return deleteErr
	}
	nowFunc = func() time.Time {
		return time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	}
	return func() {
		uploadBase64ToGCSHook = prevUpload
		uploadBytesToGCSHook = prevUploadBytes
		downloadGCSObjectHook = prevDownload
		deleteGCSObjectHook = prevDelete
		nowFunc = prevNow
	}
}

func intPtr(v int) *int {
	return &v
}

func timePtr(v time.Time) *time.Time {
	return &v
}
