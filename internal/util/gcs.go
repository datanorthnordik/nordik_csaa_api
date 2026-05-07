package util

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"cloud.google.com/go/storage"
)

var (
	ErrBucketNameRequired = errors.New("bucket name is required")
	ErrObjectNameRequired = errors.New("object name is required")
)

type gcsClient interface {
	NewWriter(ctx context.Context, bucketName, objectName, contentType string) (gcsWriter, error)
	DeleteObject(ctx context.Context, bucketName, objectName string) error
	Close() error
}

type gcsWriter interface {
	Write(p []byte) (int, error)
	Close() error
}

type gcsClientFuncs struct {
	newWriter    func(ctx context.Context, bucketName, objectName, contentType string) (gcsWriter, error)
	deleteObject func(ctx context.Context, bucketName, objectName string) error
	close        func() error
}

type gcsWriterFuncs struct {
	write func(p []byte) (int, error)
	close func() error
}

var newGCSClient = func(ctx context.Context) (gcsClient, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return gcsClientFuncs{
		newWriter: func(ctx context.Context, bucketName, objectName, contentType string) (gcsWriter, error) {
			writer := client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
			writer.ContentType = contentType
			return gcsWriterFuncs{
				write: writer.Write,
				close: writer.Close,
			}, nil
		},
		deleteObject: func(ctx context.Context, bucketName, objectName string) error {
			return client.Bucket(bucketName).Object(objectName).Delete(ctx)
		},
		close: client.Close,
	}, nil
}

func (c gcsClientFuncs) NewWriter(ctx context.Context, bucketName, objectName, contentType string) (gcsWriter, error) {
	return c.newWriter(ctx, bucketName, objectName, contentType)
}

func (c gcsClientFuncs) DeleteObject(ctx context.Context, bucketName, objectName string) error {
	return c.deleteObject(ctx, bucketName, objectName)
}

func (c gcsClientFuncs) Close() error {
	return c.close()
}

func (w gcsWriterFuncs) Write(p []byte) (int, error) {
	return w.write(p)
}

func (w gcsWriterFuncs) Close() error {
	return w.close()
}

func UploadBase64ToGCS(base64Data, bucketName, objectName, contentType string) (string, int64, error) {
	if strings.TrimSpace(bucketName) == "" {
		return "", 0, ErrBucketNameRequired
	}
	if strings.TrimSpace(objectName) == "" {
		return "", 0, ErrObjectNameRequired
	}

	ctx := context.Background()
	client, err := newGCSClient(ctx)
	if err != nil {
		return "", 0, err
	}
	defer client.Close()

	data, err := decodeBase64Payload(base64Data)
	if err != nil {
		return "", 0, err
	}

	if strings.TrimSpace(contentType) == "" {
		contentType = http.DetectContentType(data)
	}

	writer, err := client.NewWriter(ctx, bucketName, objectName, contentType)
	if err != nil {
		return "", 0, err
	}

	sizeBytes, err := writer.Write(data)
	if err != nil {
		return "", 0, err
	}

	if err := writer.Close(); err != nil {
		return "", 0, err
	}

	return PublicGCSURL(bucketName, objectName), int64(sizeBytes), nil
}

func DeleteGCSObject(bucketName, objectName string) error {
	if strings.TrimSpace(bucketName) == "" {
		return ErrBucketNameRequired
	}
	if strings.TrimSpace(objectName) == "" {
		return ErrObjectNameRequired
	}

	ctx := context.Background()
	client, err := newGCSClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.DeleteObject(ctx, bucketName, objectName)
}

func DeleteGCSObjectByURL(bucketName, rawURL string) error {
	objectName, err := ExtractObjectPathFromGCSURL(bucketName, rawURL)
	if err != nil {
		return err
	}
	return DeleteGCSObject(bucketName, objectName)
}

func decodeBase64Payload(base64Data string) ([]byte, error) {
	payload := strings.TrimSpace(base64Data)
	if idx := strings.Index(payload, ","); idx >= 0 {
		payload = payload[idx+1:]
	}
	return base64.StdEncoding.DecodeString(payload)
}

func SanitizePart(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, " ", "_")
	re := regexp.MustCompile(`[^a-z0-9_\-]`)
	s = re.ReplaceAllString(s, "")
	if s == "" {
		return "unknown"
	}
	return s
}

func ExtFromFilenameOrMime(fileName, mimeType string) string {
	if ext := strings.ToLower(filepath.Ext(strings.TrimSpace(fileName))); ext != "" {
		return ext
	}

	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "application/pdf":
		return ".pdf"
	case "application/msword":
		return ".doc"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/vnd.ms-excel":
		return ".xls"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	case "text/plain":
		return ".txt"
	default:
		return ""
	}
}

func PublicGCSURL(bucket, objectPath string) string {
	return fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucket, objectPath)
}

func ExtractObjectPathFromGCSURL(bucket, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	u.RawQuery = ""
	u.Fragment = ""

	host := u.Host
	p := strings.TrimPrefix(u.Path, "/")

	if strings.EqualFold(host, "storage.googleapis.com") {
		prefix := bucket + "/"
		if strings.HasPrefix(p, prefix) {
			return strings.TrimPrefix(p, prefix), nil
		}
		return p, nil
	}

	if strings.HasSuffix(strings.ToLower(host), ".storage.googleapis.com") {
		return p, nil
	}

	return strings.TrimPrefix(path.Clean("/"+p), "/"), nil
}
