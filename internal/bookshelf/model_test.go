package bookshelf

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestBookshelfEntrySchemaUsesExpectedColumnNames(t *testing.T) {
	t.Parallel()

	parsedSchema, err := schema.Parse(&BookshelfEntry{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	tests := map[string]string{
		"BookFileSize":        "book_file_size",
		"AuthorImageFileSize": "author_image_file_size",
		"CoverImageFileSize":  "cover_image_file_size",
	}

	for fieldName, wantColumn := range tests {
		field := parsedSchema.LookUpField(fieldName)
		if field == nil {
			t.Fatalf("field %s not found in parsed schema", fieldName)
		}
		if field.DBName != wantColumn {
			t.Fatalf("field %s mapped to %q, want %q", fieldName, field.DBName, wantColumn)
		}
	}
}
