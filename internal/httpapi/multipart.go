package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const multipartMemoryLimit = 32 << 20

type UploadedFile struct {
	Filename    string
	ContentType string
	Data        []byte
}

func IsMultipartForm(c *gin.Context) bool {
	if c == nil {
		return false
	}

	contentType := strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type")))
	return strings.HasPrefix(contentType, "multipart/form-data")
}

func MultipartPayload(c *gin.Context, field string) (string, error) {
	if err := parseMultipartForm(c); err != nil {
		return "", err
	}

	value := strings.TrimSpace(c.PostForm(field))
	if value == "" {
		return "", errors.New(field + " is required")
	}

	return value, nil
}

func ReadMultipartFile(c *gin.Context, field string) (*UploadedFile, error) {
	if err := parseMultipartForm(c); err != nil {
		return nil, err
	}

	header, err := c.FormFile(field)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, nil
		}
		return nil, err
	}

	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if contentType == "" || strings.EqualFold(contentType, "application/octet-stream") {
		contentType = http.DetectContentType(data)
	}

	return &UploadedFile{
		Filename:    strings.TrimSpace(header.Filename),
		ContentType: contentType,
		Data:        data,
	}, nil
}

func parseMultipartForm(c *gin.Context) error {
	if c == nil || c.Request == nil {
		return errors.New("request is required")
	}
	return c.Request.ParseMultipartForm(multipartMemoryLimit)
}
