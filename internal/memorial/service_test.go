package memorial

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeSaveMemorialRequest(t *testing.T) {
	req := SaveMemorialRequest{
		FullName:      "  James Montgomery  ",
		Affiliation:   " Class of 1964 ",
		Category:      "ALUMNUS",
		Status:        "PUBLISHED",
		Biography:     "  <p>Remembered with love.</p>  ",
		DateOfBirth:   "1946-03-14",
		DateOfPassing: "2023-09-01",
		Portrait: &MemorialUploadInput{
			FileName: "portrait.jpg",
			MimeType: "image/jpeg",
			Content:  []byte("portrait"),
		},
		GalleryImages: []MemorialUploadInput{
			{
				FileName: "gallery-one.png",
				MimeType: "image/png",
				Content:  []byte("gallery"),
			},
		},
		RemoveGalleryImageIDs: []int{4, 2, 4, 0, -1},
	}

	normalized, err := normalizeSaveMemorialRequest(req)
	if err != nil {
		t.Fatalf("normalizeSaveMemorialRequest returned error: %v", err)
	}

	if normalized.FullName != "James Montgomery" {
		t.Fatalf("unexpected full name: %q", normalized.FullName)
	}
	if normalized.Affiliation != "Class of 1964" {
		t.Fatalf("unexpected affiliation: %q", normalized.Affiliation)
	}
	if normalized.Category != MemorialCategoryAlumnus {
		t.Fatalf("unexpected category: %q", normalized.Category)
	}
	if normalized.Status != MemorialStatusPublished {
		t.Fatalf("unexpected status: %q", normalized.Status)
	}
	if normalized.Biography != "<p>Remembered with love.</p>" {
		t.Fatalf("unexpected biography: %q", normalized.Biography)
	}
	if normalized.DateOfBirth == nil || normalized.DateOfBirth.Format("2006-01-02") != "1946-03-14" {
		t.Fatalf("unexpected date_of_birth: %#v", normalized.DateOfBirth)
	}
	if normalized.DateOfPassing == nil || normalized.DateOfPassing.Format("2006-01-02") != "2023-09-01" {
		t.Fatalf("unexpected date_of_passing: %#v", normalized.DateOfPassing)
	}
	if len(normalized.GalleryImages) != 1 {
		t.Fatalf("expected 1 gallery image, got %d", len(normalized.GalleryImages))
	}
	if got := len(normalized.RemoveGalleryImageIDs); got != 2 {
		t.Fatalf("expected 2 unique gallery ids, got %d", got)
	}
	if normalized.RemoveGalleryImageIDs[0] != 2 || normalized.RemoveGalleryImageIDs[1] != 4 {
		t.Fatalf("unexpected remove ids: %#v", normalized.RemoveGalleryImageIDs)
	}
}

func TestNormalizeSaveMemorialRequestRejectsInvalidDates(t *testing.T) {
	_, err := normalizeSaveMemorialRequest(SaveMemorialRequest{
		FullName:      "James Montgomery",
		Category:      MemorialCategoryAlumnus,
		Status:        MemorialStatusDraft,
		DateOfBirth:   "2025-01-10",
		DateOfPassing: "2025-01-09",
	})
	if err == nil || !strings.Contains(err.Error(), "date_of_passing must be on or after date_of_birth") {
		t.Fatalf("expected invalid date ordering error, got %v", err)
	}
}

func TestStoreMemorialImageBuildsBucketObjectPath(t *testing.T) {
	previousNow := memorialNowFunc
	previousUpload := uploadBytesToGCSHook
	defer func() {
		memorialNowFunc = previousNow
		uploadBytesToGCSHook = previousUpload
	}()

	memorialNowFunc = func() time.Time {
		return time.Date(2026, 5, 25, 10, 30, 0, 0, time.UTC)
	}

	var uploadedObjectKey string
	uploadBytesToGCSHook = func(data []byte, bucketName, objectName, contentType string) (string, int64, error) {
		uploadedObjectKey = objectName
		return "gs://" + bucketName + "/" + objectName, int64(len(data)), nil
	}

	svc := &MemorialService{
		BucketName:   "drive-bucket",
		BucketPrefix: "cms",
	}

	fileURL, objectKey, fileName, mimeType, fileSize, uploadedKey, err := svc.storeMemorialImage(7, "portrait", 0, MemorialUploadInput{
		FileName: "Portrait Final.JPG",
		MimeType: "image/jpeg",
		Content:  []byte("portrait-bytes"),
	})
	if err != nil {
		t.Fatalf("storeMemorialImage returned error: %v", err)
	}

	expectedObjectKey := "cms/memorial/entry-7/portrait/20260525103000_01_portrait_final.jpg"
	if objectKey != expectedObjectKey {
		t.Fatalf("unexpected object key: %q", objectKey)
	}
	if uploadedObjectKey != expectedObjectKey || uploadedKey != expectedObjectKey {
		t.Fatalf("expected uploaded object keys to match %q, got uploaded=%q returned=%q", expectedObjectKey, uploadedObjectKey, uploadedKey)
	}
	if fileURL != "gs://drive-bucket/"+expectedObjectKey {
		t.Fatalf("unexpected file url: %q", fileURL)
	}
	if fileName != "Portrait Final.JPG" {
		t.Fatalf("unexpected file name: %q", fileName)
	}
	if mimeType != "image/jpeg" {
		t.Fatalf("unexpected mime type: %q", mimeType)
	}
	if fileSize != int64(len("portrait-bytes")) {
		t.Fatalf("unexpected file size: %d", fileSize)
	}
}
