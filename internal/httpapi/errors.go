package httpapi

import (
	"errors"
	"log"
	"strings"

	"nordikcsaaapi/internal/apiresponse"

	"github.com/gin-gonic/gin"
)

type ErrorRule struct {
	Match  func(error) bool
	Handle func(*gin.Context, error)
}

func HandleError(c *gin.Context, scope string, err error, rules ...ErrorRule) {
	LogRequestError(c, scope, err)

	for _, rule := range rules {
		if rule.Match == nil || rule.Handle == nil {
			continue
		}
		if rule.Match(err) {
			rule.Handle(c, err)
			return
		}
	}

	apiresponse.WriteInternalError(c)
}

func LogRequestError(c *gin.Context, scope string, err error) {
	if err == nil {
		return
	}

	method := ""
	path := ""
	if c != nil && c.Request != nil {
		method = c.Request.Method
		path = c.Request.URL.Path
	}

	log.Printf("%s error: method=%s path=%s err=%v", scope, method, path, err)
}

func MatchAny(targets ...error) func(error) bool {
	return func(err error) bool {
		for _, target := range targets {
			if errors.Is(err, target) {
				return true
			}
		}
		return false
	}
}

func MatchConflict(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "violates unique constraint") ||
		strings.Contains(message, "sqlstate 23505")
}

func ServiceUnavailableRule(message string, targets ...error) ErrorRule {
	return ErrorRule{
		Match: MatchAny(targets...),
		Handle: func(c *gin.Context, _ error) {
			apiresponse.WriteServiceUnavailable(c, message)
		},
	}
}

func NotFoundRule(targets ...error) ErrorRule {
	return ErrorRule{
		Match: MatchAny(targets...),
		Handle: func(c *gin.Context, err error) {
			apiresponse.WriteNotFound(c, err.Error())
		},
	}
}

func ConflictRule(message string) ErrorRule {
	return ErrorRule{
		Match: MatchConflict,
		Handle: func(c *gin.Context, _ error) {
			apiresponse.WriteConflict(c, message)
		},
	}
}

func ValidationRule(match func(error) bool) ErrorRule {
	return ErrorRule{
		Match: match,
		Handle: func(c *gin.Context, err error) {
			apiresponse.WriteValidationError(c, err.Error())
		},
	}
}
