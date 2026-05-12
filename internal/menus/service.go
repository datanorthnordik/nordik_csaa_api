package menus

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	pagespkg "nordikcsaaapi/internal/pages"

	"gorm.io/gorm"
)

var (
	ErrStoreUnavailable = errors.New("menu store unavailable")
	ErrMenuNotFound     = errors.New("menu not found")
)

var menuKeyPattern = regexp.MustCompile(`^[a-z0-9]+(?:[-_][a-z0-9]+)*$`)

type MenuService struct {
	DB *gorm.DB
}

type menuPageRecord struct {
	ID              int
	PageTitle       string
	URLSlug         string
	ParentID        *int
	ParentPageTitle string
	Status          string
}

type menuItemDraft struct {
	Label          string
	NavigationType string
	PageID         *int
	ExternalURL    string
	OpenInNewTab   bool
	Children       []menuItemDraft
}

func (s *MenuService) GetMenu(key string) (*MenuResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	normalizedKey, err := normalizeMenuKey(key)
	if err != nil {
		return nil, err
	}

	var menu Menu
	if err := s.DB.Where("menu_key = ?", normalizedKey).Take(&menu).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &MenuResponse{
				MenuKey: normalizedKey,
				Name:    defaultMenuName(normalizedKey),
				Items:   make([]MenuItemResponse, 0),
			}, nil
		}
		return nil, err
	}

	return s.loadMenuResponse(menu)
}

func (s *MenuService) ListMenuPageOptions() (*MenuPageOptionsResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	rows, err := s.loadPageRecords(nil, true)
	if err != nil {
		return nil, err
	}

	items := make([]MenuPageOption, 0, len(rows))
	for _, row := range rows {
		items = append(items, MenuPageOption{
			ID:              row.ID,
			PageTitle:       row.PageTitle,
			URLSlug:         row.URLSlug,
			ParentID:        row.ParentID,
			ParentPageTitle: row.ParentPageTitle,
			Status:          row.Status,
		})
	}

	return &MenuPageOptionsResponse{Items: items}, nil
}

func (s *MenuService) SaveMenu(key string, req SaveMenuRequest) (*MenuResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	normalizedKey, err := normalizeMenuKey(key)
	if err != nil {
		return nil, err
	}

	normalizedReq, err := normalizeSaveMenuRequest(req)
	if err != nil {
		return nil, err
	}
	if normalizedReq.Name == "" {
		normalizedReq.Name = defaultMenuName(normalizedKey)
	}

	pageIDs := collectReferencedPageIDs(normalizedReq.Items)
	pageRecords, err := s.loadPageRecords(pageIDs, false)
	if err != nil {
		return nil, err
	}

	pageMap := make(map[int]menuPageRecord, len(pageRecords))
	for _, row := range pageRecords {
		pageMap[row.ID] = row
	}

	if err := validateMenuItems(normalizedReq.Items, pageMap); err != nil {
		return nil, err
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer rollbackOnPanic(tx)

	menu, err := s.findOrCreateMenu(tx, normalizedKey, normalizedReq)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Where("menu_id = ?", menu.ID).Delete(&MenuItem{}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := s.saveMenuItems(tx, menu.ID, nil, normalizedReq.Items); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return s.loadMenuResponse(menu)
}

func normalizeMenuKey(key string) (string, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return "", errors.New("menu key is required")
	}
	if !menuKeyPattern.MatchString(key) {
		return "", errors.New("unsupported menu key")
	}
	return key, nil
}

func defaultMenuName(key string) string {
	switch key {
	case "main":
		return "Main Website Navigation"
	default:
		words := strings.FieldsFunc(key, func(r rune) bool {
			return r == '-' || r == '_'
		})
		if len(words) == 0 {
			return "Website Navigation"
		}

		for idx, word := range words {
			if word == "" {
				continue
			}
			words[idx] = strings.ToUpper(word[:1]) + word[1:]
		}
		return strings.Join(words, " ")
	}
}

func normalizeSaveMenuRequest(req SaveMenuRequest) (SaveMenuRequest, error) {
	req.Name = strings.TrimSpace(req.Name)

	items, err := normalizeSaveMenuItems(req.Items)
	if err != nil {
		return req, err
	}
	req.Items = items
	return req, nil
}

func normalizeSaveMenuItems(items []SaveMenuItemRequest) ([]SaveMenuItemRequest, error) {
	if len(items) == 0 {
		return make([]SaveMenuItemRequest, 0), nil
	}

	normalized := make([]SaveMenuItemRequest, 0, len(items))
	for _, item := range items {
		item.Label = strings.TrimSpace(item.Label)
		item.NavigationType = strings.ToLower(strings.TrimSpace(item.NavigationType))
		item.ExternalURL = strings.TrimSpace(item.ExternalURL)
		if item.NavigationType == NavigationTypePage {
			item.ExternalURL = ""
			item.OpenInNewTab = false
		}

		children, err := normalizeSaveMenuItems(item.Children)
		if err != nil {
			return nil, err
		}
		item.Children = children
		normalized = append(normalized, item)
	}

	return normalized, nil
}

func collectReferencedPageIDs(items []SaveMenuItemRequest) []int {
	pageIDs := make([]int, 0)

	var walk func([]SaveMenuItemRequest)
	walk = func(items []SaveMenuItemRequest) {
		for _, item := range items {
			if item.PageID != nil {
				pageIDs = append(pageIDs, *item.PageID)
			}
			walk(item.Children)
		}
	}
	walk(items)

	slices.Sort(pageIDs)
	return slices.Compact(pageIDs)
}

func validateMenuItems(items []SaveMenuItemRequest, pageMap map[int]menuPageRecord) error {
	seenPageIDs := make(map[int]string)

	var walk func([]SaveMenuItemRequest, *SaveMenuItemRequest) error
	walk = func(items []SaveMenuItemRequest, parent *SaveMenuItemRequest) error {
		for _, item := range items {
			if item.Label == "" {
				return errors.New("menu item label is required")
			}

			switch item.NavigationType {
			case NavigationTypePage:
				if item.PageID == nil || *item.PageID <= 0 {
					return errors.New("page_id is required when navigation_type is pages")
				}

				page, ok := pageMap[*item.PageID]
				if !ok {
					return fmt.Errorf("page_id %d references a page that does not exist", *item.PageID)
				}
				if page.Status != pagespkg.PageStatusPublished {
					return fmt.Errorf("page_id %d must reference a published page", *item.PageID)
				}

				if firstLabel, exists := seenPageIDs[*item.PageID]; exists {
					return fmt.Errorf("page_id %d is already added in menu item %q", *item.PageID, firstLabel)
				}
				seenPageIDs[*item.PageID] = item.Label

				if parent == nil {
					if page.ParentID != nil {
						return fmt.Errorf("page_id %d requires parent page_id %d to also be added to the menu", page.ID, *page.ParentID)
					}
				} else {
					if parent.NavigationType != NavigationTypePage || parent.PageID == nil {
						return fmt.Errorf("page_id %d must be placed under its parent page", page.ID)
					}
					if page.ParentID == nil || *page.ParentID != *parent.PageID {
						return fmt.Errorf("page_id %d must stay under parent page_id %d", page.ID, valueOrZero(page.ParentID))
					}
				}
			case NavigationTypeExternalLink:
				if item.PageID != nil {
					return errors.New("page_id must be omitted when navigation_type is external_link")
				}
				if item.ExternalURL == "" {
					return errors.New("external_url is required when navigation_type is external_link")
				}
				if err := validateExternalURL(item.ExternalURL); err != nil {
					return err
				}
				if parent != nil && (parent.NavigationType != NavigationTypePage || parent.PageID == nil) {
					return errors.New("external_link items can only be nested under a page item")
				}
				if len(item.Children) > 0 {
					return errors.New("external_link items cannot contain child menu items")
				}
			default:
				return errors.New("invalid navigation_type")
			}

			if err := walk(item.Children, &item); err != nil {
				return err
			}
		}
		return nil
	}

	return walk(items, nil)
}

func validateExternalURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed == nil {
		return errors.New("external_url must be a valid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("external_url must be an absolute http or https URL")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return errors.New("external_url must be an absolute http or https URL")
	}
	return nil
}

func (s *MenuService) loadPageRecords(pageIDs []int, publishedOnly bool) ([]menuPageRecord, error) {
	if len(pageIDs) == 0 && !publishedOnly {
		return make([]menuPageRecord, 0), nil
	}

	query := s.DB.Model(&pagespkg.Page{}).
		Select(`
			pages.id,
			pages.page_title,
			pages.url_slug,
			pages.parent_id,
			COALESCE(parent_pages.page_title, '') AS parent_page_title,
			pages.status
		`).
		Joins(`LEFT JOIN pages AS parent_pages ON parent_pages.id = pages.parent_id`)

	if len(pageIDs) > 0 {
		query = query.Where("pages.id IN ?", pageIDs)
	}
	if publishedOnly {
		query = query.Where("pages.status = ?", pagespkg.PageStatusPublished)
	}

	rows := make([]menuPageRecord, 0)
	if err := query.
		Order("pages.url_slug ASC").
		Order("pages.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	return rows, nil
}

func (s *MenuService) findOrCreateMenu(tx *gorm.DB, key string, req SaveMenuRequest) (Menu, error) {
	var menu Menu
	if err := tx.Where("menu_key = ?", key).Take(&menu).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return Menu{}, err
		}

		menu = Menu{
			MenuKey:   key,
			Name:      req.Name,
			CreatedBy: req.UpdatedBy,
			UpdatedBy: req.UpdatedBy,
		}
		if err := tx.Create(&menu).Error; err != nil {
			return Menu{}, err
		}
		return menu, nil
	}

	menu.Name = req.Name
	menu.UpdatedBy = req.UpdatedBy
	if err := tx.Save(&menu).Error; err != nil {
		return Menu{}, err
	}

	return menu, nil
}

func (s *MenuService) saveMenuItems(tx *gorm.DB, menuID int, parentID *int, items []SaveMenuItemRequest) error {
	for idx, item := range items {
		row := MenuItem{
			MenuID:         menuID,
			ParentID:       parentID,
			Label:          item.Label,
			NavigationType: item.NavigationType,
			PageID:         item.PageID,
			ExternalURL:    item.ExternalURL,
			OpenInNewTab:   item.OpenInNewTab,
			SortOrder:      idx,
		}

		if err := tx.Create(&row).Error; err != nil {
			return err
		}

		if err := s.saveMenuItems(tx, menuID, &row.ID, item.Children); err != nil {
			return err
		}
	}

	return nil
}

func (s *MenuService) loadMenuResponse(menu Menu) (*MenuResponse, error) {
	items := make([]MenuItem, 0)
	if menu.ID > 0 {
		if err := s.DB.
			Where("menu_id = ?", menu.ID).
			Order("sort_order ASC").
			Order("id ASC").
			Find(&items).Error; err != nil {
			return nil, err
		}
	}

	pageIDs := make([]int, 0)
	for _, item := range items {
		if item.PageID != nil {
			pageIDs = append(pageIDs, *item.PageID)
		}
	}
	slices.Sort(pageIDs)
	pageIDs = slices.Compact(pageIDs)

	pageRecords, err := s.loadPageRecords(pageIDs, false)
	if err != nil {
		return nil, err
	}

	pageMap := make(map[int]MenuPageReference, len(pageRecords))
	for _, row := range pageRecords {
		pageMap[row.ID] = MenuPageReference{
			ID:        row.ID,
			PageTitle: row.PageTitle,
			URLSlug:   row.URLSlug,
			ParentID:  row.ParentID,
			Status:    row.Status,
		}
	}

	responseItems := buildMenuTree(items, pageMap)
	if responseItems == nil {
		responseItems = make([]MenuItemResponse, 0)
	}

	return &MenuResponse{
		ID:      menu.ID,
		MenuKey: menu.MenuKey,
		Name:    menu.Name,
		Items:   responseItems,
	}, nil
}

func buildMenuTree(items []MenuItem, pageMap map[int]MenuPageReference) []MenuItemResponse {
	buckets := make(map[string][]MenuItemResponse)
	for _, item := range items {
		var pageRef *MenuPageReference
		href := strings.TrimSpace(item.ExternalURL)
		if item.PageID != nil {
			if page, ok := pageMap[*item.PageID]; ok {
				copyPage := page
				pageRef = &copyPage
				href = page.URLSlug
			}
		}

		node := MenuItemResponse{
			ID:             item.ID,
			ParentID:       item.ParentID,
			Label:          item.Label,
			NavigationType: item.NavigationType,
			PageID:         item.PageID,
			ExternalURL:    item.ExternalURL,
			OpenInNewTab:   item.OpenInNewTab,
			SortOrder:      item.SortOrder,
			Href:           href,
			Page:           pageRef,
			Children:       make([]MenuItemResponse, 0),
		}
		buckets[parentBucketKey(item.ParentID)] = append(buckets[parentBucketKey(item.ParentID)], node)
	}

	for key := range buckets {
		slices.SortFunc(buckets[key], func(left, right MenuItemResponse) int {
			if left.SortOrder != right.SortOrder {
				return left.SortOrder - right.SortOrder
			}
			return left.ID - right.ID
		})
	}

	var attach func(parentID *int) []MenuItemResponse
	attach = func(parentID *int) []MenuItemResponse {
		nodes := buckets[parentBucketKey(parentID)]
		for idx := range nodes {
			children := attach(&nodes[idx].ID)
			if children == nil {
				nodes[idx].Children = make([]MenuItemResponse, 0)
				continue
			}
			nodes[idx].Children = children
		}
		return nodes
	}

	return attach(nil)
}

func parentBucketKey(parentID *int) string {
	if parentID == nil {
		return "root"
	}
	return fmt.Sprintf("parent:%d", *parentID)
}

func rollbackOnPanic(tx *gorm.DB) {
	if recover() != nil {
		tx.Rollback()
		panic("transaction panic")
	}
}

func valueOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
