package menus

type MenuServicePort interface {
	GetMenu(key string) (*MenuResponse, error)
	ListMenuPageOptions() (*MenuPageOptionsResponse, error)
	SaveMenu(key string, req SaveMenuRequest) (*MenuResponse, error)
}

var _ MenuServicePort = (*MenuService)(nil)
