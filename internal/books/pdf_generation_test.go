package books

import (
	"bytes"
	"strings"
	"testing"

	"github.com/phpdave11/gofpdf"
	rpdf "rsc.io/pdf"
)

func TestBuildSubmissionPageContentJoinsHeadingFieldsAndSeparatesBodyBlocks(t *testing.T) {
	t.Helper()

	fields := []BookVersionField{
		{ID: 1, Label: "Title Part 1", Placement: BookFieldPlacementHeading, SortOrder: 0},
		{ID: 2, Label: "Title Part 2", Placement: BookFieldPlacementHeading, SortOrder: 1},
		{ID: 3, Label: "Ingredients", Placement: BookFieldPlacementBody, ShowLabel: true, SortOrder: 2},
		{ID: 4, Label: "Method", Placement: BookFieldPlacementBody, ShowLabel: true, SortOrder: 3},
	}
	values := []BookSubmissionValue{
		{BookFieldID: 1, Value: "Liz's"},
		{BookFieldID: 2, Value: "Bannock"},
		{BookFieldID: 3, Value: "<p>3 cups flour</p><p>1 cup water</p>"},
		{BookFieldID: 4, Value: "<p>Mix dry ingredients.</p><p>Bake until golden.</p>"},
	}
	fieldsByID := map[int]BookVersionField{
		1: fields[0],
		2: fields[1],
		3: fields[2],
		4: fields[3],
	}

	heading, body := buildSubmissionPageContent(fields, values, fieldsByID)

	if heading != "Liz's Bannock" {
		t.Fatalf("expected heading to join heading fields with a space, got %q", heading)
	}
	if len(body) != 2 {
		t.Fatalf("expected 2 body blocks, got %#v", body)
	}
	if body[0].Label != "Ingredients:" || body[1].Label != "Method:" {
		t.Fatalf("expected body labels to be preserved per field, got %#v", body)
	}
	if body[0].Value != "3 cups flour\n\n1 cup water" {
		t.Fatalf("expected ingredients body to keep paragraph breaks, got %q", body[0].Value)
	}
	if body[1].Value != "Mix dry ingredients.\n\nBake until golden." {
		t.Fatalf("expected method body to keep paragraph breaks, got %q", body[1].Value)
	}
}

func TestDeriveInitialBookLayoutSettingsFromTemplatePages(t *testing.T) {
	t.Helper()

	sourcePDF := buildSyntheticBookTemplatePDF(t)

	layout := parseBookLayoutSettings(deriveInitialBookLayoutSettings(sourcePDF, 1, 2))

	if layout.ContentPage.TemplatePageNumber != 1 {
		t.Fatalf("expected content template page 1, got %d", layout.ContentPage.TemplatePageNumber)
	}
	if layout.SectionPage.TemplatePageNumber != 2 {
		t.Fatalf("expected section template page 2, got %d", layout.SectionPage.TemplatePageNumber)
	}
	if layout.ContentPage.PageWidth <= 0 || layout.ContentPage.PageHeight <= 0 {
		t.Fatalf("expected content page dimensions to be derived, got %#v", layout.ContentPage)
	}
	if layout.ContentPage.HeadingArea.FontFamily != "Helvetica" {
		t.Fatalf("expected content heading font family to map to Helvetica, got %#v", layout.ContentPage.HeadingArea)
	}
	if layout.ContentPage.HeadingArea.FontSize <= layout.ContentPage.BodyArea.FontSize {
		t.Fatalf("expected heading font size to be larger than body font size, got heading=%f body=%f", layout.ContentPage.HeadingArea.FontSize, layout.ContentPage.BodyArea.FontSize)
	}
	if layout.ContentPage.HeadingArea.Width <= 0 || layout.ContentPage.BodyArea.Width <= 0 {
		t.Fatalf("expected text areas to be derived from template geometry, got heading=%#v body=%#v", layout.ContentPage.HeadingArea, layout.ContentPage.BodyArea)
	}
	if layout.SectionPage.TitleArea.FontSize < 20 {
		t.Fatalf("expected divider title font size to be derived from template, got %#v", layout.SectionPage.TitleArea)
	}
}

func TestGenerateBookVersionPDFAppendsSubmissionAndSectionPages(t *testing.T) {
	t.Helper()

	sourcePDF := buildSyntheticBookTemplatePDF(t)
	layout := defaultBookLayoutSettingsModel()
	layout.ContentPage.TemplatePageNumber = 1
	layout.ContentPage.PageWidth = 612
	layout.ContentPage.PageHeight = 792
	layout.ContentPage.HeadingMask = bookMaskLayout{X: 52, Y: 42, Width: 508, Height: 78, BackgroundColor: "#7f9892", Alpha: 0.98}
	layout.ContentPage.HeadingArea = bookTextLayout{X: 92, Y: 58, Width: 340, Height: 46, FontFamily: "Helvetica", FontSize: 24, MinFontSize: 16, LineHeight: 1.1, TextAlign: "C", TextColor: "#f5f0e7"}
	layout.ContentPage.BodyMask = bookMaskLayout{X: 48, Y: 144, Width: 360, Height: 176, BackgroundColor: "#f7f0df", Alpha: 0.95}
	layout.ContentPage.BodyArea = bookBodyLayout{X: 60, Y: 154, Width: 278, Height: 156, FontFamily: "Helvetica", FontSize: 12, MinFontSize: 10, LineHeight: 1.25, TextAlign: "L", TextColor: "#2c261f", LabelFontFamily: "Helvetica", LabelFontStyle: "B", LabelFontSize: 12, LabelMinFontSize: 10, LabelTextColor: "#2c261f", ParagraphSpacing: 8}
	layout.SectionPage.TemplatePageNumber = 2
	layout.SectionPage.PageWidth = 612
	layout.SectionPage.PageHeight = 792
	layout.SectionPage.TitleMask = bookMaskLayout{X: 82, Y: 238, Width: 448, Height: 116, BackgroundColor: "#f7f0df", Alpha: 0.95}
	layout.SectionPage.TitleArea = bookTextLayout{X: 118, Y: 268, Width: 376, Height: 48, FontFamily: "Helvetica", FontStyle: "B", FontSize: 30, MinFontSize: 20, LineHeight: 1.1, TextAlign: "C", TextColor: "#2c261f"}
	version := BookVersion{
		ID:                        8,
		BookID:                    5,
		VersionNumber:             1,
		SourcePageCount:           3,
		ContentTemplatePageNumber: 1,
		SectionTemplatePageNumber: 2,
		LayoutSettings:            mustMarshalBookLayoutSettings(layout),
	}
	sections := []BookVersionSection{
		{ID: 1, BookVersionID: version.ID, Name: "Recipes", SourceStartPage: intPtr(3), SourceEndPage: intPtr(3), SortOrder: 0},
		{ID: 2, BookVersionID: version.ID, Name: "Campfire Classics", SortOrder: 1},
	}
	fields := []BookVersionField{
		{ID: 1, BookVersionID: version.ID, Label: "Title Part 1", Placement: BookFieldPlacementHeading, SortOrder: 0},
		{ID: 2, BookVersionID: version.ID, Label: "Title Part 2", Placement: BookFieldPlacementHeading, SortOrder: 1},
		{ID: 3, BookVersionID: version.ID, Label: "Ingredients", Placement: BookFieldPlacementBody, ShowLabel: true, SortOrder: 2},
		{ID: 4, BookVersionID: version.ID, Label: "Method", Placement: BookFieldPlacementBody, ShowLabel: true, SortOrder: 3},
	}
	submissions := []BookSubmission{
		{ID: 10, BookVersionID: version.ID, TargetSectionID: intPtr(1), Status: BookSubmissionStatusApproved},
		{ID: 11, BookVersionID: version.ID, TargetSectionID: intPtr(2), Status: BookSubmissionStatusApproved},
	}
	valuesBySubmission := map[int][]BookSubmissionValue{
		10: {
			{BookSubmissionID: 10, BookFieldID: 1, Value: "Liz's"},
			{BookSubmissionID: 10, BookFieldID: 2, Value: "Bannock"},
			{BookSubmissionID: 10, BookFieldID: 3, Value: "<p>3 cups flour</p><p>1 cup water</p>"},
			{BookSubmissionID: 10, BookFieldID: 4, Value: "<p>Mix dry ingredients.</p><p>Bake until golden.</p>"},
		},
		11: {
			{BookSubmissionID: 11, BookFieldID: 1, Value: "Campfire"},
			{BookSubmissionID: 11, BookFieldID: 2, Value: "Bread"},
			{BookSubmissionID: 11, BookFieldID: 3, Value: "<p>2 cups flour</p><p>1 cup water</p>"},
			{BookSubmissionID: 11, BookFieldID: 4, Value: "<p>Mix together.</p><p>Cook on a hot stone.</p>"},
		},
	}

	generatedPDF, err := generateBookVersionPDF(sourcePDF, version, sections, fields, submissions, valuesBySubmission, nil)
	if err != nil {
		t.Fatalf("generateBookVersionPDF returned error: %v", err)
	}

	reader, err := rpdf.NewReader(bytes.NewReader(generatedPDF), int64(len(generatedPDF)))
	if err != nil {
		t.Fatalf("open generated PDF: %v", err)
	}
	if reader.NumPage() != 6 {
		t.Fatalf("expected 6 pages after appending one recipe page and one generated-only section with its recipe page, got %d", reader.NumPage())
	}

	page4Text := compactPDFText(extractPDFPageText(t, generatedPDF, 4))
	page5Text := compactPDFText(extractPDFPageText(t, generatedPDF, 5))
	page6Text := compactPDFText(extractPDFPageText(t, generatedPDF, 6))

	if !strings.Contains(page4Text, "liz'sbannock") {
		t.Fatalf("expected appended recipe page to contain joined heading text, got %q", page4Text)
	}
	if !strings.Contains(page4Text, "ingredients:") || !strings.Contains(page4Text, "method:") {
		t.Fatalf("expected appended recipe page to contain body labels, got %q", page4Text)
	}
	if !strings.Contains(page5Text, "campfireclassics") {
		t.Fatalf("expected generated-only section divider page to contain section title, got %q", page5Text)
	}
	if !strings.Contains(page6Text, "campfirebread") {
		t.Fatalf("expected generated-only section recipe page to contain joined heading text, got %q", page6Text)
	}
}

func buildSyntheticBookTemplatePDF(t *testing.T) []byte {
	t.Helper()

	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "pt",
		Size:           gofpdf.SizeType{Wd: 612, Ht: 792},
	})
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)

	pdf.AddPage()
	pdf.SetFillColor(127, 152, 146)
	pdf.Rect(48, 36, 516, 86, "F")
	pdf.SetFont("Helvetica", "", 24)
	pdf.SetTextColor(245, 240, 231)
	pdf.Text(186, 88, "LIZ'S BANNOCK")
	pdf.SetTextColor(44, 38, 31)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.Text(60, 164, "INGREDIENTS:")
	pdf.SetFont("Helvetica", "", 12)
	pdf.Text(60, 184, "3 cups flour")
	pdf.Text(60, 202, "1 cup water")
	pdf.SetFont("Helvetica", "B", 13)
	pdf.Text(60, 244, "METHOD:")
	pdf.SetFont("Helvetica", "", 12)
	pdf.Text(60, 264, "Mix dry ingredients.")
	pdf.Text(60, 282, "Bake until golden.")

	pdf.AddPage()
	pdf.SetFillColor(247, 240, 223)
	pdf.Rect(82, 238, 448, 116, "F")
	pdf.SetFont("Helvetica", "B", 30)
	pdf.SetTextColor(44, 38, 31)
	pdf.Text(156, 312, "MAIN RECIPES")

	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 14)
	pdf.SetTextColor(0, 0, 0)
	pdf.Text(72, 100, "Existing cookbook content")

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("build synthetic book template pdf: %v", err)
	}
	return out.Bytes()
}

func extractPDFPageText(t *testing.T, pdfBytes []byte, pageNumber int) string {
	t.Helper()

	reader, err := rpdf.NewReader(bytes.NewReader(pdfBytes), int64(len(pdfBytes)))
	if err != nil {
		t.Fatalf("open pdf reader: %v", err)
	}
	page := reader.Page(pageNumber)
	_, pageHeight := extractPageDimensions(page)
	lines := extractTemplateLines(page, pageHeight)
	text := make([]string, 0, len(lines))
	for _, line := range lines {
		text = append(text, line.Text)
	}
	return strings.Join(text, "\n")
}

func compactPDFText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), ""))
}
