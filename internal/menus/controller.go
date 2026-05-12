package menus

import (
	"net/http"
	"strconv"
	"strings"

	"nordikcsaaapi/internal/apiresponse"
	"nordikcsaaapi/internal/httpapi"

	"github.com/gin-gonic/gin"
)

type MenuController struct {
	MenuService MenuServicePort
}

func (mc *MenuController) GetMenu(c *gin.Context) {
	resp, err := mc.MenuService.GetMenu(c.Param("key"))
	if err != nil {
		writeMenuError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (mc *MenuController) ListMenuPageOptions(c *gin.Context) {
	resp, err := mc.MenuService.ListMenuPageOptions()
	if err != nil {
		writeMenuError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (mc *MenuController) SaveMenu(c *gin.Context) {
	var req SaveMenuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.WriteValidationError(c, "Invalid request payload")
		return
	}

	if authUserID := authUserIDFromContext(c); authUserID != nil {
		req.UpdatedBy = authUserID
	}

	resp, err := mc.MenuService.SaveMenu(c.Param("key"), req)
	if err != nil {
		writeMenuError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Menu saved successfully",
		"menu":    resp,
	})
}

func writeMenuError(c *gin.Context, err error) {
	httpapi.HandleError(c, "menus", err,
		httpapi.ServiceUnavailableRule("Menu service is temporarily unavailable", ErrStoreUnavailable),
		httpapi.NotFoundRule(ErrMenuNotFound),
		httpapi.ConflictRule("Unable to save menu because a conflicting record already exists"),
		httpapi.ValidationRule(isClientSafeMenuError),
	)
}

func isClientSafeMenuError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, " is required"),
		strings.Contains(message, "invalid "),
		strings.Contains(message, "must be "),
		strings.Contains(message, "menu item"),
		strings.Contains(message, "page_id"),
		strings.Contains(message, "navigation_type"),
		strings.Contains(message, "external_url"),
		strings.Contains(message, "parent"),
		strings.Contains(message, "already added"),
		strings.Contains(message, "unsupported menu key"):
		return true
	default:
		return false
	}
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

func pathInt(c *gin.Context, key string) (int, bool) {
	value, err := strconv.Atoi(c.Param(key))
	if err != nil {
		apiresponse.WritePathParamError(c, key)
		return 0, false
	}
	return value, true
}
