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
	if normalized.SourcePDF == nil || len(normalized.SourcePDF.Content) == 0 {
		t.Fatal("expected source PDF to be preserved during normalization")
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
