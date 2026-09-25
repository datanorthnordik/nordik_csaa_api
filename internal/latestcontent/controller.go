package latestcontent

import (
	"net/http"
	"strconv"
	"strings"

	"nordikcsaaapi/internal/apiresponse"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	Service ServicePort
}

func (controller *Controller) ListLatestContent(c *gin.Context) {
	if controller.Service == nil {
		apiresponse.WriteInternalError(c)
		return
	}

	limit := 3
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 1 || parsed > MaxItems {
			c.JSON(http.StatusBadRequest, gin.H{"message": "limit must be between 1 and 10"})
			return
		}
		limit = parsed
	}

	response, err := controller.Service.ListLatestContent(limit)
	if err != nil {
		apiresponse.WriteInternalError(c)
		return
	}

	c.JSON(http.StatusOK, response)
}
