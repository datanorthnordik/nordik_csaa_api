package blogs

import (
	"strconv"
	"strings"

	"nordikcsaaapi/internal/apiresponse"

	"github.com/gin-gonic/gin"
)

func withProtected(handler gin.HandlerFunc, protected ...gin.HandlerFunc) []gin.HandlerFunc {
	handlers := make([]gin.HandlerFunc, 0, len(protected)+1)
	handlers = append(handlers, protected...)
	handlers = append(handlers, handler)
	return handlers
}

func pathInt(c *gin.Context, key string) (int, bool) {
	value, err := strconv.Atoi(c.Param(key))
	if err != nil {
		apiresponse.WritePathParamError(c, key)
		return 0, false
	}
	return value, true
}

func parseQueryInt(value string) int {
	if strings.TrimSpace(value) == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return -1
	}
	return parsed
}

func sanitizeContentDispositionFilename(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	return value
}

func authUserIDFromContext(c *gin.Context) *int {
	value, exists := c.Get("auth_user_id")
	if !exists {
		return nil
	}

	switch typed := value.(type) {
	case int:
		return &typed
	case int32:
		next := int(typed)
		return &next
	case int64:
		next := int(typed)
		return &next
	default:
		return nil
	}
}
