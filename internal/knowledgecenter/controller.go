package knowledgecenter

import (
	"net/http"
	"strconv"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

type KnowledgeCenterController struct {
	KnowledgeCenterService KnowledgeCenterServicePort
}

func (kc *KnowledgeCenterController) ListSubmissions(c *gin.Context) {
	if kc.KnowledgeCenterService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	filter := ListKnowledgeCenterSubmissionsFilter{
		Page:       knowledgeCenterQueryInt(c, "page", 1, 1, 0),
		PageSize:   knowledgeCenterQueryInt(c, "page_size", 10, 1, 100),
		SearchTerm: strings.TrimSpace(c.Query("search")),
		Status:     strings.TrimSpace(strings.ToLower(c.DefaultQuery("status", KnowledgeCenterSubmissionStatusOpen))),
	}

	resp, err := kc.KnowledgeCenterService.ListSubmissions(filter)
	if err != nil {
		writeKnowledgeCenterError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (kc *KnowledgeCenterController) GetSubmission(c *gin.Context) {
	if kc.KnowledgeCenterService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	submissionID, ok := knowledgeCenterPathInt(c, "submissionId")
	if !ok {
		return
	}

	resp, err := kc.KnowledgeCenterService.GetSubmission(submissionID)
	if err != nil {
		writeKnowledgeCenterError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"submission": resp})
}

func (kc *KnowledgeCenterController) CreatePublicSubmission(c *gin.Context) {
	if kc.KnowledgeCenterService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	var req CreateKnowledgeCenterSubmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteValidationError(c, "Request body must be valid JSON")
		return
	}

	resp, err := kc.KnowledgeCenterService.CreatePublicSubmission(req)
	if err != nil {
		writeKnowledgeCenterError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Knowledge center submission created successfully",
		"submission": resp,
	})
}

func (kc *KnowledgeCenterController) MarkSubmissionCompleted(c *gin.Context) {
	if kc.KnowledgeCenterService == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	submissionID, ok := knowledgeCenterPathInt(c, "submissionId")
	if !ok {
		return
	}

	var req CompleteKnowledgeCenterSubmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteValidationError(c, "Request body must be valid JSON")
		return
	}

	resp, err := kc.KnowledgeCenterService.MarkSubmissionCompleted(
		submissionID,
		req,
		knowledgeCenterAuthUserID(c),
	)
	if err != nil {
		writeKnowledgeCenterError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Knowledge center submission completed successfully",
		"submission": resp,
	})
}

func knowledgeCenterPathInt(c *gin.Context, param string) (int, bool) {
	value := strings.TrimSpace(c.Param(param))
	if value == "" {
		apiresponse.WritePathParamError(c, param)
		return 0, false
	}

	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		apiresponse.WritePathParamError(c, param)
		return 0, false
	}

	return id, true
}

func knowledgeCenterQueryInt(c *gin.Context, key string, fallback int, min int, max int) int {
	value, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery(key, strconv.Itoa(fallback))))
	if err != nil || value < min || (max > 0 && value > max) {
		return fallback
	}
	return value
}

func knowledgeCenterAuthUserID(c *gin.Context) *int {
	if c == nil {
		return nil
	}

	for _, key := range []string{"auth_user_id", "userID", "user_id", "userId"} {
		value, exists := c.Get(key)
		if !exists {
			continue
		}
		switch v := value.(type) {
		case int:
			return &v
		case int32:
			userID := int(v)
			return &userID
		case int64:
			userID := int(v)
			return &userID
		case uint:
			userID := int(v)
			return &userID
		case float64:
			userID := int(v)
			return &userID
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				return &parsed
			}
		}
	}

	return nil
}

func writeKnowledgeCenterError(c *gin.Context, err error) {
	httpapi.HandleError(c, "knowledgecenter", err,
		httpapi.ServiceUnavailableRule("Knowledge center service is temporarily unavailable", ErrStoreUnavailable),
		httpapi.NotFoundRule(ErrKnowledgeCenterSubmissionNotFound),
		httpapi.ConflictRule("Unable to save knowledge center submission because a conflicting record already exists"),
		httpapi.ValidationRule(isClientSafeKnowledgeCenterError),
	)
}

func isClientSafeKnowledgeCenterError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))

	switch {
	case strings.Contains(message, " is required"),
		strings.Contains(message, " must be "),
		strings.Contains(message, "valid email address"),
		strings.Contains(message, "authenticated reviewer"),
		strings.Contains(message, "reviewer account not found"),
		strings.Contains(message, "already completed"):
		return true
	default:
		return false
	}
}
