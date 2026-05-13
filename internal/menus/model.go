package menus

import "time"

const (
	NavigationTypePage         = "pages"
	NavigationTypeExternalLink = "external_link"
)

type Menu struct {
	ID        int       `gorm:"primaryKey" json:"id"`
	MenuKey   string    `gorm:"column:menu_key;size:100;not null;unique" json:"menu_key"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	CreatedBy *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	UpdatedBy *int      `gorm:"column:updated_by" json:"updated_by,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Menu) TableName() string {
	return "menus"
}

type MenuItem struct {
	ID             int       `gorm:"primaryKey" json:"id"`
	MenuID         int       `gorm:"column:menu_id;not null" json:"menu_id"`
	ParentID       *int      `gorm:"column:parent_id" json:"parent_id,omitempty"`
	Label          string    `gorm:"size:255;not null" json:"label"`
	NavigationType string    `gorm:"column:navigation_type;size:30;not null" json:"navigation_type"`
	PageID         *int      `gorm:"column:page_id" json:"page_id,omitempty"`
	ExternalURL    string    `gorm:"column:external_url" json:"external_url"`
	OpenInNewTab   bool      `gorm:"column:open_in_new_tab;not null" json:"open_in_new_tab"`
	SortOrder      int       `gorm:"column:sort_order;not null" json:"sort_order"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (MenuItem) TableName() string {
	return "menu_items"
}

type SaveMenuRequest struct {
	Name      string                `json:"name"`
	Items     []SaveMenuItemRequest `json:"items"`
	UpdatedBy *int                  `json:"updated_by,omitempty"`
}

type SaveMenuItemRequest struct {
	Label          string                `json:"label"`
	NavigationType string                `json:"navigation_type"`
	PageID         *int                  `json:"page_id"`
	ExternalURL    string                `json:"external_url"`
	OpenInNewTab   bool                  `json:"open_in_new_tab"`
	Children       []SaveMenuItemRequest `json:"children"`
}

type MenuPageReference struct {
	ID        int    `json:"id"`
	PageTitle string `json:"page_title"`
	URLSlug   string `json:"url_slug"`
	ParentID  *int   `json:"parent_id"`
	PageType  string `json:"page_type"`
	Status    string `json:"status"`
}

type MenuItemResponse struct {
	ID             int                `json:"id"`
	ParentID       *int               `json:"parent_id"`
	Label          string             `json:"label"`
	NavigationType string             `json:"navigation_type"`
	PageID         *int               `json:"page_id"`
	ExternalURL    string             `json:"external_url"`
	OpenInNewTab   bool               `json:"open_in_new_tab"`
	SortOrder      int                `json:"sort_order"`
	Href           string             `json:"href"`
	PageType       string             `json:"page_type,omitempty"`
	Page           *MenuPageReference `json:"page,omitempty"`
	Children       []MenuItemResponse `json:"children"`
}

type MenuResponse struct {
	ID      int                `json:"id"`
	MenuKey string             `json:"menu_key"`
	Name    string             `json:"name"`
	Items   []MenuItemResponse `json:"items"`
}

type MenuPageOption struct {
	ID              int    `json:"id"`
	PageTitle       string `json:"page_title"`
	URLSlug         string `json:"url_slug"`
	ParentID        *int   `json:"parent_id"`
	ParentPageTitle string `json:"parent_page_title"`
	PageType        string `json:"page_type"`
	Status          string `json:"status"`
}

type MenuPageOptionsResponse struct {
	Items []MenuPageOption `json:"items"`
}
