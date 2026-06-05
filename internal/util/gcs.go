package util

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
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
	ErrObjectNotFound     = errors.New("gcs object not found")
)

type gcsClient interface {
	NewWriter(ctx context.Context, bucketName, objectName, contentType string) (gcsWriter, error)
	NewReader(ctx context.Context, bucketName, objectName string) (gcsReader, error)
	DeleteObject(ctx context.Context, bucketName, objectName string) error
	Close() error
}

type gcsWriter interface {
	Write(p []byte) (int, error)
	Close() error
}

type gcsReader interface {
	Read(p []byte) (int, error)
	Close() error
	ContentType() string
}

type gcsClientFuncs struct {
	newWriter    func(ctx context.Context, bucketName, objectName, contentType string) (gcsWriter, error)
	newReader    func(ctx context.Context, bucketName, objectName string) (gcsReader, error)
	deleteObject func(ctx context.Context, bucketName, objectName string) error
	close        func() error
}

type gcsWriterFuncs struct {
	write func(p []byte) (int, error)
	close func() error
}

type gcsReaderFuncs struct {
	read        func(p []byte) (int, error)
	close       func() error
	contentType func() string
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
		newReader: func(ctx context.Context, bucketName, objectName string) (gcsReader, error) {
			reader, err := client.Bucket(bucketName).Object(objectName).NewReader(ctx)
			if err != nil {
				return nil, err
			}
			return gcsReaderFuncs{
				read:  reader.Read,
				close: reader.Close,
				contentType: func() string {
					return reader.Attrs.ContentType
				},
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

func (c gcsClientFuncs) NewReader(ctx context.Context, bucketName, objectName string) (gcsReader, error) {
	return c.newReader(ctx, bucketName, objectName)
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

func (r gcsReaderFuncs) Read(p []byte) (int, error) {
	return r.read(p)
}

func (r gcsReaderFuncs) Close() error {
	return r.close()
}

func (r gcsReaderFuncs) ContentType() string {
	if r.contentType == nil {
		return ""
	}
	return r.contentType()
}

func UploadBase64ToGCS(base64Data, bucketName, objectName, contentType string) (string, int64, error) {
	data, err := decodeBase64Payload(base64Data)
	if err != nil {
		return "", 0, err
	}
	return uploadToGCS(data, bucketName, objectName, contentType)
}

func UploadBytesToGCS(data []byte, bucketName, objectName, contentType string) (string, int64, error) {
	return uploadToGCS(data, bucketName, objectName, contentType)
}

func uploadToGCS(data []byte, bucketName, objectName, contentType string) (string, int64, error) {
	if strings.TrimSpace(bucketName) == "" {
		return "", 0, ErrBucketNameRequired
	}
	if strings.TrimSpace(objectName) == "" {
		return "", 0, ErrObjectNameRequired
	}

	prepared := PrepareUploadForStorage(data, contentType)
	data = prepared.Data
	contentType = prepared.ContentType

	ctx := context.Background()
	client, err := newGCSClient(ctx)
	if err != nil {
		return "", 0, err
	}
	defer client.Close()

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

	return GCSObjectURI(bucketName, objectName), int64(sizeBytes), nil
}

func ReadGCSObject(bucketName, objectName string) ([]byte, string, error) {
	if strings.TrimSpace(bucketName) == "" {
		return nil, "", ErrBucketNameRequired
	}
	if strings.TrimSpace(objectName) == "" {
		return nil, "", ErrObjectNameRequired
	}

	ctx := context.Background()
	client, err := newGCSClient(ctx)
	if err != nil {
		return nil, "", err
	}
	defer client.Close()

	reader, err := client.NewReader(ctx, bucketName, objectName)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, "", ErrObjectNotFound
		}
		return nil, "", err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", err
	}

	contentType := strings.TrimSpace(reader.ContentType())
	if contentType == "" && len(data) > 0 {
		contentType = http.DetectContentType(data)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return data, contentType, nil
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
	resolvedBucket, objectName, err := ParseGCSObjectReference(bucketName, rawURL)
	if err != nil {
		return err
	}
	if strings.TrimSpace(resolvedBucket) == "" {
		resolvedBucket = bucketName
	}
	return DeleteGCSObject(resolvedBucket, objectName)
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

func GCSObjectURI(bucket, objectPath string) string {
	return fmt.Sprintf("gs://%s/%s", bucket, objectPath)
}

func ExtractObjectPathFromGCSURL(bucket, raw string) (string, error) {
	_, objectPath, err := ParseGCSObjectReference(bucket, raw)
	return objectPath, err
}

func ParseGCSObjectReference(fallbackBucket, raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", ErrObjectNameRequired
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", "", err
	}
	u.RawQuery = ""
	u.Fragment = ""

	if strings.EqualFold(u.Scheme, "gs") {
		bucketName := strings.TrimSpace(u.Host)
		objectPath := strings.TrimPrefix(path.Clean("/"+strings.TrimPrefix(u.Path, "/")), "/")
		if bucketName == "" {
			bucketName = strings.TrimSpace(fallbackBucket)
		}
		if bucketName == "" {
			return "", "", ErrBucketNameRequired
		}
		if objectPath == "" || objectPath == "." {
			return "", "", ErrObjectNameRequired
		}
		return bucketName, objectPath, nil
	}

	if u.Scheme == "" && u.Host == "" {
		objectPath := strings.TrimPrefix(path.Clean("/"+raw), "/")
		if objectPath == "" || objectPath == "." {
			return "", "", ErrObjectNameRequired
		}
		return strings.TrimSpace(fallbackBucket), objectPath, nil
	}

	host := strings.ToLower(strings.TrimSpace(u.Host))
	objectPath := strings.TrimPrefix(u.Path, "/")

	if strings.EqualFold(host, "storage.googleapis.com") {
		fallbackBucket = strings.TrimSpace(fallbackBucket)
		prefix := fallbackBucket + "/"
		if fallbackBucket != "" {
			if strings.HasPrefix(objectPath, prefix) {
				return fallbackBucket, strings.TrimPrefix(objectPath, prefix), nil
			}
			return fallbackBucket, objectPath, nil
		}
		parts := strings.SplitN(objectPath, "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
		return fallbackBucket, objectPath, nil
	}

	if strings.HasSuffix(strings.ToLower(host), ".storage.googleapis.com") {
		bucketName := strings.TrimSuffix(host, ".storage.googleapis.com")
		return bucketName, objectPath, nil
	}

	objectPath = strings.TrimPrefix(path.Clean("/"+objectPath), "/")
	if objectPath == "" || objectPath == "." {
		return "", "", ErrObjectNameRequired
	}
	return strings.TrimSpace(fallbackBucket), objectPath, nil
}
