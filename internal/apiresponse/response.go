package apiresponse

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type ErrorDetail struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type ErrorBody struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

func WriteError(c *gin.Context, status int, code string, message string, details ...ErrorDetail) {
	body := ErrorBody{
		Code:    code,
		Message: message,
	}
	if len(details) > 0 {
		body.Details = details
	}
	c.AbortWithStatusJSON(status, ErrorResponse{Error: body})
}

func WriteValidationError(c *gin.Context, message string, details ...ErrorDetail) {
	WriteError(c, http.StatusBadRequest, "validation_error", message, details...)
}

func WriteUnauthorized(c *gin.Context, code string, message string) {
	WriteError(c, http.StatusUnauthorized, code, message)
}

func WriteNotFound(c *gin.Context, message string) {
	WriteError(c, http.StatusNotFound, "not_found", message)
}

func WriteConflict(c *gin.Context, message string) {
	WriteError(c, http.StatusConflict, "conflict", message)
}

func WriteServiceUnavailable(c *gin.Context, message string) {
	WriteError(c, http.StatusServiceUnavailable, "service_unavailable", message)
}

func WriteInternalError(c *gin.Context) {
	WriteError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
}

func WriteMethodNotAllowed(c *gin.Context) {
	WriteError(c, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
}

func WriteRouteNotFound(c *gin.Context) {
	WriteNotFound(c, "Endpoint not found")
}

func WritePathParamError(c *gin.Context, field string) {
	WriteValidationError(c, "Invalid path parameter", ErrorDetail{
		Field:   field,
		Message: fmt.Sprintf("%s must be a valid integer", field),
	})
}

func WriteBindingError(c *gin.Context, err error, sample any) {
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		details := make([]ErrorDetail, 0, len(validationErrs))
		for _, fieldErr := range validationErrs {
			field := jsonFieldName(sample, fieldErr.StructField())
			details = append(details, ErrorDetail{
				Field:   field,
				Message: validationMessage(field, fieldErr),
			})
		}
		WriteValidationError(c, "Request validation failed", details...)
		return
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		WriteError(c, http.StatusBadRequest, "invalid_json", "Request body contains invalid JSON")
		return
	}

	var unmarshalTypeErr *json.UnmarshalTypeError
	if errors.As(err, &unmarshalTypeErr) {
		message := "Request body contains an invalid value"
		if strings.TrimSpace(unmarshalTypeErr.Field) != "" {
			message = fmt.Sprintf("%s must be a valid %s", unmarshalTypeErr.Field, unmarshalTypeErr.Type.String())
		}
		WriteError(c, http.StatusBadRequest, "invalid_json", message)
		return
	}

	switch {
	case errors.Is(err, io.EOF):
		WriteError(c, http.StatusBadRequest, "invalid_json", "Request body must not be empty")
	case strings.Contains(strings.ToLower(err.Error()), "unexpected eof"):
		WriteError(c, http.StatusBadRequest, "invalid_json", "Request body contains invalid JSON")
	default:
		WriteError(c, http.StatusBadRequest, "bad_request", "Invalid request body")
	}
}

func jsonFieldName(sample any, structField string) string {
	t := reflect.TypeOf(sample)
	if t == nil {
		return structField
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return structField
	}

	field, ok := t.FieldByName(structField)
	if !ok {
		return structField
	}

	tag := strings.TrimSpace(field.Tag.Get("json"))
	if tag == "" {
		return structField
	}

	name := strings.Split(tag, ",")[0]
	if name == "" || name == "-" {
		return structField
	}

	return name
}

func validationMessage(field string, fieldErr validator.FieldError) string {
	switch fieldErr.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters long", field, fieldErr.Param())
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}
