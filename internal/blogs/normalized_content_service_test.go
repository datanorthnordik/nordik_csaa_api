package blogs

import "testing"

func TestNormalizeSaveBlogDetailUsesArrayOrder(t *testing.T) {
	detail, err := normalizeSaveBlogDetailRequest(&SaveBlogDetailRequest{
		Sections: []SaveBlogSectionRequest{
			{
				SectionType: BlogSectionTypeVideo,
				SortOrder:   99,
				IsEnabled:   true,
				Video:       &BlogVideoSectionInput{YouTubeURL: "https://youtu.be/example"},
			},
			{
				SectionType: BlogSectionTypeAnimation,
				SortOrder:   10,
				IsEnabled:   true,
				Animation: &BlogAnimationSectionInput{
					Navigation:    BlogAnimationNavigationHorizontal,
					ImagePosition: BlogAnimationImagePositionRight,
					Items: []BlogAnimationItemInput{
						{
							SortOrder:   50,
							Heading:     "First",
							SubHeading:  "One",
							Description: "First item",
							Image:       &BlogUploadInput{GCPObjectKey: "blogs/1/first.jpg"},
						},
						{
							SortOrder:   5,
							Heading:     "Second",
							SubHeading:  "Two",
							Description: "Second item",
							Image:       &BlogUploadInput{GCPObjectKey: "blogs/1/second.jpg"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("normalize detail: %v", err)
	}
	if detail.Sections[0].SortOrder != 0 || detail.Sections[1].SortOrder != 1 {
		t.Fatalf("section order was not normalized: %#v", detail.Sections)
	}
	items := detail.Sections[1].Animation.Items
	if items[0].SortOrder != 0 || items[1].SortOrder != 1 {
		t.Fatalf("animation item order was not normalized: %#v", items)
	}
}

func TestNormalizedBlogModelsUseSeparateTables(t *testing.T) {
	tables := map[string]string{
		"detail":     (BlogContentDetail{}).TableName(),
		"sections":   (BlogSection{}).TableName(),
		"heading":    (BlogSectionHeadingModule{}).TableName(),
		"image":      (BlogSectionImageModule{}).TableName(),
		"typography": (BlogSectionTypographyModule{}).TableName(),
		"action":     (BlogSectionActionModule{}).TableName(),
		"video":      (BlogSectionVideoModule{}).TableName(),
		"animation":  (BlogSectionAnimationModule{}).TableName(),
		"items":      (BlogAnimationItem{}).TableName(),
	}
	for name, table := range tables {
		if table == "" {
			t.Fatalf("%s model has no table name", name)
		}
	}
}
