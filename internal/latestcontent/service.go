package latestcontent

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var ErrStoreUnavailable = errors.New("latest content store unavailable")

type Service struct {
	DB *gorm.DB
}

func (s *Service) ListLatestContent(limit int) (*LatestContentResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	limit = normalizeLimit(limit)
	rows := make([]LatestContent, 0, limit)
	if err := s.DB.
		Order("published_at DESC").
		Order("id DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]LatestContentItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, LatestContentItem{
			ID:          row.ID,
			SourceType:  row.SourceType,
			SourceID:    row.SourceID,
			Title:       row.Title,
			Description: row.Description,
			DisplayDate: row.DisplayDate,
			PublishedAt: row.PublishedAt,
			DetailPath:  detailPath(row.SourceType, row.SourceID),
		})
	}

	return &LatestContentResponse{Items: items}, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 3
	}
	if limit > MaxItems {
		return MaxItems
	}
	return limit
}

func detailPath(sourceType string, sourceID int) string {
	switch sourceType {
	case SourceTypeEvent:
		return fmt.Sprintf("/events/%d", sourceID)
	case SourceTypeNewsletter:
		return fmt.Sprintf("/news-media/digital-newsletter/%d", sourceID)
	case SourceTypePress:
		return fmt.Sprintf("/news-media/press-archive/%d", sourceID)
	default:
		return ""
	}
}
