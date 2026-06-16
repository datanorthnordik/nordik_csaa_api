package books

import (
	"reflect"
	"strings"
	"testing"
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
