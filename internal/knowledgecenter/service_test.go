package knowledgecenter

import (
	"strings"
	"testing"
	"time"
)

type recordingKnowledgeCenterEmailSender struct {
	to      []string
	subject string
	body    string
}

func (r *recordingKnowledgeCenterEmailSender) SendEmail(to []string, subject string, body string) error {
	r.to = append([]string(nil), to...)
	r.subject = subject
	r.body = body
	return nil
}

func TestNormalizeCreateKnowledgeCenterSubmissionRequest(t *testing.T) {
	req, err := normalizeCreateKnowledgeCenterSubmissionRequest(CreateKnowledgeCenterSubmissionRequest{
		SubmitterName:  "  Alice Walker  ",
		SubmitterEmail: " Alice@example.com ",
		SubmitterPhone: " 555-0100 ",
		SubmissionType: "VIDEO",
		Message:        "  I have a clip to contribute. ",
	})
	if err != nil {
		t.Fatalf("normalizeCreateKnowledgeCenterSubmissionRequest returned error: %v", err)
	}

	if req.SubmitterName != "Alice Walker" {
		t.Fatalf("expected trimmed name, got %q", req.SubmitterName)
	}
	if req.SubmitterEmail != "alice@example.com" {
		t.Fatalf("expected normalized email, got %q", req.SubmitterEmail)
	}
	if req.SubmissionType != KnowledgeCenterSubmissionTypeVideo {
		t.Fatalf("expected normalized type, got %q", req.SubmissionType)
	}
	if req.Message != "I have a clip to contribute." {
		t.Fatalf("expected trimmed message, got %q", req.Message)
	}
}

func TestNormalizeCreateKnowledgeCenterSubmissionRequestRejectsInvalidInput(t *testing.T) {
	testCases := []struct {
		name string
		req  CreateKnowledgeCenterSubmissionRequest
	}{
		{
			name: "missing name",
			req: CreateKnowledgeCenterSubmissionRequest{
				SubmitterEmail: "alice@example.com",
				SubmissionType: KnowledgeCenterSubmissionTypePost,
				Message:        "Hello",
			},
		},
		{
			name: "invalid email",
			req: CreateKnowledgeCenterSubmissionRequest{
				SubmitterName:  "Alice",
				SubmitterEmail: "bad-email",
				SubmissionType: KnowledgeCenterSubmissionTypePost,
				Message:        "Hello",
			},
		},
		{
			name: "invalid type",
			req: CreateKnowledgeCenterSubmissionRequest{
				SubmitterName:  "Alice",
				SubmitterEmail: "alice@example.com",
				SubmissionType: "audio",
				Message:        "Hello",
			},
		},
		{
			name: "missing message",
			req: CreateKnowledgeCenterSubmissionRequest{
				SubmitterName:  "Alice",
				SubmitterEmail: "alice@example.com",
				SubmissionType: KnowledgeCenterSubmissionTypePost,
			},
		},
	}

	for _, testCase := range testCases {
		if _, err := normalizeCreateKnowledgeCenterSubmissionRequest(testCase.req); err == nil {
			t.Fatalf("%s: expected validation error", testCase.name)
		}
	}
}

func TestNormalizeCompleteKnowledgeCenterSubmissionRequest(t *testing.T) {
	req, err := normalizeCompleteKnowledgeCenterSubmissionRequest(CompleteKnowledgeCenterSubmissionRequest{
		CompletionNotes: "  Added to the homepage and linked in the archive.  ",
	})
	if err != nil {
		t.Fatalf("normalizeCompleteKnowledgeCenterSubmissionRequest returned error: %v", err)
	}
	if req.CompletionNotes != "Added to the homepage and linked in the archive." {
		t.Fatalf("expected trimmed notes, got %q", req.CompletionNotes)
	}

	if _, err := normalizeCompleteKnowledgeCenterSubmissionRequest(CompleteKnowledgeCenterSubmissionRequest{}); err == nil {
		t.Fatal("expected validation error for empty completion notes")
	}
}

func TestKnowledgeCenterSubmissionResponseFromModelIncludesCompletedBy(t *testing.T) {
	completedByID := 17
	completedAt := time.Date(2026, 6, 18, 14, 30, 0, 0, time.UTC)

	resp := knowledgeCenterSubmissionResponseFromModel(KnowledgeCenterSubmission{
		ID:                9,
		SubmitterName:     "Alice",
		SubmitterEmail:    "alice@example.com",
		SubmitterPhone:    "555-0100",
		SubmissionType:    KnowledgeCenterSubmissionTypeBoth,
		Message:           "Story and video",
		Status:            KnowledgeCenterSubmissionStatusCompleted,
		CompletionNotes:   "Published and tagged.",
		CompletedByUserID: &completedByID,
		CompletedByName:   "Jane Doe",
		CompletedByEmail:  "jane@example.com",
		CompletedAt:       &completedAt,
	})

	if resp.CompletedBy == nil {
		t.Fatal("expected completed_by to be populated")
	}
	if resp.CompletedBy.ID != 17 || resp.CompletedBy.Name != "Jane Doe" || resp.CompletedBy.Email != "jane@example.com" {
		t.Fatalf("unexpected completed_by payload: %#v", resp.CompletedBy)
	}
	if resp.CompletedAt == nil || !resp.CompletedAt.Equal(completedAt) {
		t.Fatalf("expected completed_at %s, got %#v", completedAt, resp.CompletedAt)
	}
}

func TestNormalizeKnowledgeCenterEmailList(t *testing.T) {
	resp := normalizeKnowledgeCenterEmailList([]string{
		"athul.narayanan@algomau.ca",
		" ATHUL.NARAYANAN@ALGOMAU.CA ",
		"not-an-email",
		"",
	})

	if len(resp) != 1 || resp[0] != "athul.narayanan@algomau.ca" {
		t.Fatalf("unexpected normalized email list: %#v", resp)
	}
}

func TestSendNewSubmissionEmailBestEffortBuildsUrgentActionableEmail(t *testing.T) {
	sender := &recordingKnowledgeCenterEmailSender{}
	service := &KnowledgeCenterService{EmailSender: sender}

	service.sendNewSubmissionEmailBestEffort(KnowledgeCenterSubmission{
		ID:             1,
		SubmitterName:  "Alice Walker",
		SubmitterEmail: "alice@example.com",
		SubmitterPhone: "555-0100",
		SubmissionType: KnowledgeCenterSubmissionTypeVideo,
		Message:        "I have a clip to contribute.",
		CreatedAt:      time.Date(2026, 6, 19, 14, 30, 0, 0, time.UTC),
	})

	if len(sender.to) != 1 || sender.to[0] != knowledgeCenterNotificationEmail {
		t.Fatalf("unexpected recipients: %#v", sender.to)
	}
	if sender.subject != "Urgent: New Living History Hub submission from Alice Walker" {
		t.Fatalf("unexpected subject: %q", sender.subject)
	}

	expectedSnippets := []string{
		"Please log in to the CMS, open the Knowledge Center section, review this submission, and mark it as completed once action has been taken.",
		knowledgeCenterCMSReviewURL,
		"Friday, June 19, 2026 at 10:30 AM EDT",
		"Content type: A video",
		"\n\nI have a clip to contribute.\n\n",
		"Please treat this submission as urgent",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(sender.body, snippet) {
			t.Fatalf("expected email body to contain %q, got:\n%s", snippet, sender.body)
		}
	}

	if strings.Contains(sender.body, "Submission ID:") {
		t.Fatalf("email body should not include submission ID, got:\n%s", sender.body)
	}
	if strings.Contains(sender.body, "2026-06-19T14:30:00Z") {
		t.Fatalf("email body should use a readable date format, got:\n%s", sender.body)
	}
}
