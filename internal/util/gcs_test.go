package util

import (
	"context"
	"errors"
	"testing"
)

type fakeGCSClient struct {
	writer           gcsWriter
	newWriterErr     error
	deleteErr        error
	closeErr         error
	gotBucketName    string
	gotObjectName    string
	gotContentType   string
	gotDeleteBucket  string
	gotDeleteObject  string
}

type fakeGCSWriter struct {
	writeErr error
	closeErr error
	gotData  []byte
}

func (c *fakeGCSClient) NewWriter(ctx context.Context, bucketName, objectName, contentType string) (gcsWriter, error) {
	c.gotBucketName = bucketName
	c.gotObjectName = objectName
	c.gotContentType = contentType
	if c.newWriterErr != nil {
		return nil, c.newWriterErr
	}
	return c.writer, nil
}

func (c *fakeGCSClient) DeleteObject(ctx context.Context, bucketName, objectName string) error {
	c.gotDeleteBucket = bucketName
	c.gotDeleteObject = objectName
	return c.deleteErr
}

func (c *fakeGCSClient) Close() error {
	return c.closeErr
}

func (w *fakeGCSWriter) Write(p []byte) (int, error) {
	w.gotData = append([]byte(nil), p...)
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return len(p), nil
}

func (w *fakeGCSWriter) Close() error {
	return w.closeErr
}

func TestUploadBase64ToGCS(t *testing.T) {
	writer := &fakeGCSWriter{}
	client := &fakeGCSClient{writer: writer}
	restore := stubGCSClient(client, nil)
	defer restore()

	url, size, err := UploadBase64ToGCS("aGVsbG8=", "bucket", "path/file.txt", "")
	if err != nil {
		t.Fatalf("UploadBase64ToGCS returned error: %v", err)
	}
	if url != "https://storage.googleapis.com/bucket/path/file.txt" {
		t.Fatalf("unexpected url: %q", url)
	}
	if size != 5 {
		t.Fatalf("expected size 5, got %d", size)
	}
	if string(writer.gotData) != "hello" {
		t.Fatalf("expected decoded payload hello, got %q", string(writer.gotData))
	}
	if client.gotContentType == "" {
		t.Fatal("expected content type to be detected")
	}
}

func TestUploadBase64ToGCSErrors(t *testing.T) {
	if _, _, err := UploadBase64ToGCS("aGVsbG8=", "", "obj", ""); !errors.Is(err, ErrBucketNameRequired) {
		t.Fatalf("expected ErrBucketNameRequired, got %v", err)
	}
	if _, _, err := UploadBase64ToGCS("aGVsbG8=", "bucket", "", ""); !errors.Is(err, ErrObjectNameRequired) {
		t.Fatalf("expected ErrObjectNameRequired, got %v", err)
	}

	restore := stubGCSClient(nil, errors.New("client create failed"))
	defer restore()
	if _, _, err := UploadBase64ToGCS("aGVsbG8=", "bucket", "obj", ""); err == nil {
		t.Fatal("expected client create error")
	}
	restore()

	writer := &fakeGCSWriter{}
	client := &fakeGCSClient{writer: writer, newWriterErr: errors.New("new writer failed")}
	restore = stubGCSClient(client, nil)
	defer restore()
	if _, _, err := UploadBase64ToGCS("aGVsbG8=", "bucket", "obj", "text/plain"); err == nil {
		t.Fatal("expected new writer error")
	}
	restore()

	writer = &fakeGCSWriter{writeErr: errors.New("write failed")}
	client = &fakeGCSClient{writer: writer}
	restore = stubGCSClient(client, nil)
	defer restore()
	if _, _, err := UploadBase64ToGCS("aGVsbG8=", "bucket", "obj", "text/plain"); err == nil {
		t.Fatal("expected write error")
	}
	restore()

	writer = &fakeGCSWriter{closeErr: errors.New("close failed")}
	client = &fakeGCSClient{writer: writer}
	restore = stubGCSClient(client, nil)
	defer restore()
	if _, _, err := UploadBase64ToGCS("aGVsbG8=", "bucket", "obj", "text/plain"); err == nil {
		t.Fatal("expected close error")
	}

	if _, _, err := UploadBase64ToGCS("not-base64", "bucket", "obj", ""); err == nil {
		t.Fatal("expected base64 decode error")
	}
}

func TestDeleteGCSObjectAndByURL(t *testing.T) {
	client := &fakeGCSClient{}
	restore := stubGCSClient(client, nil)
	defer restore()

	if err := DeleteGCSObject("bucket", "folder/file.pdf"); err != nil {
		t.Fatalf("DeleteGCSObject returned error: %v", err)
	}
	if client.gotDeleteBucket != "bucket" || client.gotDeleteObject != "folder/file.pdf" {
		t.Fatalf("unexpected delete target: %s %s", client.gotDeleteBucket, client.gotDeleteObject)
	}

	if err := DeleteGCSObjectByURL("bucket", "https://storage.googleapis.com/bucket/folder/file.pdf?x=1"); err != nil {
		t.Fatalf("DeleteGCSObjectByURL returned error: %v", err)
	}
	if client.gotDeleteObject != "folder/file.pdf" {
		t.Fatalf("expected extracted object path, got %q", client.gotDeleteObject)
	}
}

func TestDeleteGCSObjectErrors(t *testing.T) {
	if err := DeleteGCSObject("", "obj"); !errors.Is(err, ErrBucketNameRequired) {
		t.Fatalf("expected ErrBucketNameRequired, got %v", err)
	}
	if err := DeleteGCSObject("bucket", ""); !errors.Is(err, ErrObjectNameRequired) {
		t.Fatalf("expected ErrObjectNameRequired, got %v", err)
	}

	restore := stubGCSClient(nil, errors.New("client create failed"))
	defer restore()
	if err := DeleteGCSObject("bucket", "obj"); err == nil {
		t.Fatal("expected client create error")
	}
	restore()

	client := &fakeGCSClient{deleteErr: errors.New("delete failed")}
	restore = stubGCSClient(client, nil)
	defer restore()
	if err := DeleteGCSObject("bucket", "obj"); err == nil {
		t.Fatal("expected delete error")
	}

	if err := DeleteGCSObjectByURL("bucket", "://bad-url"); err == nil {
		t.Fatal("expected parse error for invalid URL")
	}
}

func TestHelperFunctions(t *testing.T) {
	if got := SanitizePart(" Hello, World! "); got != "hello_world" {
		t.Fatalf("unexpected sanitized part: %q", got)
	}
	if got := SanitizePart("!!!"); got != "unknown" {
		t.Fatalf("expected fallback unknown, got %q", got)
	}

	if got := ExtFromFilenameOrMime("photo.JPG", ""); got != ".jpg" {
		t.Fatalf("expected .jpg from filename, got %q", got)
	}
	if got := ExtFromFilenameOrMime("", "image/png"); got != ".png" {
		t.Fatalf("expected .png from mime, got %q", got)
	}
	if got := ExtFromFilenameOrMime("", "image/jpeg"); got != ".jpg" {
		t.Fatalf("expected .jpg, got %q", got)
	}
	if got := ExtFromFilenameOrMime("", "image/gif"); got != ".gif" {
		t.Fatalf("expected .gif, got %q", got)
	}
	if got := ExtFromFilenameOrMime("", "image/webp"); got != ".webp" {
		t.Fatalf("expected .webp, got %q", got)
	}
	if got := ExtFromFilenameOrMime("", "application/pdf"); got != ".pdf" {
		t.Fatalf("expected .pdf, got %q", got)
	}
	if got := ExtFromFilenameOrMime("", "application/msword"); got != ".doc" {
		t.Fatalf("expected .doc, got %q", got)
	}
	if got := ExtFromFilenameOrMime("", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"); got != ".docx" {
		t.Fatalf("expected .docx, got %q", got)
	}
	if got := ExtFromFilenameOrMime("", "application/vnd.ms-excel"); got != ".xls" {
		t.Fatalf("expected .xls, got %q", got)
	}
	if got := ExtFromFilenameOrMime("", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"); got != ".xlsx" {
		t.Fatalf("expected .xlsx, got %q", got)
	}
	if got := ExtFromFilenameOrMime("", "text/plain"); got != ".txt" {
		t.Fatalf("expected .txt, got %q", got)
	}
	if got := ExtFromFilenameOrMime("", "application/octet-stream"); got != "" {
		t.Fatalf("expected empty extension, got %q", got)
	}

	if got := PublicGCSURL("bucket", "folder/file.pdf"); got != "https://storage.googleapis.com/bucket/folder/file.pdf" {
		t.Fatalf("unexpected public url: %q", got)
	}

	if data, err := decodeBase64Payload("data:text/plain;base64,aGVsbG8="); err != nil || string(data) != "hello" {
		t.Fatalf("unexpected decoded payload %q err=%v", string(data), err)
	}

	tests := []struct {
		name   string
		bucket string
		rawURL string
		want   string
	}{
		{"storage host with bucket path", "bucket", "https://storage.googleapis.com/bucket/folder/file.pdf?x=1", "folder/file.pdf"},
		{"storage host without bucket prefix", "bucket", "https://storage.googleapis.com/folder/file.pdf", "folder/file.pdf"},
		{"bucket host", "bucket", "https://bucket.storage.googleapis.com/folder/file.pdf", "folder/file.pdf"},
		{"unknown host", "bucket", "https://example.com/folder/file.pdf", "folder/file.pdf"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractObjectPathFromGCSURL(tc.bucket, tc.rawURL)
			if err != nil {
				t.Fatalf("ExtractObjectPathFromGCSURL returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}

	if _, err := ExtractObjectPathFromGCSURL("bucket", "://bad-url"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestFunctionAdapters(t *testing.T) {
	writer := gcsWriterFuncs{
		write: func(p []byte) (int, error) { return len(p), nil },
		close: func() error { return nil },
	}
	if n, err := writer.Write([]byte("abc")); err != nil || n != 3 {
		t.Fatalf("unexpected writer result n=%d err=%v", n, err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("unexpected writer close error: %v", err)
	}

	client := gcsClientFuncs{
		newWriter: func(ctx context.Context, bucketName, objectName, contentType string) (gcsWriter, error) {
			return writer, nil
		},
		deleteObject: func(ctx context.Context, bucketName, objectName string) error { return nil },
		close:        func() error { return nil },
	}
	if _, err := client.NewWriter(context.Background(), "bucket", "obj", "text/plain"); err != nil {
		t.Fatalf("unexpected NewWriter error: %v", err)
	}
	if err := client.DeleteObject(context.Background(), "bucket", "obj"); err != nil {
		t.Fatalf("unexpected DeleteObject error: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("unexpected Close error: %v", err)
	}
}

func stubGCSClient(client gcsClient, err error) func() {
	prev := newGCSClient
	newGCSClient = func(ctx context.Context) (gcsClient, error) {
		if err != nil {
			return nil, err
		}
		return client, nil
	}
	return func() {
		newGCSClient = prev
	}
}
