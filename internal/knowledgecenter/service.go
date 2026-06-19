package knowledgecenter

import (
	"errors"
	"fmt"
	"log"
	"net/mail"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	knowledgeCenterNotificationEmail = "athul.narayanan@algomau.ca"
	knowledgeCenterCMSReviewURL      = "https://nordikcsaacms-724838782318.us-west1.run.app/knowledge-center"
)

var (
	ErrStoreUnavailable                  = errors.New("knowledge center service unavailable")
	ErrKnowledgeCenterSubmissionNotFound = errors.New("knowledge center submission not found")

	knowledgeCenterNowFunc = time.Now
)

type KnowledgeCenterEmailSender interface {
	SendEmail(to []string, subject string, body string) error
}

type KnowledgeCenterService struct {
	DB          *gorm.DB
	EmailSender KnowledgeCenterEmailSender
}

type knowledgeCenterReviewerAccount struct {
	ID        int    `gorm:"primaryKey;autoIncrement"`
	FirstName string `gorm:"column:firstname"`
	LastName  string `gorm:"column:lastname"`
	Email     string `gorm:"column:email"`
}

func (knowledgeCenterReviewerAccount) TableName() string {
	return "users"
}

func (s *KnowledgeCenterService) ListSubmissions(filter ListKnowledgeCenterSubmissionsFilter) (*KnowledgeCenterSubmissionsListResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	normalized, err := normalizeListKnowledgeCenterSubmissionsFilter(filter)
	if err != nil {
		return nil, err
	}

	summary, err := s.countByStatus(s.submissionsBaseQuery(normalized.SearchTerm))
	if err != nil {
		return nil, err
	}

	itemQuery := s.submissionsBaseQuery(normalized.SearchTerm).
		Where("status = ?", normalized.Status)

	var totalItems int64
	if err := itemQuery.Count(&totalItems).Error; err != nil {
		return nil, err
	}

	var rows []KnowledgeCenterSubmission
	orderedQuery := itemQuery
	if normalized.Status == KnowledgeCenterSubmissionStatusCompleted {
		orderedQuery = orderedQuery.
			Order(clause.OrderByColumn{Column: clause.Column{Name: "completed_at"}, Desc: true}).
			Order(clause.OrderByColumn{Column: clause.Column{Name: "id"}, Desc: true})
	} else {
		orderedQuery = orderedQuery.
			Order(clause.OrderByColumn{Column: clause.Column{Name: "created_at"}, Desc: true}).
			Order(clause.OrderByColumn{Column: clause.Column{Name: "id"}, Desc: true})
	}

	if err := orderedQuery.
		Offset((normalized.Page - 1) * normalized.PageSize).
		Limit(normalized.PageSize).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]KnowledgeCenterSubmissionResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, knowledgeCenterSubmissionResponseFromModel(row))
	}

	totalPages := 0
	if totalItems > 0 {
		totalPages = int((totalItems + int64(normalized.PageSize) - 1) / int64(normalized.PageSize))
	}

	return &KnowledgeCenterSubmissionsListResponse{
		Items: items,
		Pagination: KnowledgeCenterSubmissionsPageMeta{
			Page:       normalized.Page,
			PageSize:   normalized.PageSize,
			TotalItems: totalItems,
			TotalPages: totalPages,
			HasNext:    normalized.Page < totalPages,
			HasPrev:    normalized.Page > 1 && totalPages > 0,
		},
		Summary: summary,
		Applied: KnowledgeCenterSubmissionsAppliedFilters{
			Page:       normalized.Page,
			PageSize:   normalized.PageSize,
			SearchTerm: normalized.SearchTerm,
			Status:     normalized.Status,
		},
	}, nil
}

func (s *KnowledgeCenterService) GetSubmission(id int) (*KnowledgeCenterSubmissionResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	submission, err := s.getSubmissionModel(id)
	if err != nil {
		return nil, err
	}

	resp := knowledgeCenterSubmissionResponseFromModel(*submission)
	return &resp, nil
}

func (s *KnowledgeCenterService) CreatePublicSubmission(req CreateKnowledgeCenterSubmissionRequest) (*KnowledgeCenterSubmissionResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	cleanReq, err := normalizeCreateKnowledgeCenterSubmissionRequest(req)
	if err != nil {
		return nil, err
	}

	submission := KnowledgeCenterSubmission{
		SubmitterName:  cleanReq.SubmitterName,
		SubmitterEmail: cleanReq.SubmitterEmail,
		SubmitterPhone: cleanReq.SubmitterPhone,
		SubmissionType: cleanReq.SubmissionType,
		Message:        cleanReq.Message,
		Status:         KnowledgeCenterSubmissionStatusOpen,
	}

	if err := s.DB.Create(&submission).Error; err != nil {
		return nil, err
	}

	go s.sendNewSubmissionEmailBestEffort(submission)

	resp := knowledgeCenterSubmissionResponseFromModel(submission)
	return &resp, nil
}

func (s *KnowledgeCenterService) MarkSubmissionCompleted(id int, req CompleteKnowledgeCenterSubmissionRequest, userID *int) (*KnowledgeCenterSubmissionResponse, error) {
	if s.DB == nil {
		return nil, ErrStoreUnavailable
	}

	cleanReq, err := normalizeCompleteKnowledgeCenterSubmissionRequest(req)
	if err != nil {
		return nil, err
	}
	if userID == nil || *userID <= 0 {
		return nil, fmt.Errorf("authenticated reviewer is required")
	}

	reviewer, err := s.getReviewerAccount(*userID)
	if err != nil {
		return nil, err
	}

	var updated KnowledgeCenterSubmission
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		var submission KnowledgeCenterSubmission
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&submission, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrKnowledgeCenterSubmissionNotFound
			}
			return err
		}

		if submission.Status == KnowledgeCenterSubmissionStatusCompleted {
			return fmt.Errorf("submission is already completed")
		}

		now := knowledgeCenterNowFunc()
		submission.Status = KnowledgeCenterSubmissionStatusCompleted
		submission.CompletionNotes = cleanReq.CompletionNotes
		submission.CompletedByUserID = &reviewer.ID
		submission.CompletedByName = reviewerDisplayName(reviewer)
		submission.CompletedByEmail = strings.TrimSpace(strings.ToLower(reviewer.Email))
		submission.CompletedAt = &now

		if err := tx.Save(&submission).Error; err != nil {
			return err
		}

		updated = submission
		return nil
	})
	if err != nil {
		return nil, err
	}

	resp := knowledgeCenterSubmissionResponseFromModel(updated)
	return &resp, nil
}

func normalizeListKnowledgeCenterSubmissionsFilter(filter ListKnowledgeCenterSubmissionsFilter) (ListKnowledgeCenterSubmissionsFilter, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 10
	}

	filter.SearchTerm = strings.TrimSpace(filter.SearchTerm)
	filter.Status = strings.ToLower(strings.TrimSpace(filter.Status))
	if filter.Status == "" {
		filter.Status = KnowledgeCenterSubmissionStatusOpen
	}
	if !isAllowedKnowledgeCenterSubmissionStatus(filter.Status) {
		return filter, fmt.Errorf("status must be one of open, completed")
	}

	return filter, nil
}

func normalizeCreateKnowledgeCenterSubmissionRequest(req CreateKnowledgeCenterSubmissionRequest) (CreateKnowledgeCenterSubmissionRequest, error) {
	req.SubmitterName = strings.TrimSpace(req.SubmitterName)
	req.SubmitterEmail = strings.TrimSpace(strings.ToLower(req.SubmitterEmail))
	req.SubmitterPhone = strings.TrimSpace(req.SubmitterPhone)
	req.SubmissionType = strings.ToLower(strings.TrimSpace(req.SubmissionType))
	req.Message = strings.TrimSpace(req.Message)

	if req.SubmitterName == "" {
		return req, fmt.Errorf("name is required")
	}
	if len(req.SubmitterName) > 255 {
		return req, fmt.Errorf("name must be 255 characters or fewer")
	}
	if req.SubmitterEmail == "" {
		return req, fmt.Errorf("email is required")
	}
	if _, err := mail.ParseAddress(req.SubmitterEmail); err != nil {
		return req, fmt.Errorf("email must be a valid email address")
	}
	if len(req.SubmitterPhone) > 80 {
		return req, fmt.Errorf("phone must be 80 characters or fewer")
	}
	if !isAllowedKnowledgeCenterSubmissionType(req.SubmissionType) {
		return req, fmt.Errorf("type must be one of post, video, both")
	}
	if req.Message == "" {
		return req, fmt.Errorf("message is required")
	}
	if len(req.Message) > 5000 {
		return req, fmt.Errorf("message must be 5000 characters or fewer")
	}

	return req, nil
}

func normalizeCompleteKnowledgeCenterSubmissionRequest(req CompleteKnowledgeCenterSubmissionRequest) (CompleteKnowledgeCenterSubmissionRequest, error) {
	req.CompletionNotes = strings.TrimSpace(req.CompletionNotes)

	if req.CompletionNotes == "" {
		return req, fmt.Errorf("completion_notes is required")
	}
	if len(req.CompletionNotes) > 5000 {
		return req, fmt.Errorf("completion_notes must be 5000 characters or fewer")
	}

	return req, nil
}

func (s *KnowledgeCenterService) applySearchFilter(query *gorm.DB, searchTerm string) *gorm.DB {
	searchTerm = strings.ToLower(strings.TrimSpace(searchTerm))
	if searchTerm == "" {
		return query
	}

	pattern := "%" + searchTerm + "%"
	return query.Where(
		`LOWER(COALESCE(submitter_name, '')) LIKE ?
		OR LOWER(COALESCE(submitter_email, '')) LIKE ?
		OR LOWER(COALESCE(submitter_phone, '')) LIKE ?
		OR LOWER(COALESCE(submission_type, '')) LIKE ?
		OR LOWER(COALESCE(message, '')) LIKE ?
		OR LOWER(COALESCE(completion_notes, '')) LIKE ?
		OR LOWER(COALESCE(completed_by_name, '')) LIKE ?
		OR LOWER(COALESCE(completed_by_email, '')) LIKE ?`,
		pattern,
		pattern,
		pattern,
		pattern,
		pattern,
		pattern,
		pattern,
		pattern,
	)
}

func (s *KnowledgeCenterService) submissionsBaseQuery(searchTerm string) *gorm.DB {
	return s.applySearchFilter(s.DB.Model(&KnowledgeCenterSubmission{}), searchTerm)
}

func (s *KnowledgeCenterService) countByStatus(query *gorm.DB) (KnowledgeCenterSubmissionsSummary, error) {
	type countRow struct {
		Status string
		Count  int64
	}

	var rows []countRow
	if err := query.
		Select("status, COUNT(*) AS count").
		Group("status").
		Scan(&rows).Error; err != nil {
		return KnowledgeCenterSubmissionsSummary{}, err
	}

	summary := KnowledgeCenterSubmissionsSummary{}
	for _, row := range rows {
		switch strings.ToLower(strings.TrimSpace(row.Status)) {
		case KnowledgeCenterSubmissionStatusOpen:
			summary.OpenCount = row.Count
		case KnowledgeCenterSubmissionStatusCompleted:
			summary.CompletedCount = row.Count
		}
	}

	return summary, nil
}

func (s *KnowledgeCenterService) getSubmissionModel(id int) (*KnowledgeCenterSubmission, error) {
	var submission KnowledgeCenterSubmission
	if err := s.DB.First(&submission, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrKnowledgeCenterSubmissionNotFound
		}
		return nil, err
	}

	return &submission, nil
}

func (s *KnowledgeCenterService) getReviewerAccount(id int) (*knowledgeCenterReviewerAccount, error) {
	var reviewer knowledgeCenterReviewerAccount
	if err := s.DB.First(&reviewer, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("reviewer account not found")
		}
		return nil, err
	}

	return &reviewer, nil
}

func knowledgeCenterSubmissionResponseFromModel(model KnowledgeCenterSubmission) KnowledgeCenterSubmissionResponse {
	resp := KnowledgeCenterSubmissionResponse{
		ID:              model.ID,
		SubmitterName:   strings.TrimSpace(model.SubmitterName),
		SubmitterEmail:  strings.TrimSpace(model.SubmitterEmail),
		SubmitterPhone:  strings.TrimSpace(model.SubmitterPhone),
		SubmissionType:  strings.TrimSpace(model.SubmissionType),
		Message:         strings.TrimSpace(model.Message),
		Status:          strings.TrimSpace(model.Status),
		CompletionNotes: strings.TrimSpace(model.CompletionNotes),
		CompletedAt:     cloneTimePointer(model.CompletedAt),
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
	}

	if model.CompletedByUserID != nil && *model.CompletedByUserID > 0 {
		resp.CompletedBy = &KnowledgeCenterCompletedByResponse{
			ID:    *model.CompletedByUserID,
			Name:  strings.TrimSpace(model.CompletedByName),
			Email: strings.TrimSpace(model.CompletedByEmail),
		}
	}

	return resp
}

func (s *KnowledgeCenterService) sendNewSubmissionEmailBestEffort(submission KnowledgeCenterSubmission) {
	if s.EmailSender == nil {
		return
	}

	recipients := normalizeKnowledgeCenterEmailList([]string{knowledgeCenterNotificationEmail})
	if len(recipients) == 0 {
		return
	}

	subject := fmt.Sprintf("Urgent: New Living History Hub submission from %s", strings.TrimSpace(submission.SubmitterName))
	body := fmt.Sprintf(
		"Hello Team,\n\nAn urgent new Living History Hub submission has been received and is ready for review.\n\nPlease log in to the CMS, open the Knowledge Center section, review this submission, and mark it as completed once action has been taken.\n\nCMS review link:\n%s\n\nSubmitted on:\n%s\n\nContributor details:\nName: %s\nEmail: %s\nPhone: %s\nContent type: %s\n\n%s\n\nPlease treat this submission as urgent and follow up with the contributor shortly for any additional details needed.\n",
		knowledgeCenterCMSReviewURL,
		formatKnowledgeCenterNotificationTime(submission.CreatedAt),
		strings.TrimSpace(submission.SubmitterName),
		strings.TrimSpace(submission.SubmitterEmail),
		chooseNonEmpty(strings.TrimSpace(submission.SubmitterPhone), "Not provided"),
		knowledgeCenterSubmissionTypeLabel(submission.SubmissionType),
		strings.TrimSpace(submission.Message),
	)

	if err := s.EmailSender.SendEmail(recipients, subject, body); err != nil {
		log.Printf("knowledge center email send failed: recipients=%v subject=%q err=%v", recipients, subject, err)
	}
}

func formatKnowledgeCenterNotificationTime(value time.Time) string {
	location, err := time.LoadLocation("America/Toronto")
	if err == nil {
		value = value.In(location)
	}

	return value.Format("Monday, January 2, 2006 at 3:04 PM MST")
}

func normalizeKnowledgeCenterEmailList(emails []string) []string {
	seen := make(map[string]struct{}, len(emails))
	resp := make([]string, 0, len(emails))
	for _, email := range emails {
		email = strings.TrimSpace(strings.ToLower(email))
		if email == "" {
			continue
		}
		if _, err := mail.ParseAddress(email); err != nil {
			continue
		}
		if _, exists := seen[email]; exists {
			continue
		}
		seen[email] = struct{}{}
		resp = append(resp, email)
	}
	return resp
}

func knowledgeCenterSubmissionTypeLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case KnowledgeCenterSubmissionTypePost:
		return "A written post / story"
	case KnowledgeCenterSubmissionTypeVideo:
		return "A video"
	case KnowledgeCenterSubmissionTypeBoth:
		return "Both a post and a video"
	default:
		return "Unknown"
	}
}

func isAllowedKnowledgeCenterSubmissionType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case KnowledgeCenterSubmissionTypePost,
		KnowledgeCenterSubmissionTypeVideo,
		KnowledgeCenterSubmissionTypeBoth:
		return true
	default:
		return false
	}
}

func isAllowedKnowledgeCenterSubmissionStatus(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case KnowledgeCenterSubmissionStatusOpen,
		KnowledgeCenterSubmissionStatusCompleted:
		return true
	default:
		return false
	}
}

func reviewerDisplayName(reviewer *knowledgeCenterReviewerAccount) string {
	if reviewer == nil {
		return ""
	}

	return strings.TrimSpace(strings.TrimSpace(reviewer.FirstName) + " " + strings.TrimSpace(reviewer.LastName))
}

func chooseNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
