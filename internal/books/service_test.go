package books

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBookVersionFieldSchemaDoesNotRequireFieldKey(t *testing.T) {
	t.Helper()

	if _, exists := reflect.TypeOf(BookVersionField{}).FieldByName("FieldKey"); exists {
		t.Fatal("BookVersionField unexpectedly includes FieldKey, which does not exist in the database schema")
	}
}

func TestNormalizeSaveBookVersionRequestAcceptsTypicalCreatePayload(t *testing.T) {
	t.Helper()

	req := validSaveBookVersionRequest()

	normalized, err := normalizeSaveBookVersionRequest(req, true)
	if err != nil {
		t.Fatalf("normalizeSaveBookVersionRequest returned error: %v", err)
	}

	if len(normalized.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(normalized.Fields))
	}
	if !jsonEqual(t, normalized.LayoutSettings, defaultBookLayoutSettings) {
		t.Fatalf("expected default layout settings to be applied, got %s", string(normalized.LayoutSettings))
	}
	if normalized.SourcePDF == nil || len(normalized.SourcePDF.Content) == 0 {
		t.Fatal("expected source PDF to be preserved during normalization")
	}
}

func TestNormalizeSaveBookVersionRequestIgnoresClientLayoutSettings(t *testing.T) {
	t.Helper()

	req := validSaveBookVersionRequest()
	req.LayoutSettings = json.RawMessage(`{"heading_area":{"font_size":24},"body_area":{"font_size":5}}`)

	normalized, err := normalizeSaveBookVersionRequest(req, true)
	if err != nil {
		t.Fatalf("normalizeSaveBookVersionRequest returned error: %v", err)
	}

	if !jsonEqual(t, normalized.LayoutSettings, defaultBookLayoutSettings) {
		t.Fatalf("expected client layout settings to be ignored in favor of backend defaults, got %s", string(normalized.LayoutSettings))
	}
}

func TestNormalizeSaveBookVersionRequestIgnoresClientGeneratedPDF(t *testing.T) {
	t.Helper()

	req := validSaveBookVersionRequest()
	req.GeneratedPDF = &BookUploadInput{
		FileName: "manual.pdf",
		MimeType: "application/pdf",
		Content:  []byte("%PDF-1.4 manual"),
	}

	normalized, err := normalizeSaveBookVersionRequest(req, true)
	if err != nil {
		t.Fatalf("normalizeSaveBookVersionRequest returned error: %v", err)
	}
	if normalized.GeneratedPDF != nil {
		t.Fatalf("expected client generated PDF upload to be ignored, got %#v", normalized.GeneratedPDF)
	}
}

func TestNormalizeSaveBookVersionRequestPreservesTemplatePDFUploads(t *testing.T) {
	t.Helper()

	req := validSaveBookVersionRequest()
	req.ContentTemplatePDF = &BookUploadInput{
		FileName: "content-template.pdf",
		MimeType: "application/pdf",
		Content:  []byte("%PDF-1.4 content"),
	}
	req.ContentImageTemplatePDF = &BookUploadInput{
		FileName: "content-image-template.pdf",
		MimeType: "application/pdf",
		Content:  []byte("%PDF-1.4 image"),
	}
	req.SectionTemplatePDF = &BookUploadInput{
		FileName: "section-template.pdf",
		MimeType: "application/pdf",
		Content:  []byte("%PDF-1.4 section"),
	}

	normalized, err := normalizeSaveBookVersionRequest(req, true)
	if err != nil {
		t.Fatalf("normalizeSaveBookVersionRequest returned error: %v", err)
	}
	if normalized.ContentTemplatePDF == nil || normalized.ContentTemplatePDF.FileName != "content-template.pdf" {
		t.Fatalf("expected content template upload to be preserved, got %#v", normalized.ContentTemplatePDF)
	}
	if normalized.ContentImageTemplatePDF == nil || normalized.ContentImageTemplatePDF.FileName != "content-image-template.pdf" {
		t.Fatalf("expected image content template upload to be preserved, got %#v", normalized.ContentImageTemplatePDF)
	}
	if normalized.SectionTemplatePDF == nil || normalized.SectionTemplatePDF.FileName != "section-template.pdf" {
		t.Fatalf("expected section template upload to be preserved, got %#v", normalized.SectionTemplatePDF)
	}
}

func TestApplyBookTemplatePDFUploadsToLayoutStoresTemplateRefs(t *testing.T) {
	t.Helper()

	layout := applyBookTemplatePDFUploadsToLayout(defaultBookLayoutSettings,
		storedBookUpload{FileName: "content-template.pdf", FileURL: "gs://bucket/content-template.pdf", StorageURI: "gs://bucket/content-template.pdf", ObjectKey: "books/content-template.pdf"},
		storedBookUpload{FileName: "content-image-template.pdf", FileURL: "gs://bucket/content-image-template.pdf", StorageURI: "gs://bucket/content-image-template.pdf", ObjectKey: "books/content-image-template.pdf"},
		storedBookUpload{FileName: "section-template.pdf", FileURL: "gs://bucket/section-template.pdf", StorageURI: "gs://bucket/section-template.pdf", ObjectKey: "books/section-template.pdf"},
	)
	parsed := parseBookLayoutSettings(layout)

	if parsed.ContentPage.TemplatePDF.ObjectKey != "books/content-template.pdf" || parsed.ContentPage.TemplatePDF.PageNumber != 1 {
		t.Fatalf("expected content template ref in layout, got %#v", parsed.ContentPage.TemplatePDF)
	}
	if parsed.ContentPage.ImageTemplatePDF.ObjectKey != "books/content-image-template.pdf" || parsed.ContentPage.ImageTemplatePDF.PageNumber != 1 {
		t.Fatalf("expected content image template ref in layout, got %#v", parsed.ContentPage.ImageTemplatePDF)
	}
	if parsed.SectionPage.TemplatePDF.ObjectKey != "books/section-template.pdf" || parsed.SectionPage.TemplatePDF.PageNumber != 1 {
		t.Fatalf("expected section template ref in layout, got %#v", parsed.SectionPage.TemplatePDF)
	}
}

func TestNormalizeSaveBookVersionRequestRejectsDuplicateFieldLabels(t *testing.T) {
	t.Helper()

	req := validSaveBookVersionRequest()
	req.Fields[1].Label = " submitter email "

	_, err := normalizeSaveBookVersionRequest(req, true)
	if err == nil {
		t.Fatal("expected duplicate field labels to fail validation")
	}
	if !strings.Contains(err.Error(), "fields[1].label duplicates fields[0].label") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestNormalizeSaveBookVersionRequestRejectsRichTextEmailField(t *testing.T) {
	t.Helper()

	req := validSaveBookVersionRequest()
	req.Fields[0].InputType = BookFieldInputTypeRichText

	_, err := normalizeSaveBookVersionRequest(req, true)
	if err == nil {
		t.Fatal("expected email field with rich_text input type to fail validation")
	}
	if !strings.Contains(err.Error(), "fields[0].input_type must be single_line when is_email_field is true") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateManualInitialVersionNumberAllowsOnlyFirstVersion(t *testing.T) {
	t.Helper()

	if err := validateManualInitialVersionNumber(1); err != nil {
		t.Fatalf("expected version number 1 to be allowed, got %v", err)
	}

	err := validateManualInitialVersionNumber(2)
	if err == nil {
		t.Fatal("expected manual creation to fail after the first version")
	}
	if !strings.Contains(err.Error(), "initial version can only be created manually once per book") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestBuildApprovedVersionCloneCopiesSourceVersionIntoDraft(t *testing.T) {
	t.Helper()

	reviewerID := 99
	source := BookVersion{
		ID:                        1,
		BookID:                    7,
		VersionNumber:             1,
		SourcePageCount:           10,
		ContentTemplatePageNumber: 9,
		SectionTemplatePageNumber: 3,
		AllowPageImage:            true,
		AllowNewSections:          true,
		LayoutSettings:            json.RawMessage(`{"layout":"default"}`),
		SourcePDFFileName:         "source.pdf",
		SourcePDFFileURL:          "https://storage.example/source.pdf",
		SourcePDFStorageURI:       "gs://bucket/books/book-7/source.pdf",
		SourcePDFObjectKey:        "books/book-7/source.pdf",
		GeneratedPDFFileName:      "generated.pdf",
		GeneratedPDFFileURL:       "https://storage.example/generated.pdf",
		GeneratedPDFStorageURI:    "gs://bucket/books/book-7/generated.pdf",
		GeneratedPDFObjectKey:     "books/book-7/generated.pdf",
	}
	cloned := buildApprovedVersionClone(source, 2, &reviewerID)

	if cloned.ID != 0 {
		t.Fatalf("expected cloned version id to be empty before persistence, got %d", cloned.ID)
	}
	if cloned.BookID != source.BookID {
		t.Fatalf("expected book id %d, got %d", source.BookID, cloned.BookID)
	}
	if cloned.VersionNumber != 2 {
		t.Fatalf("expected new version number 2, got %d", cloned.VersionNumber)
	}
	if !reflect.DeepEqual(cloned.LayoutSettings, source.LayoutSettings) {
		t.Fatalf("expected layout settings to be copied, got %#v", cloned.LayoutSettings)
	}
	if cloned.SourcePDFObjectKey != source.SourcePDFObjectKey {
		t.Fatalf("expected source pdf object key %q, got %q", source.SourcePDFObjectKey, cloned.SourcePDFObjectKey)
	}
	if cloned.GeneratedPDFFileName != "" || cloned.GeneratedPDFFileURL != "" || cloned.GeneratedPDFObjectKey != "" {
		t.Fatalf("expected generated pdf metadata to be reset, got %#v", cloned)
	}
	if cloned.LastGeneratedAt != nil {
		t.Fatal("expected cloned version to clear last generated timestamp")
	}
	if cloned.CreatedBy == nil || *cloned.CreatedBy != reviewerID {
		t.Fatalf("expected created_by %d, got %#v", reviewerID, cloned.CreatedBy)
	}
	if cloned.UpdatedBy == nil || *cloned.UpdatedBy != reviewerID {
		t.Fatalf("expected updated_by %d, got %#v", reviewerID, cloned.UpdatedBy)
	}
}

func TestBuildApprovedSubmissionRecordMovesSubmissionToDraftVersion(t *testing.T) {
	t.Helper()

	reviewerID := 42
	reviewedAt := time.Date(2026, 6, 17, 9, 30, 0, 0, time.UTC)
	submission := BookSubmission{
		ID:              30,
		BookID:          7,
		BookVersionID:   1,
		TargetSectionID: intPtr(11),
		Status:          BookSubmissionStatusPending,
		RejectionReason: "Needs revision",
	}
	targetSectionID := intPtr(21)

	approved := buildApprovedSubmissionRecord(submission, 2, targetSectionID, &reviewerID, reviewedAt)

	if approved.BookVersionID != 2 {
		t.Fatalf("expected submission to move to version 2, got %d", approved.BookVersionID)
	}
	if approved.TargetSectionID == nil || *approved.TargetSectionID != 21 {
		t.Fatalf("expected target section id 21, got %#v", approved.TargetSectionID)
	}
	if approved.Status != BookSubmissionStatusApproved {
		t.Fatalf("expected approved status, got %q", approved.Status)
	}
	if approved.ReviewedBy == nil || *approved.ReviewedBy != reviewerID {
		t.Fatalf("expected reviewed_by %d, got %#v", reviewerID, approved.ReviewedBy)
	}
	if approved.ReviewedAt == nil || !approved.ReviewedAt.Equal(reviewedAt) {
		t.Fatalf("expected reviewed_at %s, got %#v", reviewedAt, approved.ReviewedAt)
	}
	if approved.RejectionReason != "" {
		t.Fatalf("expected rejection reason to be cleared, got %q", approved.RejectionReason)
	}
	if submission.BookVersionID != 1 {
		t.Fatalf("expected source submission to remain unchanged, got version %d", submission.BookVersionID)
	}
}

func TestNullableStringPointerTrimsBlankToNil(t *testing.T) {
	t.Helper()

	if value := nullableStringPointer("   "); value != nil {
		t.Fatalf("expected blank string to normalize to nil, got %#v", value)
	}

	value := nullableStringPointer(" Main Recipes ")
	if value == nil || *value != "Main Recipes" {
		t.Fatalf("expected trimmed string pointer, got %#v", value)
	}
}

func validSaveBookVersionRequest() SaveBookVersionRequest {
	return SaveBookVersionRequest{
		SourcePageCount:           10,
		ContentTemplatePageNumber: 10,
		SectionTemplatePageNumber: 3,
		AllowPageImage:            true,
		AllowNewSections:          true,
		Sections: []SaveBookVersionSectionRequest{
			{
				Name:            "Main Recipes",
				SourceStartPage: intPtr(4),
				SourceEndPage:   intPtr(6),
			},
		},
		Fields: []SaveBookVersionFieldRequest{
			{
				Label:        "Submitter Email",
				InputType:    BookFieldInputTypeSingleLine,
				Placement:    BookFieldPlacementBody,
				IsRequired:   true,
				IsEmailField: true,
			},
			{
				Label:      "Story",
				InputType:  BookFieldInputTypeRichText,
				Placement:  BookFieldPlacementBody,
				ShowLabel:  false,
				IsRequired: true,
			},
		},
		SourcePDF: &BookUploadInput{
			FileName: "cookbook.pdf",
			MimeType: "application/pdf",
			Content:  []byte("%PDF-1.4"),
		},
	}
}

func intPtr(value int) *int {
	return &value
}

func jsonEqual(t *testing.T, left json.RawMessage, right json.RawMessage) bool {
	t.Helper()

	var leftValue any
	if err := json.Unmarshal(left, &leftValue); err != nil {
		t.Fatalf("unmarshal left json: %v", err)
	}

	var rightValue any
	if err := json.Unmarshal(right, &rightValue); err != nil {
		t.Fatalf("unmarshal right json: %v", err)
	}

	return reflect.DeepEqual(leftValue, rightValue)
}
