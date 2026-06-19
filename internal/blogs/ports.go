package blogs

type BlogServicePort interface {
	ListBlogs(filter BlogListFilters) (*BlogListResponse, error)
	GetBlog(id int) (*BlogDetailResponse, error)
	GetBlogCoverImageContent(id int) (*BlogMediaContent, error)
	GetBlogSectionImageContent(id int, sectionID int) (*BlogMediaContent, error)
	GetBlogAnimationItemImageContent(id int, sectionID int, itemID int) (*BlogMediaContent, error)
	CreateBlog(req SaveBlogRequest) (*BlogMutationResponse, error)
	UpdateBlog(id int, req SaveBlogRequest) (*BlogMutationResponse, error)
	DeleteBlog(id int) error
}

var _ BlogServicePort = (*BlogService)(nil)
