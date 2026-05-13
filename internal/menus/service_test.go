package menus

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	return db, mock, func() { _ = sqlDB.Close() }
}

func TestMenuServiceStoreUnavailable(t *testing.T) {
	svc := &MenuService{}

	if _, err := svc.GetMenu("main"); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ErrStoreUnavailable from GetMenu, got %v", err)
	}
	if _, err := svc.ListMenuPageOptions(); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ErrStoreUnavailable from ListMenuPageOptions, got %v", err)
	}
	if _, err := svc.SaveMenu("main", SaveMenuRequest{}); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected ErrStoreUnavailable from SaveMenu, got %v", err)
	}
}

func TestGetMenuReturnsEmptyMenuWhenMissing(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &MenuService{DB: db}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "menus" WHERE menu_key = $1 LIMIT $2`)).
		WithArgs("main", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	resp, err := svc.GetMenu("main")
	if err != nil {
		t.Fatalf("GetMenu returned error: %v", err)
	}
	if resp.MenuKey != "main" || resp.Name != "Main Website Navigation" {
		t.Fatalf("unexpected empty menu response: %#v", resp)
	}
	if len(resp.Items) != 0 {
		t.Fatalf("expected no items, got %#v", resp.Items)
	}
}

func TestGetMenuReturnsHydratedTreeWhenMenuExists(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &MenuService{DB: db}
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "menus" WHERE menu_key = $1 LIMIT $2`)).
		WithArgs("main", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "menu_key", "name", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(5, "main", "Main Website Navigation", 1, 2, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "menu_items" WHERE menu_id = $1 ORDER BY sort_order ASC,id ASC`)).
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "menu_id", "parent_id", "label", "navigation_type", "page_id", "external_url", "open_in_new_tab", "sort_order", "created_at", "updated_at",
		}).
			AddRow(10, 5, nil, "About", NavigationTypePage, 100, "", false, 0, now, now).
			AddRow(11, 5, 10, "Team", NavigationTypePage, 101, "", false, 0, now, now).
			AddRow(12, 5, 10, "Partner Site", NavigationTypeExternalLink, nil, "https://example.com", true, 1, now, now))

	mock.ExpectQuery(`SELECT .* FROM "pages" LEFT JOIN pages AS parent_pages ON parent_pages.id = pages.parent_id WHERE pages.id IN \(\$1,\$2\) ORDER BY pages.url_slug ASC,pages.id ASC`).
		WithArgs(100, 101).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "parent_id", "parent_page_title", "page_type", "status",
		}).
			AddRow(100, "About Us", "/about", nil, "", "page", "published").
			AddRow(101, "Team", "/about/team", 100, "About Us", "module", "published"))

	resp, err := svc.GetMenu("main")
	if err != nil {
		t.Fatalf("GetMenu returned error: %v", err)
	}
	if len(resp.Items) != 1 || len(resp.Items[0].Children) != 2 {
		t.Fatalf("expected hydrated menu tree, got %#v", resp)
	}
	if resp.Items[0].Href != "/about" || resp.Items[0].Children[0].Href != "/about/team" {
		t.Fatalf("expected page hrefs in response, got %#v", resp.Items)
	}
	if resp.Items[0].PageType != "page" || resp.Items[0].Page == nil || resp.Items[0].Page.PageType != "page" {
		t.Fatalf("expected root page item type metadata, got %#v", resp.Items[0])
	}
	if resp.Items[0].Children[0].PageType != "module" || resp.Items[0].Children[0].Page == nil || resp.Items[0].Children[0].Page.PageType != "module" {
		t.Fatalf("expected child page item type metadata, got %#v", resp.Items[0].Children[0])
	}
	if resp.Items[0].Children[1].Href != "https://example.com" || !resp.Items[0].Children[1].OpenInNewTab {
		t.Fatalf("expected external link metadata in response, got %#v", resp.Items[0].Children[1])
	}
	if resp.Items[0].Children[1].PageType != "" {
		t.Fatalf("expected external item page_type to be empty, got %#v", resp.Items[0].Children[1])
	}
}

func TestListMenuPageOptionsReturnsPublishedPages(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &MenuService{DB: db}

	mock.ExpectQuery(`SELECT .* FROM "pages" LEFT JOIN pages AS parent_pages ON parent_pages.id = pages.parent_id WHERE pages.status = \$1 ORDER BY pages.url_slug ASC,pages.id ASC`).
		WithArgs("published").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "parent_id", "parent_page_title", "page_type", "status",
		}).AddRow(10, "About Us", "/about", nil, "", "page", "published").
			AddRow(11, "Team", "/about/team", 10, "About Us", "module", "published"))

	resp, err := svc.ListMenuPageOptions()
	if err != nil {
		t.Fatalf("ListMenuPageOptions returned error: %v", err)
	}
	if len(resp.Items) != 2 || resp.Items[1].ParentID == nil || *resp.Items[1].ParentID != 10 {
		t.Fatalf("unexpected menu page options: %#v", resp)
	}
	if resp.Items[0].PageType != "page" || resp.Items[1].PageType != "module" {
		t.Fatalf("expected page types in menu options, got %#v", resp.Items)
	}
}

func TestSaveMenuCreatesNewMenuWithDefaultName(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &MenuService{DB: db}
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "menus" WHERE menu_key = $1 LIMIT $2`)).
		WithArgs("footer_links", 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "menus"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(8))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "menu_items" WHERE menu_id = $1`)).
		WithArgs(8).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "menu_items" WHERE menu_id = $1 ORDER BY sort_order ASC,id ASC`)).
		WithArgs(8).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "menu_id", "parent_id", "label", "navigation_type", "page_id", "external_url", "open_in_new_tab", "sort_order", "created_at", "updated_at",
		}))

	resp, err := svc.SaveMenu("footer_links", SaveMenuRequest{
		UpdatedBy: intPtr(9),
	})
	if err != nil {
		t.Fatalf("SaveMenu returned error: %v", err)
	}
	if resp.MenuKey != "footer_links" || resp.Name != "Footer Links" {
		t.Fatalf("expected default name for custom menu key, got %#v", resp)
	}
	if len(resp.Items) != 0 {
		t.Fatalf("expected empty items after initial save, got %#v", resp.Items)
	}
	_ = now
}

func TestSaveMenuUpdatesExistingMenuAndPersistsHierarchy(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &MenuService{DB: db}
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	rootPageID := 100
	childPageID := 101

	mock.ExpectQuery(`SELECT .* FROM "pages" LEFT JOIN pages AS parent_pages ON parent_pages.id = pages.parent_id WHERE pages.id IN \(\$1,\$2\) ORDER BY pages.url_slug ASC,pages.id ASC`).
		WithArgs(rootPageID, childPageID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "parent_id", "parent_page_title", "page_type", "status",
		}).
			AddRow(rootPageID, "About Us", "/about", nil, "", "page", "published").
			AddRow(childPageID, "Team", "/about/team", rootPageID, "About Us", "module", "published"))

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "menus" WHERE menu_key = $1 LIMIT $2`)).
		WithArgs("main", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "menu_key", "name", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(5, "main", "Old Name", 1, 2, now, now))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "menus" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "menu_items" WHERE menu_id = $1`)).
		WithArgs(5).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "menu_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "menu_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "menu_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(12))
	mock.ExpectCommit()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "menu_items" WHERE menu_id = $1 ORDER BY sort_order ASC,id ASC`)).
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "menu_id", "parent_id", "label", "navigation_type", "page_id", "external_url", "open_in_new_tab", "sort_order", "created_at", "updated_at",
		}).
			AddRow(10, 5, nil, "About", NavigationTypePage, rootPageID, "", false, 0, now, now).
			AddRow(11, 5, 10, "Team", NavigationTypePage, childPageID, "", false, 0, now, now).
			AddRow(12, 5, 10, "Partner Site", NavigationTypeExternalLink, nil, "https://example.com", true, 1, now, now))

	mock.ExpectQuery(`SELECT .* FROM "pages" LEFT JOIN pages AS parent_pages ON parent_pages.id = pages.parent_id WHERE pages.id IN \(\$1,\$2\) ORDER BY pages.url_slug ASC,pages.id ASC`).
		WithArgs(rootPageID, childPageID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "parent_id", "parent_page_title", "page_type", "status",
		}).
			AddRow(rootPageID, "About Us", "/about", nil, "", "page", "published").
			AddRow(childPageID, "Team", "/about/team", rootPageID, "About Us", "module", "published"))

	resp, err := svc.SaveMenu("main", SaveMenuRequest{
		Name: "Main Website Navigation",
		Items: []SaveMenuItemRequest{
			{
				Label:          "About",
				NavigationType: NavigationTypePage,
				PageID:         &rootPageID,
				Children: []SaveMenuItemRequest{
					{
						Label:          "Team",
						NavigationType: NavigationTypePage,
						PageID:         &childPageID,
					},
					{
						Label:          "Partner Site",
						NavigationType: NavigationTypeExternalLink,
						ExternalURL:    "https://example.com",
						OpenInNewTab:   true,
					},
				},
			},
		},
		UpdatedBy: intPtr(7),
	})
	if err != nil {
		t.Fatalf("SaveMenu returned error: %v", err)
	}
	if resp.Name != "Main Website Navigation" || len(resp.Items) != 1 {
		t.Fatalf("unexpected saved menu response: %#v", resp)
	}
	if len(resp.Items[0].Children) != 2 || resp.Items[0].Children[1].Href != "https://example.com" {
		t.Fatalf("expected persisted hierarchy in response, got %#v", resp.Items[0])
	}
	if resp.Items[0].PageType != "page" || resp.Items[0].Children[0].PageType != "module" {
		t.Fatalf("expected page type metadata in saved menu response, got %#v", resp.Items[0])
	}
}

func TestValidateMenuItemsRules(t *testing.T) {
	parentID := 10
	childID := 11
	pageMap := map[int]menuPageRecord{
		10: {ID: 10, PageTitle: "About Us", URLSlug: "/about", Status: "published"},
		11: {ID: 11, PageTitle: "Team", URLSlug: "/about/team", ParentID: &parentID, Status: "published"},
		12: {ID: 12, PageTitle: "Hidden", URLSlug: "/hidden", Status: "draft"},
		13: {ID: 13, PageTitle: "Contact", URLSlug: "/contact", Status: "published"},
	}

	valid := []SaveMenuItemRequest{
		{
			Label:          "About",
			NavigationType: NavigationTypePage,
			PageID:         &parentID,
			Children: []SaveMenuItemRequest{
				{
					Label:          "Team",
					NavigationType: NavigationTypePage,
					PageID:         &childID,
				},
				{
					Label:          "Partner Site",
					NavigationType: NavigationTypeExternalLink,
					ExternalURL:    "https://example.com",
					OpenInNewTab:   true,
				},
			},
		},
	}
	if err := validateMenuItems(valid, pageMap); err != nil {
		t.Fatalf("expected valid menu tree, got %v", err)
	}

	duplicate := []SaveMenuItemRequest{
		{Label: "About", NavigationType: NavigationTypePage, PageID: &parentID},
		{Label: "About Duplicate", NavigationType: NavigationTypePage, PageID: &parentID},
	}
	if err := validateMenuItems(duplicate, pageMap); err == nil || !strings.Contains(err.Error(), "already added") {
		t.Fatalf("expected duplicate page validation error, got %v", err)
	}

	childAtRoot := []SaveMenuItemRequest{
		{Label: "Team", NavigationType: NavigationTypePage, PageID: &childID},
	}
	if err := validateMenuItems(childAtRoot, pageMap); err == nil || !strings.Contains(err.Error(), "requires parent") {
		t.Fatalf("expected parent requirement error, got %v", err)
	}

	draftPage := []SaveMenuItemRequest{
		{Label: "Hidden", NavigationType: NavigationTypePage, PageID: intPtr(12)},
	}
	if err := validateMenuItems(draftPage, pageMap); err == nil || !strings.Contains(err.Error(), "published page") {
		t.Fatalf("expected published page validation error, got %v", err)
	}

	externalLinkChildTree := []SaveMenuItemRequest{
		{
			Label:          "External Root",
			NavigationType: NavigationTypeExternalLink,
			ExternalURL:    "https://example.com",
			Children: []SaveMenuItemRequest{
				{Label: "Nested", NavigationType: NavigationTypeExternalLink, ExternalURL: "https://example.org"},
			},
		},
	}
	if err := validateMenuItems(externalLinkChildTree, pageMap); err == nil || !strings.Contains(err.Error(), "cannot contain child") {
		t.Fatalf("expected external link child validation error, got %v", err)
	}

	missingLabel := []SaveMenuItemRequest{
		{NavigationType: NavigationTypePage, PageID: &parentID},
	}
	if err := validateMenuItems(missingLabel, pageMap); err == nil || !strings.Contains(err.Error(), "label is required") {
		t.Fatalf("expected missing label validation error, got %v", err)
	}

	missingPageID := []SaveMenuItemRequest{
		{Label: "No Page", NavigationType: NavigationTypePage},
	}
	if err := validateMenuItems(missingPageID, pageMap); err == nil || !strings.Contains(err.Error(), "page_id is required") {
		t.Fatalf("expected missing page_id validation error, got %v", err)
	}

	missingPageReference := []SaveMenuItemRequest{
		{Label: "Unknown", NavigationType: NavigationTypePage, PageID: intPtr(999)},
	}
	if err := validateMenuItems(missingPageReference, pageMap); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected missing page reference validation error, got %v", err)
	}

	childUnderWrongParent := []SaveMenuItemRequest{
		{
			Label:          "Contact",
			NavigationType: NavigationTypePage,
			PageID:         intPtr(13),
			Children: []SaveMenuItemRequest{
				{Label: "Team", NavigationType: NavigationTypePage, PageID: &childID},
			},
		},
	}
	if err := validateMenuItems(childUnderWrongParent, pageMap); err == nil || !strings.Contains(err.Error(), "must stay under parent") {
		t.Fatalf("expected wrong parent validation error, got %v", err)
	}

	externalWithPageID := []SaveMenuItemRequest{
		{Label: "Bad External", NavigationType: NavigationTypeExternalLink, PageID: &parentID, ExternalURL: "https://example.com"},
	}
	if err := validateMenuItems(externalWithPageID, pageMap); err == nil || !strings.Contains(err.Error(), "page_id must be omitted") {
		t.Fatalf("expected external page_id validation error, got %v", err)
	}

	externalWithoutURL := []SaveMenuItemRequest{
		{Label: "Bad External", NavigationType: NavigationTypeExternalLink},
	}
	if err := validateMenuItems(externalWithoutURL, pageMap); err == nil || !strings.Contains(err.Error(), "external_url is required") {
		t.Fatalf("expected missing external_url validation error, got %v", err)
	}

	externalUnderExternalParent := []SaveMenuItemRequest{
		{
			Label:          "External Root",
			NavigationType: NavigationTypePage,
			PageID:         &parentID,
			Children: []SaveMenuItemRequest{
				{Label: "Partner Site", NavigationType: NavigationTypeExternalLink, ExternalURL: "https://example.com"},
				{Label: "Bad Nested", NavigationType: NavigationTypeExternalLink, ExternalURL: "mailto:test@example.com"},
			},
		},
	}
	if err := validateMenuItems(externalUnderExternalParent, pageMap); err == nil || !strings.Contains(err.Error(), "absolute http or https URL") {
		t.Fatalf("expected invalid external_url validation error, got %v", err)
	}

	invalidType := []SaveMenuItemRequest{
		{Label: "Bad", NavigationType: "custom"},
	}
	if err := validateMenuItems(invalidType, pageMap); err == nil || !strings.Contains(err.Error(), "invalid navigation_type") {
		t.Fatalf("expected invalid navigation_type validation error, got %v", err)
	}
}

func TestBuildMenuTreeIncludesChildrenAndHref(t *testing.T) {
	rootID := 1
	pageID := 10
	childPageID := 11
	flat := []MenuItem{
		{ID: rootID, Label: "About", NavigationType: NavigationTypePage, PageID: &pageID, SortOrder: 0},
		{ID: 2, ParentID: &rootID, Label: "Team", NavigationType: NavigationTypePage, PageID: &childPageID, SortOrder: 0},
		{ID: 3, ParentID: &rootID, Label: "External", NavigationType: NavigationTypeExternalLink, ExternalURL: "https://example.com", OpenInNewTab: true, SortOrder: 1},
	}
	pageMap := map[int]MenuPageReference{
		10: {ID: 10, PageTitle: "About Us", URLSlug: "/about", PageType: "page", Status: "published"},
		11: {ID: 11, PageTitle: "Team", URLSlug: "/about/team", ParentID: &pageID, PageType: "module", Status: "published"},
	}

	tree := buildMenuTree(flat, pageMap)
	if len(tree) != 1 || len(tree[0].Children) != 2 {
		t.Fatalf("unexpected menu tree: %#v", tree)
	}
	if tree[0].Href != "/about" || tree[0].Children[0].Href != "/about/team" {
		t.Fatalf("expected page hrefs to use url slugs, got %#v", tree)
	}
	if tree[0].PageType != "page" || tree[0].Children[0].PageType != "module" {
		t.Fatalf("expected page type metadata in built tree, got %#v", tree)
	}
	if tree[0].Children[1].Href != "https://example.com" || !tree[0].Children[1].OpenInNewTab {
		t.Fatalf("expected external link metadata, got %#v", tree[0].Children[1])
	}
}

func TestMenuHelpersAndDirectMethods(t *testing.T) {
	if normalize, err := normalizeMenuKey(" Main_Menu "); err != nil || normalize != "main_menu" {
		t.Fatalf("expected normalized key main_menu, got %q err=%v", normalize, err)
	}
	if _, err := normalizeMenuKey("bad key"); err == nil {
		t.Fatal("expected invalid menu key error")
	}
	if _, err := normalizeMenuKey(""); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("expected required menu key error, got %v", err)
	}

	if got := defaultMenuName("main"); got != "Main Website Navigation" {
		t.Fatalf("unexpected default main menu name: %q", got)
	}
	if got := defaultMenuName("footer_links"); got != "Footer Links" {
		t.Fatalf("unexpected default custom menu name: %q", got)
	}
	if got := defaultMenuName("__"); got != "Website Navigation" {
		t.Fatalf("unexpected fallback menu name: %q", got)
	}

	req, err := normalizeSaveMenuRequest(SaveMenuRequest{
		Name: "  Primary  ",
		Items: []SaveMenuItemRequest{
			{
				Label:          "  About  ",
				NavigationType: " PAGES ",
				PageID:         intPtr(10),
				ExternalURL:    " https://ignore.me ",
				OpenInNewTab:   true,
				Children: []SaveMenuItemRequest{
					{
						Label:          " Partner ",
						NavigationType: " external_link ",
						ExternalURL:    " https://example.com ",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("normalizeSaveMenuRequest returned error: %v", err)
	}
	if req.Name != "Primary" || req.Items[0].NavigationType != NavigationTypePage || req.Items[0].ExternalURL != "" || req.Items[0].OpenInNewTab {
		t.Fatalf("expected normalized page item, got %#v", req)
	}
	if req.Items[0].Children[0].NavigationType != NavigationTypeExternalLink || req.Items[0].Children[0].ExternalURL != "https://example.com" {
		t.Fatalf("expected normalized external child item, got %#v", req.Items[0].Children[0])
	}

	collected := collectReferencedPageIDs([]SaveMenuItemRequest{
		{PageID: intPtr(12)},
		{PageID: intPtr(10), Children: []SaveMenuItemRequest{{PageID: intPtr(12)}, {PageID: intPtr(11)}}},
	})
	if fmt.Sprint(collected) != "[10 11 12]" {
		t.Fatalf("expected sorted unique page ids, got %v", collected)
	}

	if err := validateExternalURL("https://example.com/path"); err != nil {
		t.Fatalf("expected valid url, got %v", err)
	}
	if err := validateExternalURL("ftp://example.com"); err == nil {
		t.Fatal("expected invalid scheme error")
	}
	if err := validateExternalURL("https:///missing-host"); err == nil {
		t.Fatal("expected missing host error")
	}
	if err := validateExternalURL("http://[::1"); err == nil {
		t.Fatal("expected invalid parse error")
	}

	if got := parentBucketKey(nil); got != "root" {
		t.Fatalf("unexpected root parent bucket key: %q", got)
	}
	if got := parentBucketKey(intPtr(4)); got != "parent:4" {
		t.Fatalf("unexpected parent bucket key: %q", got)
	}
	if got := valueOrZero(nil); got != 0 {
		t.Fatalf("expected zero for nil parent id, got %d", got)
	}
	if got := valueOrZero(intPtr(6)); got != 6 {
		t.Fatalf("expected six from pointer, got %d", got)
	}

	if (Menu{}).TableName() != "menus" || (MenuItem{}).TableName() != "menu_items" {
		t.Fatal("unexpected table names")
	}

	svc := &MenuService{}
	resp, err := svc.loadMenuResponse(Menu{MenuKey: "main", Name: "Main"})
	if err != nil {
		t.Fatalf("loadMenuResponse returned error for empty menu: %v", err)
	}
	if len(resp.Items) != 0 {
		t.Fatalf("expected empty response tree, got %#v", resp.Items)
	}
}

func TestFindOrCreateMenuAndSaveMenuItemsDirectly(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &MenuService{DB: db}
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("db.Begin returned error: %v", tx.Error)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "menus" WHERE menu_key = $1 LIMIT $2`)).
		WithArgs("main", 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "menus"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(20))
	menu, err := svc.findOrCreateMenu(tx, "main", SaveMenuRequest{Name: "Main Website Navigation", UpdatedBy: intPtr(7)})
	if err != nil {
		t.Fatalf("findOrCreateMenu create returned error: %v", err)
	}
	if menu.ID != 20 || menu.Name != "Main Website Navigation" {
		t.Fatalf("unexpected created menu: %#v", menu)
	}
	mock.ExpectCommit()
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("tx.Commit returned error: %v", err)
	}

	mock.ExpectBegin()
	tx = db.Begin()
	if tx.Error != nil {
		t.Fatalf("db.Begin returned error: %v", tx.Error)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "menus" WHERE menu_key = $1 LIMIT $2`)).
		WithArgs("main", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "menu_key", "name", "created_by", "updated_by", "created_at", "updated_at",
		}).AddRow(20, "main", "Old Name", 1, 2, now, now))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "menus" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	menu, err = svc.findOrCreateMenu(tx, "main", SaveMenuRequest{Name: "Updated Name", UpdatedBy: intPtr(8)})
	if err != nil {
		t.Fatalf("findOrCreateMenu update returned error: %v", err)
	}
	if menu.Name != "Updated Name" || menu.UpdatedBy == nil || *menu.UpdatedBy != 8 {
		t.Fatalf("unexpected updated menu: %#v", menu)
	}
	mock.ExpectCommit()
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("tx.Commit returned error: %v", err)
	}

	mock.ExpectBegin()
	tx = db.Begin()
	if tx.Error != nil {
		t.Fatalf("db.Begin returned error: %v", tx.Error)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "menu_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(30))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "menu_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(31))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "menu_items"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(32))
	err = svc.saveMenuItems(tx, 20, nil, []SaveMenuItemRequest{
		{
			Label:          "About",
			NavigationType: NavigationTypePage,
			PageID:         intPtr(100),
			Children: []SaveMenuItemRequest{
				{
					Label:          "Team",
					NavigationType: NavigationTypePage,
					PageID:         intPtr(101),
				},
				{
					Label:          "Partner Site",
					NavigationType: NavigationTypeExternalLink,
					ExternalURL:    "https://example.com",
					OpenInNewTab:   true,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("saveMenuItems returned error: %v", err)
	}
	mock.ExpectCommit()
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("tx.Commit returned error: %v", err)
	}
}

func TestMenuServiceErrorBranches(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &MenuService{DB: db}

	if _, err := svc.SaveMenu("bad key", SaveMenuRequest{}); err == nil {
		t.Fatal("expected invalid menu key error from SaveMenu")
	}

	rootPageID := 100
	mock.ExpectQuery(`SELECT .* FROM "pages" LEFT JOIN pages AS parent_pages ON parent_pages.id = pages.parent_id WHERE pages.id IN \(\$1\) ORDER BY pages.url_slug ASC,pages.id ASC`).
		WithArgs(rootPageID).
		WillReturnError(errors.New("page lookup failed"))
	if _, err := svc.SaveMenu("main", SaveMenuRequest{
		Items: []SaveMenuItemRequest{{
			Label:          "About",
			NavigationType: NavigationTypePage,
			PageID:         &rootPageID,
		}},
	}); err == nil || !strings.Contains(err.Error(), "page lookup failed") {
		t.Fatalf("expected page lookup error from SaveMenu, got %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "menus" WHERE menu_key = $1 LIMIT $2`)).
		WithArgs("main", 1).
		WillReturnError(errors.New("menu lookup failed"))
	if _, err := svc.GetMenu("main"); err == nil || !strings.Contains(err.Error(), "menu lookup failed") {
		t.Fatalf("expected menu lookup error from GetMenu, got %v", err)
	}

	mock.ExpectQuery(`SELECT .* FROM "pages" LEFT JOIN pages AS parent_pages ON parent_pages.id = pages.parent_id WHERE pages.status = \$1 ORDER BY pages.url_slug ASC,pages.id ASC`).
		WithArgs("published").
		WillReturnError(errors.New("options lookup failed"))
	if _, err := svc.ListMenuPageOptions(); err == nil || !strings.Contains(err.Error(), "options lookup failed") {
		t.Fatalf("expected list page options error, got %v", err)
	}

	mock.ExpectBegin()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("db.Begin returned error: %v", tx.Error)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "menus" WHERE menu_key = $1 LIMIT $2`)).
		WithArgs("main", 1).
		WillReturnError(errors.New("db unavailable"))
	if _, err := svc.findOrCreateMenu(tx, "main", SaveMenuRequest{Name: "Main"}); err == nil || !strings.Contains(err.Error(), "db unavailable") {
		t.Fatalf("expected findOrCreateMenu lookup error, got %v", err)
	}
	mock.ExpectRollback()
	if err := tx.Rollback().Error; err != nil {
		t.Fatalf("tx.Rollback returned error: %v", err)
	}

	mock.ExpectBegin()
	tx = db.Begin()
	if tx.Error != nil {
		t.Fatalf("db.Begin returned error: %v", tx.Error)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "menu_items"`)).
		WillReturnError(errors.New("insert failed"))
	if err := svc.saveMenuItems(tx, 1, nil, []SaveMenuItemRequest{{
		Label:          "About",
		NavigationType: NavigationTypePage,
		PageID:         intPtr(100),
	}}); err == nil || !strings.Contains(err.Error(), "insert failed") {
		t.Fatalf("expected saveMenuItems insert error, got %v", err)
	}
	mock.ExpectRollback()
	if err := tx.Rollback().Error; err != nil {
		t.Fatalf("tx.Rollback returned error: %v", err)
	}
}

func TestLoadPageRecordsAndRollbackOnPanic(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	svc := &MenuService{DB: db}

	rows, err := svc.loadPageRecords(nil, false)
	if err != nil {
		t.Fatalf("expected early-return empty rows, got err=%v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected empty rows for no page ids, got %#v", rows)
	}

	mock.ExpectQuery(`SELECT .* FROM "pages" LEFT JOIN pages AS parent_pages ON parent_pages.id = pages.parent_id WHERE pages.id IN \(\$1\) ORDER BY pages.url_slug ASC,pages.id ASC`).
		WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "page_title", "url_slug", "parent_id", "parent_page_title", "page_type", "status",
		}).AddRow(10, "About Us", "/about", nil, "", "page", "published"))

	rows, err = svc.loadPageRecords([]int{10}, false)
	if err != nil || len(rows) != 1 || rows[0].ID != 10 {
		t.Fatalf("expected loaded page row, got rows=%#v err=%v", rows, err)
	}
	if rows[0].PageType != "page" {
		t.Fatalf("expected loaded page type, got %#v", rows[0])
	}

	mock.ExpectBegin()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("db.Begin returned error: %v", tx.Error)
	}
	mock.ExpectRollback()
	defer func() {
		recovered := recover()
		if recovered != "transaction panic" {
			t.Fatalf("expected transaction panic recover value, got %#v", recovered)
		}
	}()
	defer rollbackOnPanic(tx)
	panic("boom")
}

func intPtr(value int) *int {
	return &value
}
