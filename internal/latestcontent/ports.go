package latestcontent

type ServicePort interface {
	ListLatestContent(limit int) (*LatestContentResponse, error)
}

var _ ServicePort = (*Service)(nil)
