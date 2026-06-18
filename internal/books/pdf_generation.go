package books

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	htmlstd "html"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/phpdave11/gofpdf"
	"github.com/phpdave11/gofpdf/contrib/gofpdi"
	"golang.org/x/net/html"
	rpdf "rsc.io/pdf"
)

const (
	bookLayoutSettingsVersion = 2
	bookDefaultPDFMimeType    = "application/pdf"
	bookDefaultPageWidth      = 612.0
	bookDefaultPageHeight     = 792.0
)

type bookLayoutSettings struct {
	Version     int                   `json:"version"`
	ContentPage bookContentPageLayout `json:"content_page"`
	SectionPage bookSectionPageLayout `json:"section_page"`
}

type bookContentPageLayout struct {
	TemplatePageNumber int             `json:"template_page_number,omitempty"`
	PageWidth          float64         `json:"page_width"`
	PageHeight         float64         `json:"page_height"`
	HeadingMask        bookMaskLayout  `json:"heading_mask"`
	HeadingArea        bookTextLayout  `json:"heading_area"`
	BodyMask           bookMaskLayout  `json:"body_mask"`
	BodyArea           bookBodyLayout  `json:"body_area"`
	ImageArea          bookImageLayout `json:"image_area"`
}

type bookSectionPageLayout struct {
	TemplatePageNumber int            `json:"template_page_number,omitempty"`
	PageWidth          float64        `json:"page_width"`
	PageHeight         float64        `json:"page_height"`
	TitleMask          bookMaskLayout `json:"title_mask"`
	TitleArea          bookTextLayout `json:"title_area"`
}

type bookMaskLayout struct {
	X               float64 `json:"x"`
	Y               float64 `json:"y"`
	Width           float64 `json:"width"`
	Height          float64 `json:"height"`
	BackgroundColor string  `json:"background_color"`
	Alpha           float64 `json:"alpha"`
}

type bookTextLayout struct {
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	FontFamily  string  `json:"font_family"`
	FontStyle   string  `json:"font_style"`
	FontSize    float64 `json:"font_size"`
	MinFontSize float64 `json:"min_font_size"`
	LineHeight  float64 `json:"line_height"`
	TextAlign   string  `json:"text_align"`
	TextColor   string  `json:"text_color"`
}

type bookBodyLayout struct {
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	Width            float64 `json:"width"`
	Height           float64 `json:"height"`
	FontFamily       string  `json:"font_family"`
	FontStyle        string  `json:"font_style"`
	FontSize         float64 `json:"font_size"`
	MinFontSize      float64 `json:"min_font_size"`
	LineHeight       float64 `json:"line_height"`
	TextAlign        string  `json:"text_align"`
	TextColor        string  `json:"text_color"`
	LabelFontFamily  string  `json:"label_font_family"`
	LabelFontStyle   string  `json:"label_font_style"`
	LabelFontSize    float64 `json:"label_font_size"`
	LabelMinFontSize float64 `json:"label_min_font_size"`
	LabelTextColor   string  `json:"label_text_color"`
	ParagraphSpacing float64 `json:"paragraph_spacing"`
}

type bookImageLayout struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Alpha  float64 `json:"alpha"`
}

type legacyBookLayoutSettings struct {
	ContentMask bookMaskLayout `json:"content_mask"`
	HeadingArea bookTextLayout `json:"heading_area"`
	BodyArea    struct {
		X          float64 `json:"x"`
		Y          float64 `json:"y"`
		Width      float64 `json:"width"`
		Height     float64 `json:"height"`
		FontSize   float64 `json:"font_size"`
		LineHeight float64 `json:"line_height"`
		TextAlign  string  `json:"text_align"`
	} `json:"body_area"`
	ImageArea        bookImageLayout `json:"image_area"`
	SectionMask      bookMaskLayout  `json:"section_mask"`
	SectionTitleArea bookTextLayout  `json:"section_title_area"`
}

type bookImportedTemplate struct {
	ID     int
	Width  float64
	Height float64
}

type bookTextLine struct {
	Text     string
	X        float64
	Y        float64
	Width    float64
	Height   float64
	FontSize float64
	FontName string
}

type bookRect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

type bookBodyBlock struct {
	Label string
	Value string
}

type bookSubmissionRenderData struct {
	SectionID int
	Heading   string
	Body      []bookBodyBlock
	Image     *storedBookUploadContent
}

type storedBookUploadContent struct {
	Data     []byte
	MimeType string
	FileName string
}

func defaultBookLayoutSettingsModel() bookLayoutSettings {
	return bookLayoutSettings{
		Version: bookLayoutSettingsVersion,
		ContentPage: bookContentPageLayout{
			HeadingMask: bookMaskLayout{
				X:               48,
				Y:               34,
				Width:           516,
				Height:          110,
				BackgroundColor: "#7f9892",
				Alpha:           1,
			},
			HeadingArea: bookTextLayout{
				X:           86,
				Y:           58,
				Width:       440,
				Height:      54,
				FontFamily:  "Helvetica",
				FontStyle:   "",
				FontSize:    20,
				MinFontSize: 13,
				LineHeight:  1.2,
				TextAlign:   "C",
				TextColor:   "#f5f0e7",
			},
			BodyMask: bookMaskLayout{
				X:               54,
				Y:               140,
				Width:           404,
				Height:          390,
				BackgroundColor: "#f7f0df",
				Alpha:           1,
			},
			BodyArea: bookBodyLayout{
				X:                72,
				Y:                160,
				Width:            330,
				Height:           350,
				FontFamily:       "Helvetica",
				FontStyle:        "",
				FontSize:         11,
				MinFontSize:      8,
				LineHeight:       1.35,
				TextAlign:        "L",
				TextColor:        "#2c261f",
				LabelFontFamily:  "Helvetica",
				LabelFontStyle:   "B",
				LabelFontSize:    11,
				LabelMinFontSize: 8,
				LabelTextColor:   "#2c261f",
				ParagraphSpacing: 10,
			},
			ImageArea: bookImageLayout{
				X:      314,
				Y:      542,
				Width:  120,
				Height: 120,
				Alpha:  0.88,
			},
		},
		SectionPage: bookSectionPageLayout{
			TitleMask: bookMaskLayout{
				X:               70,
				Y:               228,
				Width:           360,
				Height:          114,
				BackgroundColor: "#f7f0df",
				Alpha:           0.92,
			},
			TitleArea: bookTextLayout{
				X:           90,
				Y:           248,
				Width:       320,
				Height:      74,
				FontFamily:  "Helvetica",
				FontStyle:   "B",
				FontSize:    28,
				MinFontSize: 18,
				LineHeight:  1.1,
				TextAlign:   "C",
				TextColor:   "#2c261f",
			},
		},
	}
}

func mustMarshalBookLayoutSettings(layout bookLayoutSettings) json.RawMessage {
	data, err := json.Marshal(layout)
	if err != nil {
		panic(err)
	}
	return data
}

func parseBookLayoutSettings(raw json.RawMessage) bookLayoutSettings {
	layout := defaultBookLayoutSettingsModel()
	if len(strings.TrimSpace(string(raw))) == 0 {
		return layout
	}

	var decoded bookLayoutSettings
	if err := json.Unmarshal(raw, &decoded); err == nil {
		if decoded.ContentPage.HeadingArea.Width > 0 || decoded.SectionPage.TitleArea.Width > 0 {
			return normalizeBookLayoutSettingsModel(decoded)
		}
	}

	var legacy legacyBookLayoutSettings
	if err := json.Unmarshal(raw, &legacy); err == nil {
		layout.ContentPage.HeadingMask = normalizeMaskLayout(legacy.ContentMask, layout.ContentPage.HeadingMask)
		layout.ContentPage.HeadingArea = normalizeTextLayout(legacy.HeadingArea, layout.ContentPage.HeadingArea)
		layout.ContentPage.BodyMask = normalizeMaskLayout(legacy.ContentMask, layout.ContentPage.BodyMask)
		layout.ContentPage.BodyArea = normalizeBodyLayout(bookBodyLayout{
			X:                legacy.BodyArea.X,
			Y:                legacy.BodyArea.Y,
			Width:            legacy.BodyArea.Width,
			Height:           legacy.BodyArea.Height,
			FontFamily:       layout.ContentPage.BodyArea.FontFamily,
			FontStyle:        layout.ContentPage.BodyArea.FontStyle,
			FontSize:         legacy.BodyArea.FontSize,
			MinFontSize:      layout.ContentPage.BodyArea.MinFontSize,
			LineHeight:       legacy.BodyArea.LineHeight,
			TextAlign:        legacy.BodyArea.TextAlign,
			TextColor:        layout.ContentPage.BodyArea.TextColor,
			LabelFontFamily:  layout.ContentPage.BodyArea.LabelFontFamily,
			LabelFontStyle:   layout.ContentPage.BodyArea.LabelFontStyle,
			LabelFontSize:    legacy.BodyArea.FontSize,
			LabelMinFontSize: layout.ContentPage.BodyArea.LabelMinFontSize,
			LabelTextColor:   layout.ContentPage.BodyArea.LabelTextColor,
			ParagraphSpacing: layout.ContentPage.BodyArea.ParagraphSpacing,
		}, layout.ContentPage.BodyArea)
		layout.ContentPage.ImageArea = normalizeImageLayout(legacy.ImageArea, layout.ContentPage.ImageArea)
		layout.SectionPage.TitleMask = normalizeMaskLayout(legacy.SectionMask, layout.SectionPage.TitleMask)
		layout.SectionPage.TitleArea = normalizeTextLayout(legacy.SectionTitleArea, layout.SectionPage.TitleArea)
		return normalizeBookLayoutSettingsModel(layout)
	}

	return layout
}

func normalizeBookLayoutSettingsModel(layout bookLayoutSettings) bookLayoutSettings {
	if layout.Version == 0 {
		layout.Version = bookLayoutSettingsVersion
	}
	contentDefaults := scaledDefaultContentPageLayout(
		layout.ContentPage.TemplatePageNumber,
		layout.ContentPage.PageWidth,
		layout.ContentPage.PageHeight,
	)
	sectionDefaults := scaledDefaultSectionPageLayout(
		layout.SectionPage.TemplatePageNumber,
		layout.SectionPage.PageWidth,
		layout.SectionPage.PageHeight,
	)
	layout.ContentPage = repairContentPageLayout(
		normalizeContentPageLayout(layout.ContentPage, contentDefaults),
		contentDefaults,
	)
	layout.SectionPage = repairSectionPageLayout(
		normalizeSectionPageLayout(layout.SectionPage, sectionDefaults),
		sectionDefaults,
	)
	return layout
}

func normalizeContentPageLayout(layout bookContentPageLayout, defaults bookContentPageLayout) bookContentPageLayout {
	if layout.TemplatePageNumber == 0 {
		layout.TemplatePageNumber = defaults.TemplatePageNumber
	}
	if layout.PageWidth <= 0 {
		layout.PageWidth = defaults.PageWidth
	}
	if layout.PageHeight <= 0 {
		layout.PageHeight = defaults.PageHeight
	}
	layout.HeadingMask = normalizeMaskLayout(layout.HeadingMask, defaults.HeadingMask)
	layout.HeadingArea = normalizeTextLayout(layout.HeadingArea, defaults.HeadingArea)
	layout.BodyMask = normalizeMaskLayout(layout.BodyMask, defaults.BodyMask)
	layout.BodyArea = normalizeBodyLayout(layout.BodyArea, defaults.BodyArea)
	layout.ImageArea = normalizeImageLayout(layout.ImageArea, defaults.ImageArea)
	return layout
}

func normalizeSectionPageLayout(layout bookSectionPageLayout, defaults bookSectionPageLayout) bookSectionPageLayout {
	if layout.TemplatePageNumber == 0 {
		layout.TemplatePageNumber = defaults.TemplatePageNumber
	}
	if layout.PageWidth <= 0 {
		layout.PageWidth = defaults.PageWidth
	}
	if layout.PageHeight <= 0 {
		layout.PageHeight = defaults.PageHeight
	}
	layout.TitleMask = normalizeMaskLayout(layout.TitleMask, defaults.TitleMask)
	layout.TitleArea = normalizeTextLayout(layout.TitleArea, defaults.TitleArea)
	return layout
}

func normalizeMaskLayout(mask bookMaskLayout, defaults bookMaskLayout) bookMaskLayout {
	mask.X = choosePositiveFloat(mask.X, defaults.X)
	mask.Y = choosePositiveFloat(mask.Y, defaults.Y)
	mask.Width = choosePositiveFloat(mask.Width, defaults.Width)
	mask.Height = choosePositiveFloat(mask.Height, defaults.Height)
	mask.BackgroundColor = chooseNonEmpty(strings.TrimSpace(mask.BackgroundColor), defaults.BackgroundColor)
	if mask.Alpha <= 0 || mask.Alpha > 1 {
		mask.Alpha = defaults.Alpha
	}
	return mask
}

func normalizeTextLayout(layout bookTextLayout, defaults bookTextLayout) bookTextLayout {
	layout.X = choosePositiveFloat(layout.X, defaults.X)
	layout.Y = choosePositiveFloat(layout.Y, defaults.Y)
	layout.Width = choosePositiveFloat(layout.Width, defaults.Width)
	layout.Height = choosePositiveFloat(layout.Height, defaults.Height)
	layout.FontFamily = chooseNonEmpty(strings.TrimSpace(layout.FontFamily), defaults.FontFamily)
	layout.FontStyle = chooseNonEmpty(strings.TrimSpace(layout.FontStyle), defaults.FontStyle)
	layout.FontSize = choosePositiveFloat(layout.FontSize, defaults.FontSize)
	layout.MinFontSize = choosePositiveFloat(layout.MinFontSize, defaults.MinFontSize)
	layout.LineHeight = choosePositiveFloat(layout.LineHeight, defaults.LineHeight)
	layout.TextAlign = normalizePDFTextAlign(layout.TextAlign, defaults.TextAlign)
	layout.TextColor = chooseNonEmpty(strings.TrimSpace(layout.TextColor), defaults.TextColor)
	if layout.MinFontSize > layout.FontSize {
		layout.MinFontSize = layout.FontSize
	}
	return layout
}

func normalizeBodyLayout(layout bookBodyLayout, defaults bookBodyLayout) bookBodyLayout {
	layout.X = choosePositiveFloat(layout.X, defaults.X)
	layout.Y = choosePositiveFloat(layout.Y, defaults.Y)
	layout.Width = choosePositiveFloat(layout.Width, defaults.Width)
	layout.Height = choosePositiveFloat(layout.Height, defaults.Height)
	layout.FontFamily = chooseNonEmpty(strings.TrimSpace(layout.FontFamily), defaults.FontFamily)
	layout.FontStyle = chooseNonEmpty(strings.TrimSpace(layout.FontStyle), defaults.FontStyle)
	layout.FontSize = choosePositiveFloat(layout.FontSize, defaults.FontSize)
	layout.MinFontSize = choosePositiveFloat(layout.MinFontSize, defaults.MinFontSize)
	layout.LineHeight = choosePositiveFloat(layout.LineHeight, defaults.LineHeight)
	layout.TextAlign = normalizePDFTextAlign(layout.TextAlign, defaults.TextAlign)
	layout.TextColor = chooseNonEmpty(strings.TrimSpace(layout.TextColor), defaults.TextColor)
	layout.LabelFontFamily = chooseNonEmpty(strings.TrimSpace(layout.LabelFontFamily), defaults.LabelFontFamily)
	layout.LabelFontStyle = chooseNonEmpty(strings.TrimSpace(layout.LabelFontStyle), defaults.LabelFontStyle)
	layout.LabelFontSize = choosePositiveFloat(layout.LabelFontSize, defaults.LabelFontSize)
	layout.LabelMinFontSize = choosePositiveFloat(layout.LabelMinFontSize, defaults.LabelMinFontSize)
	layout.LabelTextColor = chooseNonEmpty(strings.TrimSpace(layout.LabelTextColor), defaults.LabelTextColor)
	layout.ParagraphSpacing = choosePositiveFloat(layout.ParagraphSpacing, defaults.ParagraphSpacing)
	if layout.MinFontSize > layout.FontSize {
		layout.MinFontSize = layout.FontSize
	}
	if layout.LabelMinFontSize > layout.LabelFontSize {
		layout.LabelMinFontSize = layout.LabelFontSize
	}
	return layout
}

func normalizeImageLayout(layout bookImageLayout, defaults bookImageLayout) bookImageLayout {
	layout.X = choosePositiveFloat(layout.X, defaults.X)
	layout.Y = choosePositiveFloat(layout.Y, defaults.Y)
	layout.Width = choosePositiveFloat(layout.Width, defaults.Width)
	layout.Height = choosePositiveFloat(layout.Height, defaults.Height)
	if layout.Alpha <= 0 || layout.Alpha > 1 {
		layout.Alpha = defaults.Alpha
	}
	return layout
}

func scaledDefaultContentPageLayout(pageNumber int, pageWidth float64, pageHeight float64) bookContentPageLayout {
	defaults := defaultBookLayoutSettingsModel().ContentPage
	if pageWidth <= 0 {
		pageWidth = bookDefaultPageWidth
	}
	if pageHeight <= 0 {
		pageHeight = bookDefaultPageHeight
	}

	scaleX := pageWidth / bookDefaultPageWidth
	scaleY := pageHeight / bookDefaultPageHeight
	fontScale := math.Min(scaleX, scaleY)

	defaults.TemplatePageNumber = pageNumber
	defaults.PageWidth = pageWidth
	defaults.PageHeight = pageHeight
	defaults.HeadingMask = scaleMaskLayout(defaults.HeadingMask, scaleX, scaleY)
	defaults.HeadingArea = scaleTextLayout(defaults.HeadingArea, scaleX, scaleY, fontScale)
	defaults.BodyMask = scaleMaskLayout(defaults.BodyMask, scaleX, scaleY)
	defaults.BodyArea = scaleBodyLayout(defaults.BodyArea, scaleX, scaleY, fontScale)
	defaults.ImageArea = scaleImageLayout(defaults.ImageArea, scaleX, scaleY)
	return defaults
}

func scaledDefaultSectionPageLayout(pageNumber int, pageWidth float64, pageHeight float64) bookSectionPageLayout {
	defaults := defaultBookLayoutSettingsModel().SectionPage
	if pageWidth <= 0 {
		pageWidth = bookDefaultPageWidth
	}
	if pageHeight <= 0 {
		pageHeight = bookDefaultPageHeight
	}

	scaleX := pageWidth / bookDefaultPageWidth
	scaleY := pageHeight / bookDefaultPageHeight
	fontScale := math.Min(scaleX, scaleY)

	defaults.TemplatePageNumber = pageNumber
	defaults.PageWidth = pageWidth
	defaults.PageHeight = pageHeight
	defaults.TitleMask = scaleMaskLayout(defaults.TitleMask, scaleX, scaleY)
	defaults.TitleArea = scaleTextLayout(defaults.TitleArea, scaleX, scaleY, fontScale)
	return defaults
}

func scaleMaskLayout(layout bookMaskLayout, scaleX float64, scaleY float64) bookMaskLayout {
	layout.X *= scaleX
	layout.Y *= scaleY
	layout.Width *= scaleX
	layout.Height *= scaleY
	return layout
}

func scaleTextLayout(layout bookTextLayout, scaleX float64, scaleY float64, fontScale float64) bookTextLayout {
	layout.X *= scaleX
	layout.Y *= scaleY
	layout.Width *= scaleX
	layout.Height *= scaleY
	layout.FontSize *= fontScale
	layout.MinFontSize *= fontScale
	return layout
}

func scaleBodyLayout(layout bookBodyLayout, scaleX float64, scaleY float64, fontScale float64) bookBodyLayout {
	layout.X *= scaleX
	layout.Y *= scaleY
	layout.Width *= scaleX
	layout.Height *= scaleY
	layout.FontSize *= fontScale
	layout.MinFontSize *= fontScale
	layout.LabelFontSize *= fontScale
	layout.LabelMinFontSize *= fontScale
	layout.ParagraphSpacing *= fontScale
	return layout
}

func scaleImageLayout(layout bookImageLayout, scaleX float64, scaleY float64) bookImageLayout {
	layout.X *= scaleX
	layout.Y *= scaleY
	layout.Width *= scaleX
	layout.Height *= scaleY
	return layout
}

func pageDimensionsFromReader(reader *rpdf.Reader, pageNumber int) (float64, float64) {
	if reader == nil || pageNumber <= 0 || pageNumber > reader.NumPage() {
		return bookDefaultPageWidth, bookDefaultPageHeight
	}
	pageWidth, pageHeight := extractPageDimensions(reader.Page(pageNumber))
	if pageWidth <= 0 || pageHeight <= 0 {
		return bookDefaultPageWidth, bookDefaultPageHeight
	}
	return pageWidth, pageHeight
}

func resolveBookLayoutSettings(reader *rpdf.Reader, version BookVersion, layout bookLayoutSettings) bookLayoutSettings {
	if layout.ContentPage.TemplatePageNumber == 0 {
		layout.ContentPage.TemplatePageNumber = version.ContentTemplatePageNumber
	}
	if layout.SectionPage.TemplatePageNumber == 0 {
		layout.SectionPage.TemplatePageNumber = version.SectionTemplatePageNumber
	}

	contentWidth, contentHeight := pageDimensionsFromReader(reader, layout.ContentPage.TemplatePageNumber)
	sectionWidth, sectionHeight := pageDimensionsFromReader(reader, layout.SectionPage.TemplatePageNumber)
	contentDefaults := scaledDefaultContentPageLayout(layout.ContentPage.TemplatePageNumber, contentWidth, contentHeight)
	sectionDefaults := scaledDefaultSectionPageLayout(layout.SectionPage.TemplatePageNumber, sectionWidth, sectionHeight)

	layout.ContentPage = repairContentPageLayout(
		normalizeContentPageLayout(layout.ContentPage, contentDefaults),
		contentDefaults,
	)
	layout.SectionPage = repairSectionPageLayout(
		normalizeSectionPageLayout(layout.SectionPage, sectionDefaults),
		sectionDefaults,
	)
	return layout
}

func repairContentPageLayout(layout bookContentPageLayout, defaults bookContentPageLayout) bookContentPageLayout {
	layout.TemplatePageNumber = defaults.TemplatePageNumber
	layout.PageWidth = defaults.PageWidth
	layout.PageHeight = defaults.PageHeight
	layout.HeadingMask = clampMaskLayout(layout.HeadingMask, layout.PageWidth, layout.PageHeight)
	layout.HeadingArea = clampTextLayout(layout.HeadingArea, layout.PageWidth, layout.PageHeight)
	layout.BodyMask = clampMaskLayout(layout.BodyMask, layout.PageWidth, layout.PageHeight)
	layout.BodyArea = clampBodyLayout(layout.BodyArea, layout.PageWidth, layout.PageHeight)
	layout.ImageArea = clampImageLayout(layout.ImageArea, layout.PageWidth, layout.PageHeight)

	if !isPlausibleMaskLayout(layout.HeadingMask, layout.PageWidth, layout.PageHeight, layout.PageWidth*0.22, layout.PageHeight*0.05) ||
		!isPlausibleTextLayout(layout.HeadingArea, layout.PageWidth, layout.PageHeight, layout.PageWidth*0.18, layout.PageHeight*0.03) ||
		layout.HeadingArea.Y > layout.PageHeight*0.12 ||
		layout.HeadingMask.Y > layout.PageHeight*0.1 {
		layout.HeadingMask = defaults.HeadingMask
		layout.HeadingArea = defaults.HeadingArea
	}

	if !isPlausibleMaskLayout(layout.BodyMask, layout.PageWidth, layout.PageHeight, layout.PageWidth*0.26, layout.PageHeight*0.12) ||
		!isPlausibleBodyLayout(layout.BodyArea, layout.PageWidth, layout.PageHeight) ||
		layout.BodyArea.Y <= layout.HeadingArea.Y+layout.HeadingArea.Height*0.25 ||
		layout.BodyArea.Y > layout.PageHeight*0.25 ||
		layout.BodyMask.Y > layout.PageHeight*0.24 {
		layout.BodyMask = defaults.BodyMask
		layout.BodyArea = defaults.BodyArea
	}

	if !isPlausibleImageLayout(layout.ImageArea, layout.PageWidth, layout.PageHeight) {
		layout.ImageArea = defaults.ImageArea
	}

	if layout.HeadingArea.FontSize < layout.BodyArea.FontSize {
		layout.HeadingArea.FontSize = defaults.HeadingArea.FontSize
		layout.HeadingArea.MinFontSize = defaults.HeadingArea.MinFontSize
		layout.HeadingArea.TextAlign = defaults.HeadingArea.TextAlign
	}
	return layout
}

func mergedContentEraseLayout(layout bookContentPageLayout, defaults bookContentPageLayout) bookContentPageLayout {
	layout.HeadingMask = mergeEraseMask(layout.HeadingMask, defaults.HeadingMask, layout.PageWidth, layout.PageHeight)
	layout.BodyMask = mergeEraseMask(layout.BodyMask, defaults.BodyMask, layout.PageWidth, layout.PageHeight)
	return layout
}

func mergedSectionEraseLayout(layout bookSectionPageLayout, defaults bookSectionPageLayout) bookSectionPageLayout {
	layout.TitleMask = mergeEraseMask(layout.TitleMask, defaults.TitleMask, layout.PageWidth, layout.PageHeight)
	return layout
}

func mergeEraseMask(layout bookMaskLayout, defaults bookMaskLayout, pageWidth float64, pageHeight float64) bookMaskLayout {
	left := math.Min(layout.X, defaults.X)
	top := math.Min(layout.Y, defaults.Y)
	right := math.Max(layout.X+layout.Width, defaults.X+defaults.Width)
	bottom := math.Max(layout.Y+layout.Height, defaults.Y+defaults.Height)
	merged := defaults
	merged.X = left
	merged.Y = top
	merged.Width = right - left
	merged.Height = bottom - top
	merged.Alpha = 1
	return clampMaskLayout(merged, pageWidth, pageHeight)
}

func repairSectionPageLayout(layout bookSectionPageLayout, defaults bookSectionPageLayout) bookSectionPageLayout {
	layout.TemplatePageNumber = defaults.TemplatePageNumber
	layout.PageWidth = defaults.PageWidth
	layout.PageHeight = defaults.PageHeight
	layout.TitleMask = clampMaskLayout(layout.TitleMask, layout.PageWidth, layout.PageHeight)
	layout.TitleArea = clampTextLayout(layout.TitleArea, layout.PageWidth, layout.PageHeight)

	if !isPlausibleMaskLayout(layout.TitleMask, layout.PageWidth, layout.PageHeight, layout.PageWidth*0.26, layout.PageHeight*0.08) ||
		!isPlausibleTextLayout(layout.TitleArea, layout.PageWidth, layout.PageHeight, layout.PageWidth*0.22, layout.PageHeight*0.05) {
		layout.TitleMask = defaults.TitleMask
		layout.TitleArea = defaults.TitleArea
	}
	return layout
}

func clampMaskLayout(layout bookMaskLayout, pageWidth float64, pageHeight float64) bookMaskLayout {
	rect := clampRect(bookRect{
		X:      layout.X,
		Y:      layout.Y,
		Width:  layout.Width,
		Height: layout.Height,
	}, pageWidth, pageHeight)
	layout.X = rect.X
	layout.Y = rect.Y
	layout.Width = rect.Width
	layout.Height = rect.Height
	return layout
}

func clampTextLayout(layout bookTextLayout, pageWidth float64, pageHeight float64) bookTextLayout {
	rect := clampRect(bookRect{
		X:      layout.X,
		Y:      layout.Y,
		Width:  layout.Width,
		Height: layout.Height,
	}, pageWidth, pageHeight)
	layout.X = rect.X
	layout.Y = rect.Y
	layout.Width = rect.Width
	layout.Height = rect.Height
	return layout
}

func clampBodyLayout(layout bookBodyLayout, pageWidth float64, pageHeight float64) bookBodyLayout {
	rect := clampRect(bookRect{
		X:      layout.X,
		Y:      layout.Y,
		Width:  layout.Width,
		Height: layout.Height,
	}, pageWidth, pageHeight)
	layout.X = rect.X
	layout.Y = rect.Y
	layout.Width = rect.Width
	layout.Height = rect.Height
	return layout
}

func clampImageLayout(layout bookImageLayout, pageWidth float64, pageHeight float64) bookImageLayout {
	rect := clampRect(bookRect{
		X:      layout.X,
		Y:      layout.Y,
		Width:  layout.Width,
		Height: layout.Height,
	}, pageWidth, pageHeight)
	layout.X = rect.X
	layout.Y = rect.Y
	layout.Width = rect.Width
	layout.Height = rect.Height
	return layout
}

func isPlausibleMaskLayout(layout bookMaskLayout, pageWidth float64, pageHeight float64, minWidth float64, minHeight float64) bool {
	return layout.X >= 0 && layout.Y >= 0 &&
		layout.Width >= minWidth &&
		layout.Height >= minHeight &&
		layout.X+layout.Width <= pageWidth+0.1 &&
		layout.Y+layout.Height <= pageHeight+0.1
}

func isPlausibleTextLayout(layout bookTextLayout, pageWidth float64, pageHeight float64, minWidth float64, minHeight float64) bool {
	return layout.X >= 0 && layout.Y >= 0 &&
		layout.Width >= minWidth &&
		layout.Height >= minHeight &&
		layout.FontSize >= 8 &&
		layout.X+layout.Width <= pageWidth+0.1 &&
		layout.Y+layout.Height <= pageHeight+0.1
}

func isPlausibleBodyLayout(layout bookBodyLayout, pageWidth float64, pageHeight float64) bool {
	return layout.X >= 0 && layout.Y >= 0 &&
		layout.Width >= pageWidth*0.22 &&
		layout.Height >= pageHeight*0.14 &&
		layout.FontSize >= 8 &&
		layout.X+layout.Width <= pageWidth+0.1 &&
		layout.Y+layout.Height <= pageHeight+0.1
}

func isPlausibleImageLayout(layout bookImageLayout, pageWidth float64, pageHeight float64) bool {
	return layout.X >= 0 && layout.Y >= 0 &&
		layout.Width >= pageWidth*0.1 &&
		layout.Height >= pageHeight*0.08 &&
		layout.X+layout.Width <= pageWidth+0.1 &&
		layout.Y+layout.Height <= pageHeight+0.1
}

func deriveInitialBookLayoutSettings(sourcePDF []byte, contentTemplatePageNumber int, sectionTemplatePageNumber int) json.RawMessage {
	layout := bookLayoutSettings{
		Version: bookLayoutSettingsVersion,
		ContentPage: scaledDefaultContentPageLayout(
			contentTemplatePageNumber,
			bookDefaultPageWidth,
			bookDefaultPageHeight,
		),
		SectionPage: scaledDefaultSectionPageLayout(
			sectionTemplatePageNumber,
			bookDefaultPageWidth,
			bookDefaultPageHeight,
		),
	}

	reader, err := rpdf.NewReader(bytes.NewReader(sourcePDF), int64(len(sourcePDF)))
	if err != nil {
		return mustMarshalBookLayoutSettings(layout)
	}

	contentWidth, contentHeight := pageDimensionsFromReader(reader, contentTemplatePageNumber)
	sectionWidth, sectionHeight := pageDimensionsFromReader(reader, sectionTemplatePageNumber)
	contentDefaults := scaledDefaultContentPageLayout(
		contentTemplatePageNumber,
		contentWidth,
		contentHeight,
	)
	sectionDefaults := scaledDefaultSectionPageLayout(
		sectionTemplatePageNumber,
		sectionWidth,
		sectionHeight,
	)
	layout.ContentPage = contentDefaults
	layout.SectionPage = sectionDefaults

	if analysis, ok := analyzeTemplatePage(reader.Page(contentTemplatePageNumber)); ok {
		applyContentTemplateAnalysis(&layout.ContentPage, analysis)
	}
	if analysis, ok := analyzeTemplatePage(reader.Page(sectionTemplatePageNumber)); ok {
		applySectionTemplateAnalysis(&layout.SectionPage, analysis)
	}
	layout.ContentPage = repairContentPageLayout(layout.ContentPage, contentDefaults)
	layout.SectionPage = repairSectionPageLayout(layout.SectionPage, sectionDefaults)

	return mustMarshalBookLayoutSettings(layout)
}

func generateBookVersionPDF(sourcePDF []byte, version BookVersion, sections []BookVersionSection, fields []BookVersionField, submissions []BookSubmission, valuesBySubmission map[int][]BookSubmissionValue, imagesBySubmission map[int]*storedBookUploadContent) ([]byte, error) {
	if len(sourcePDF) == 0 {
		return nil, errors.New("source PDF content is required")
	}

	reader, err := rpdf.NewReader(bytes.NewReader(sourcePDF), int64(len(sourcePDF)))
	if err != nil {
		return nil, err
	}

	layout := parseBookLayoutSettings(version.LayoutSettings)
	if layout.ContentPage.TemplatePageNumber == 0 {
		layout.ContentPage.TemplatePageNumber = version.ContentTemplatePageNumber
	}
	if layout.SectionPage.TemplatePageNumber == 0 {
		layout.SectionPage.TemplatePageNumber = version.SectionTemplatePageNumber
	}
	layout = resolveBookLayoutSettings(reader, version, layout)

	pdfDoc := gofpdf.NewCustom(&gofpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "pt",
		Size:           gofpdf.SizeType{Wd: 612, Ht: 792},
	})
	pdfDoc.SetMargins(0, 0, 0)
	pdfDoc.SetAutoPageBreak(false, 0)
	pdfDoc.SetCompression(true)

	importer := gofpdi.NewImporter()
	templateCache := make(map[int]bookImportedTemplate)
	submissionsBySection := groupSubmissionRenderData(fields, submissions, valuesBySubmission, imagesBySubmission)

	sourceCursor := 1
	sourcePageCount := reader.NumPage()

	for _, section := range sections {
		if section.SourceStartPage != nil && section.SourceEndPage != nil {
			for sourceCursor < *section.SourceStartPage && sourceCursor <= sourcePageCount {
				if err := appendImportedPage(pdfDoc, importer, templateCache, sourcePDF, sourceCursor); err != nil {
					return nil, err
				}
				sourceCursor++
			}

			for pageNo := *section.SourceStartPage; pageNo <= *section.SourceEndPage && pageNo <= sourcePageCount; pageNo++ {
				if err := appendImportedPage(pdfDoc, importer, templateCache, sourcePDF, pageNo); err != nil {
					return nil, err
				}
			}
			sourceCursor = *section.SourceEndPage + 1

			for _, submission := range submissionsBySection[section.ID] {
				if err := appendContentTemplatePage(pdfDoc, importer, templateCache, sourcePDF, layout.ContentPage, submission); err != nil {
					return nil, err
				}
			}
			continue
		}

		for sourceCursor <= sourcePageCount {
			if err := appendImportedPage(pdfDoc, importer, templateCache, sourcePDF, sourceCursor); err != nil {
				return nil, err
			}
			sourceCursor++
		}

		if err := appendSectionTemplatePage(pdfDoc, importer, templateCache, sourcePDF, layout.SectionPage, strings.TrimSpace(section.Name)); err != nil {
			return nil, err
		}
		for _, submission := range submissionsBySection[section.ID] {
			if err := appendContentTemplatePage(pdfDoc, importer, templateCache, sourcePDF, layout.ContentPage, submission); err != nil {
				return nil, err
			}
		}
	}

	for sourceCursor <= sourcePageCount {
		if err := appendImportedPage(pdfDoc, importer, templateCache, sourcePDF, sourceCursor); err != nil {
			return nil, err
		}
		sourceCursor++
	}

	var out bytes.Buffer
	if err := pdfDoc.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func appendImportedPage(pdfDoc *gofpdf.Fpdf, importer *gofpdi.Importer, cache map[int]bookImportedTemplate, sourcePDF []byte, pageNumber int) error {
	template, err := importTemplatePage(pdfDoc, importer, cache, sourcePDF, pageNumber)
	if err != nil {
		return err
	}

	pdfDoc.AddPageFormat(pageOrientation(template.Width, template.Height), gofpdf.SizeType{Wd: template.Width, Ht: template.Height})
	importer.UseImportedTemplate(pdfDoc, template.ID, 0, 0, template.Width, template.Height)
	return pdfDoc.Error()
}

func appendContentTemplatePage(pdfDoc *gofpdf.Fpdf, importer *gofpdi.Importer, cache map[int]bookImportedTemplate, sourcePDF []byte, layout bookContentPageLayout, submission bookSubmissionRenderData) error {
	template, err := importTemplatePage(pdfDoc, importer, cache, sourcePDF, layout.TemplatePageNumber)
	if err != nil {
		return err
	}

	pdfDoc.AddPageFormat(pageOrientation(template.Width, template.Height), gofpdf.SizeType{Wd: template.Width, Ht: template.Height})
	importer.UseImportedTemplate(pdfDoc, template.ID, 0, 0, template.Width, template.Height)

	eraseLayout := mergedContentEraseLayout(layout, scaledDefaultContentPageLayout(layout.TemplatePageNumber, template.Width, template.Height))
	drawMask(pdfDoc, eraseLayout.HeadingMask)
	drawMask(pdfDoc, eraseLayout.BodyMask)

	if submission.Image != nil {
		drawSubmissionImage(pdfDoc, layout.ImageArea, *submission.Image)
	}
	drawFittedTextBox(pdfDoc, layout.HeadingArea, submission.Heading)
	drawBodyBlocks(pdfDoc, layout.BodyArea, submission.Body)
	return pdfDoc.Error()
}

func appendSectionTemplatePage(pdfDoc *gofpdf.Fpdf, importer *gofpdi.Importer, cache map[int]bookImportedTemplate, sourcePDF []byte, layout bookSectionPageLayout, title string) error {
	template, err := importTemplatePage(pdfDoc, importer, cache, sourcePDF, layout.TemplatePageNumber)
	if err != nil {
		return err
	}

	pdfDoc.AddPageFormat(pageOrientation(template.Width, template.Height), gofpdf.SizeType{Wd: template.Width, Ht: template.Height})
	importer.UseImportedTemplate(pdfDoc, template.ID, 0, 0, template.Width, template.Height)

	eraseLayout := mergedSectionEraseLayout(layout, scaledDefaultSectionPageLayout(layout.TemplatePageNumber, template.Width, template.Height))
	drawMask(pdfDoc, eraseLayout.TitleMask)
	drawFittedTextBox(pdfDoc, layout.TitleArea, strings.TrimSpace(title))
	return pdfDoc.Error()
}

func importTemplatePage(pdfDoc *gofpdf.Fpdf, importer *gofpdi.Importer, cache map[int]bookImportedTemplate, sourcePDF []byte, pageNumber int) (bookImportedTemplate, error) {
	if template, ok := cache[pageNumber]; ok {
		return template, nil
	}
	if pageNumber <= 0 {
		return bookImportedTemplate{}, fmt.Errorf("template page number %d is invalid", pageNumber)
	}

	rs := io.ReadSeeker(bytes.NewReader(sourcePDF))
	tplID := importer.ImportPageFromStream(pdfDoc, &rs, pageNumber, "/MediaBox")
	pageSizes := importer.GetPageSizes()
	boxes, ok := pageSizes[pageNumber]
	if !ok {
		return bookImportedTemplate{}, fmt.Errorf("template page %d not found in imported page sizes", pageNumber)
	}

	size, ok := boxes["/MediaBox"]
	if !ok {
		for _, candidate := range []string{"/CropBox", "/TrimBox"} {
			if candidateSize, exists := boxes[candidate]; exists {
				size = candidateSize
				ok = true
				break
			}
		}
	}
	if !ok {
		return bookImportedTemplate{}, fmt.Errorf("template page %d does not expose a supported page box", pageNumber)
	}

	template := bookImportedTemplate{
		ID:     tplID,
		Width:  size["w"],
		Height: size["h"],
	}
	cache[pageNumber] = template
	return template, nil
}

func applyContentTemplateAnalysis(layout *bookContentPageLayout, analysis bookTemplateAnalysis) {
	layout.PageWidth = analysis.PageWidth
	layout.PageHeight = analysis.PageHeight

	if analysis.Title.Width > 0 {
		layout.HeadingMask.X = analysis.TitleMask.X
		layout.HeadingMask.Y = analysis.TitleMask.Y
		layout.HeadingMask.Width = analysis.TitleMask.Width
		layout.HeadingMask.Height = analysis.TitleMask.Height
		layout.HeadingArea.X = analysis.TitleArea.X
		layout.HeadingArea.Y = analysis.TitleArea.Y
		layout.HeadingArea.Width = analysis.TitleArea.Width
		layout.HeadingArea.Height = analysis.TitleArea.Height
		layout.HeadingArea.FontSize = analysis.TitleFontSize
		layout.HeadingArea.MinFontSize = math.Max(analysis.TitleFontSize*0.68, 11)
		layout.HeadingArea.TextAlign = analysis.TitleAlign
		layout.HeadingArea.FontFamily = analysis.TitleFontFamily
		layout.HeadingArea.FontStyle = analysis.TitleFontStyle
	}

	if analysis.Body.Width > 0 {
		layout.BodyMask.X = analysis.BodyMask.X
		layout.BodyMask.Y = analysis.BodyMask.Y
		layout.BodyMask.Width = analysis.BodyMask.Width
		layout.BodyMask.Height = analysis.BodyMask.Height
		layout.BodyArea.X = analysis.BodyArea.X
		layout.BodyArea.Y = analysis.BodyArea.Y
		layout.BodyArea.Width = analysis.BodyArea.Width
		layout.BodyArea.Height = analysis.BodyArea.Height
		layout.BodyArea.FontSize = analysis.BodyFontSize
		layout.BodyArea.MinFontSize = math.Max(analysis.BodyFontSize*0.72, 8)
		layout.BodyArea.LabelFontSize = analysis.BodyFontSize
		layout.BodyArea.LabelMinFontSize = math.Max(analysis.BodyFontSize*0.72, 8)
		layout.BodyArea.TextAlign = analysis.BodyAlign
		layout.BodyArea.FontFamily = analysis.BodyFontFamily
		layout.BodyArea.FontStyle = analysis.BodyFontStyle
		layout.BodyArea.LabelFontFamily = analysis.BodyFontFamily
		layout.BodyArea.LabelFontStyle = "B"
	}

	if analysis.ImageArea.Width > 0 {
		layout.ImageArea = analysis.ImageArea
	}
}

func applySectionTemplateAnalysis(layout *bookSectionPageLayout, analysis bookTemplateAnalysis) {
	layout.PageWidth = analysis.PageWidth
	layout.PageHeight = analysis.PageHeight
	if analysis.Title.Width <= 0 {
		return
	}

	layout.TitleMask.X = analysis.TitleMask.X
	layout.TitleMask.Y = analysis.TitleMask.Y
	layout.TitleMask.Width = analysis.TitleMask.Width
	layout.TitleMask.Height = analysis.TitleMask.Height
	layout.TitleArea.X = analysis.TitleArea.X
	layout.TitleArea.Y = analysis.TitleArea.Y
	layout.TitleArea.Width = analysis.TitleArea.Width
	layout.TitleArea.Height = analysis.TitleArea.Height
	layout.TitleArea.FontSize = analysis.TitleFontSize
	layout.TitleArea.MinFontSize = math.Max(analysis.TitleFontSize*0.68, 16)
	layout.TitleArea.TextAlign = analysis.TitleAlign
	layout.TitleArea.FontFamily = analysis.TitleFontFamily
	layout.TitleArea.FontStyle = analysis.TitleFontStyle
}

type bookTemplateAnalysis struct {
	PageWidth       float64
	PageHeight      float64
	Title           bookRect
	TitleMask       bookRect
	TitleArea       bookRect
	TitleFontSize   float64
	TitleFontFamily string
	TitleFontStyle  string
	TitleAlign      string
	Body            bookRect
	BodyMask        bookRect
	BodyArea        bookRect
	BodyFontSize    float64
	BodyFontFamily  string
	BodyFontStyle   string
	BodyAlign       string
	ImageArea       bookImageLayout
}

func analyzeTemplatePage(page rpdf.Page) (bookTemplateAnalysis, bool) {
	if page.V.IsNull() {
		return bookTemplateAnalysis{}, false
	}

	pageWidth, pageHeight := extractPageDimensions(page)
	if pageWidth <= 0 || pageHeight <= 0 {
		return bookTemplateAnalysis{}, false
	}

	lines := extractTemplateLines(page, pageHeight)
	if len(lines) == 0 {
		return bookTemplateAnalysis{
			PageWidth:  pageWidth,
			PageHeight: pageHeight,
		}, true
	}

	titleLines := selectTitleLines(lines, pageWidth, pageHeight)
	bodyLines := selectBodyLines(lines, titleLines)

	analysis := bookTemplateAnalysis{
		PageWidth:  pageWidth,
		PageHeight: pageHeight,
	}

	if len(titleLines) > 0 {
		titleBox := unionTextLines(titleLines)
		analysis.Title = titleBox
		analysis.TitleMask = clampRect(expandRect(titleBox, math.Max(titleBox.Height*0.8, 18), math.Max(titleBox.Height*0.45, 10)), pageWidth, pageHeight)
		analysis.TitleArea = clampRect(expandRect(titleBox, math.Max(titleBox.Height*0.25, 8), math.Max(titleBox.Height*0.2, 6)), pageWidth, pageHeight)
		analysis.TitleFontSize = dominantFontSize(titleLines)
		analysis.TitleFontFamily, analysis.TitleFontStyle = mapPDFFontToBuiltIn(dominantFontName(titleLines))
		analysis.TitleAlign = detectTextAlign(titleBox, pageWidth)
	}

	if len(bodyLines) > 0 {
		bodyBox := unionTextLines(bodyLines)
		analysis.Body = bodyBox
		analysis.BodyMask = clampRect(expandRect(bodyBox, math.Max(bodyBox.Width*0.04, 14), math.Max(bodyBox.Height*0.04, 10)), pageWidth, pageHeight)
		analysis.BodyArea = clampRect(expandRect(bodyBox, math.Max(bodyBox.Width*0.015, 6), math.Max(bodyBox.Height*0.015, 4)), pageWidth, pageHeight)
		analysis.BodyFontSize = dominantFontSize(bodyLines)
		analysis.BodyFontFamily, analysis.BodyFontStyle = mapPDFFontToBuiltIn(dominantFontName(bodyLines))
		analysis.BodyAlign = detectTextAlign(bodyBox, pageWidth)
	}

	analysis.ImageArea = inferImageArea(pageWidth, pageHeight, analysis.BodyMask)
	return analysis, true
}

func extractTemplateLines(page rpdf.Page, pageHeight float64) []bookTextLine {
	content := page.Content()
	if len(content.Text) == 0 {
		return nil
	}

	characters := append([]rpdf.Text(nil), content.Text...)
	sort.Sort(rpdf.TextVertical(characters))

	lineGroups := make([][]rpdf.Text, 0)
	for _, char := range characters {
		if strings.TrimSpace(char.S) == "" {
			continue
		}
		if len(lineGroups) == 0 {
			lineGroups = append(lineGroups, []rpdf.Text{char})
			continue
		}
		last := lineGroups[len(lineGroups)-1]
		lastY := last[0].Y
		threshold := math.Max(last[0].FontSize*0.4, 2.4)
		if math.Abs(char.Y-lastY) <= threshold {
			lineGroups[len(lineGroups)-1] = append(last, char)
			continue
		}
		lineGroups = append(lineGroups, []rpdf.Text{char})
	}

	lines := make([]bookTextLine, 0, len(lineGroups))
	for _, chars := range lineGroups {
		sort.Sort(rpdf.TextHorizontal(chars))
		line := buildTextLine(chars, pageHeight)
		if strings.TrimSpace(line.Text) == "" {
			continue
		}
		lines = append(lines, line)
	}

	sort.Slice(lines, func(i, j int) bool {
		if almostEqual(lines[i].Y, lines[j].Y) {
			return lines[i].X < lines[j].X
		}
		return lines[i].Y < lines[j].Y
	})
	return lines
}

func buildTextLine(chars []rpdf.Text, pageHeight float64) bookTextLine {
	if len(chars) == 0 {
		return bookTextLine{}
	}

	var builder strings.Builder
	prevEnd := chars[0].X
	xMin := chars[0].X
	xMax := chars[0].X + chars[0].W
	maxFont := chars[0].FontSize
	fontCounts := map[string]int{}

	for idx, char := range chars {
		if idx > 0 {
			spaceThreshold := math.Max(char.FontSize*0.25, 2.5)
			if char.X-prevEnd > spaceThreshold {
				builder.WriteByte(' ')
			}
		}
		builder.WriteString(char.S)
		prevEnd = char.X + char.W
		xMax = math.Max(xMax, prevEnd)
		maxFont = math.Max(maxFont, char.FontSize)
		fontCounts[char.Font]++
	}

	fontName := ""
	fontCount := 0
	for name, count := range fontCounts {
		if count > fontCount {
			fontName = name
			fontCount = count
		}
	}

	topY := pageHeight - chars[0].Y - maxFont
	if topY < 0 {
		topY = 0
	}

	return bookTextLine{
		Text:     strings.TrimSpace(builder.String()),
		X:        xMin,
		Y:        topY,
		Width:    math.Max(xMax-xMin, maxFont),
		Height:   maxFont * 1.15,
		FontSize: maxFont,
		FontName: fontName,
	}
}

func selectTitleLines(lines []bookTextLine, pageWidth, pageHeight float64) []bookTextLine {
	if len(lines) == 0 {
		return nil
	}

	bestIndex := -1
	bestScore := -1.0
	for idx, line := range lines {
		if strings.TrimSpace(line.Text) == "" {
			continue
		}
		topPenalty := 1.0
		if line.Y <= pageHeight*0.45 {
			topPenalty = 0
		}
		score := line.FontSize*10 - topPenalty*line.FontSize*2
		if score > bestScore {
			bestScore = score
			bestIndex = idx
		}
	}
	if bestIndex < 0 {
		return nil
	}

	selected := []bookTextLine{lines[bestIndex]}
	base := lines[bestIndex]
	for idx := bestIndex - 1; idx >= 0; idx-- {
		line := lines[idx]
		if math.Abs(line.FontSize-base.FontSize) > base.FontSize*0.28 {
			break
		}
		if math.Abs(line.Y-base.Y) > base.Height*1.6 {
			break
		}
		selected = append([]bookTextLine{line}, selected...)
		base = line
	}

	base = lines[bestIndex]
	for idx := bestIndex + 1; idx < len(lines); idx++ {
		line := lines[idx]
		if math.Abs(line.FontSize-base.FontSize) > base.FontSize*0.28 {
			break
		}
		if math.Abs(line.Y-base.Y) > base.Height*1.6 {
			break
		}
		selected = append(selected, line)
		base = line
	}

	return selected
}

func selectBodyLines(lines []bookTextLine, titleLines []bookTextLine) []bookTextLine {
	if len(lines) == 0 {
		return nil
	}
	if len(titleLines) == 0 {
		return append([]bookTextLine(nil), lines...)
	}

	titleBounds := unionTextLines(titleLines)
	bodyLines := make([]bookTextLine, 0, len(lines))
	for _, line := range lines {
		if line.Y+line.Height <= titleBounds.Y {
			bodyLines = append(bodyLines, line)
			continue
		}
		if line.Y >= titleBounds.Y+titleBounds.Height {
			bodyLines = append(bodyLines, line)
		}
	}
	return bodyLines
}

func unionTextLines(lines []bookTextLine) bookRect {
	if len(lines) == 0 {
		return bookRect{}
	}

	xMin := lines[0].X
	yMin := lines[0].Y
	xMax := lines[0].X + lines[0].Width
	yMax := lines[0].Y + lines[0].Height

	for _, line := range lines[1:] {
		xMin = math.Min(xMin, line.X)
		yMin = math.Min(yMin, line.Y)
		xMax = math.Max(xMax, line.X+line.Width)
		yMax = math.Max(yMax, line.Y+line.Height)
	}

	return bookRect{
		X:      xMin,
		Y:      yMin,
		Width:  xMax - xMin,
		Height: yMax - yMin,
	}
}

func dominantFontName(lines []bookTextLine) string {
	counts := make(map[string]int)
	bestName := ""
	bestCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line.FontName) == "" {
			continue
		}
		counts[line.FontName]++
		if counts[line.FontName] > bestCount {
			bestName = line.FontName
			bestCount = counts[line.FontName]
		}
	}
	return bestName
}

func dominantFontSize(lines []bookTextLine) float64 {
	if len(lines) == 0 {
		return 0
	}
	values := make([]float64, 0, len(lines))
	for _, line := range lines {
		if line.FontSize > 0 {
			values = append(values, line.FontSize)
		}
	}
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	return values[len(values)/2]
}

func detectTextAlign(rect bookRect, pageWidth float64) string {
	center := rect.X + (rect.Width / 2)
	if math.Abs(center-(pageWidth/2)) <= pageWidth*0.12 {
		return "C"
	}
	if rect.X+rect.Width >= pageWidth*0.85 {
		return "R"
	}
	return "L"
}

func inferImageArea(pageWidth, pageHeight float64, bodyMask bookRect) bookImageLayout {
	width := math.Max(pageWidth*0.18, 96)
	height := math.Max(pageHeight*0.16, 96)
	x := pageWidth - width - math.Max(pageWidth*0.08, 40)
	y := pageHeight - height - math.Max(pageHeight*0.1, 48)

	if bodyMask.Width > 0 && x < bodyMask.X+bodyMask.Width {
		x = pageWidth - width - math.Max(pageWidth*0.05, 26)
	}
	if x < pageWidth*0.55 {
		x = pageWidth * 0.62
	}
	if y < pageHeight*0.6 {
		y = pageHeight * 0.68
	}

	return bookImageLayout{
		X:      x,
		Y:      y,
		Width:  width,
		Height: height,
		Alpha:  0.88,
	}
}

func extractPageDimensions(page rpdf.Page) (float64, float64) {
	box := findInheritedValue(page.V, "CropBox")
	if box.IsNull() {
		box = findInheritedValue(page.V, "MediaBox")
	}
	if box.IsNull() || box.Len() < 4 {
		return 0, 0
	}

	x1 := box.Index(0).Float64()
	y1 := box.Index(1).Float64()
	x2 := box.Index(2).Float64()
	y2 := box.Index(3).Float64()
	return math.Abs(x2 - x1), math.Abs(y2 - y1)
}

func findInheritedValue(value rpdf.Value, key string) rpdf.Value {
	for current := value; !current.IsNull(); current = current.Key("Parent") {
		candidate := current.Key(key)
		if !candidate.IsNull() {
			return candidate
		}
	}
	return rpdf.Value{}
}

func groupSubmissionRenderData(fields []BookVersionField, submissions []BookSubmission, valuesBySubmission map[int][]BookSubmissionValue, imagesBySubmission map[int]*storedBookUploadContent) map[int][]bookSubmissionRenderData {
	fieldsByID := make(map[int]BookVersionField, len(fields))
	for _, field := range fields {
		fieldsByID[field.ID] = field
	}

	grouped := make(map[int][]bookSubmissionRenderData)
	for _, submission := range submissions {
		if submission.TargetSectionID == nil {
			continue
		}
		renderData := bookSubmissionRenderData{
			SectionID: *submission.TargetSectionID,
			Image:     imagesBySubmission[submission.ID],
		}
		renderData.Heading, renderData.Body = buildSubmissionPageContent(fields, valuesBySubmission[submission.ID], fieldsByID)
		grouped[renderData.SectionID] = append(grouped[renderData.SectionID], renderData)
	}
	return grouped
}

func buildSubmissionPageContent(fields []BookVersionField, values []BookSubmissionValue, fieldsByID map[int]BookVersionField) (string, []bookBodyBlock) {
	valueMap := make(map[int]string, len(values))
	for _, value := range values {
		valueMap[value.BookFieldID] = strings.TrimSpace(value.Value)
	}

	headingParts := make([]string, 0)
	bodyBlocks := make([]bookBodyBlock, 0)

	for _, field := range fields {
		value := strings.TrimSpace(valueMap[field.ID])
		if value == "" {
			continue
		}
		switch field.Placement {
		case BookFieldPlacementHeading:
			headingParts = append(headingParts, sanitizeRichTextToPlainText(value))
		case BookFieldPlacementBody:
			bodyValue := sanitizeRichTextToPlainText(value)
			if bodyValue == "" {
				continue
			}
			block := bookBodyBlock{
				Value: bodyValue,
			}
			if field.ShowLabel {
				block.Label = strings.TrimSpace(field.Label) + ":"
			}
			bodyBlocks = append(bodyBlocks, block)
		}
	}

	return strings.TrimSpace(strings.Join(headingParts, " ")), bodyBlocks
}

func sanitizeRichTextToPlainText(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, "<") {
		return normalizePlainText(htmlstd.UnescapeString(trimmed))
	}

	tokenizer := html.NewTokenizer(strings.NewReader(trimmed))
	var builder strings.Builder
	wroteText := false

	writeLineBreak := func(force bool) {
		if builder.Len() == 0 {
			return
		}
		current := builder.String()
		if strings.HasSuffix(current, "\n") {
			if force && !strings.HasSuffix(current, "\n\n") {
				builder.WriteByte('\n')
			}
			return
		}
		builder.WriteByte('\n')
		if force {
			builder.WriteByte('\n')
		}
	}

	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return normalizePlainText(builder.String())
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			switch strings.ToLower(token.Data) {
			case "br":
				writeLineBreak(false)
			case "p", "div":
				if wroteText {
					writeLineBreak(true)
				}
			case "li":
				if wroteText {
					writeLineBreak(false)
				}
				builder.WriteString("- ")
			}
		case html.EndTagToken:
			token := tokenizer.Token()
			switch strings.ToLower(token.Data) {
			case "p", "div":
				if wroteText {
					writeLineBreak(true)
				}
			case "li":
				if wroteText {
					writeLineBreak(false)
				}
			}
		case html.TextToken:
			text := normalizeInlineText(htmlstd.UnescapeString(string(tokenizer.Text())))
			if text == "" {
				continue
			}
			builder.WriteString(text)
			wroteText = true
		}
	}
}

func normalizeInlineText(value string) string {
	parts := strings.Fields(value)
	return strings.Join(parts, " ")
}

func normalizePlainText(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	normalizedLines := make([]string, 0, len(lines))
	blankPending := false
	for _, line := range lines {
		clean := strings.TrimSpace(line)
		if clean == "" {
			if len(normalizedLines) == 0 {
				continue
			}
			blankPending = true
			continue
		}
		if blankPending {
			normalizedLines = append(normalizedLines, "")
			blankPending = false
		}
		normalizedLines = append(normalizedLines, clean)
	}
	return strings.TrimSpace(strings.Join(normalizedLines, "\n"))
}

func drawMask(pdfDoc *gofpdf.Fpdf, mask bookMaskLayout) {
	if mask.Width <= 0 || mask.Height <= 0 {
		return
	}
	r, g, b := parseHexColor(mask.BackgroundColor)
	pdfDoc.SetAlpha(mask.Alpha, "Normal")
	pdfDoc.SetFillColor(r, g, b)
	pdfDoc.Rect(mask.X, mask.Y, mask.Width, mask.Height, "F")
	pdfDoc.SetAlpha(1, "Normal")
}

func drawSubmissionImage(pdfDoc *gofpdf.Fpdf, layout bookImageLayout, image storedBookUploadContent) {
	if layout.Width <= 0 || layout.Height <= 0 || len(image.Data) == 0 {
		return
	}

	imageType := detectImageTypeFromMime(image.MimeType)
	if imageType == "" {
		imageType = detectImageTypeFromName(image.FileName)
	}
	if imageType == "" {
		return
	}

	imgName := fmt.Sprintf("submission-image-%d-%d", len(image.Data), len(image.FileName))
	pdfDoc.RegisterImageOptionsReader(imgName, gofpdf.ImageOptions{
		ImageType: imageType,
		ReadDpi:   true,
	}, bytes.NewReader(image.Data))
	info := pdfDoc.GetImageInfo(imgName)
	if info == nil {
		return
	}

	drawWidth, drawHeight := fitImageIntoBox(info.Width(), info.Height(), layout.Width, layout.Height)
	x := layout.X + ((layout.Width - drawWidth) / 2)
	y := layout.Y + ((layout.Height - drawHeight) / 2)
	pdfDoc.SetAlpha(layout.Alpha, "Normal")
	pdfDoc.ImageOptions(imgName, x, y, drawWidth, drawHeight, false, gofpdf.ImageOptions{
		ImageType: imageType,
		ReadDpi:   true,
	}, 0, "")
	pdfDoc.SetAlpha(1, "Normal")
}

func fitImageIntoBox(srcWidth, srcHeight, maxWidth, maxHeight float64) (float64, float64) {
	if srcWidth <= 0 || srcHeight <= 0 {
		return maxWidth, maxHeight
	}
	scale := math.Min(maxWidth/srcWidth, maxHeight/srcHeight)
	if scale <= 0 {
		scale = 1
	}
	return srcWidth * scale, srcHeight * scale
}

func detectImageTypeFromName(fileName string) string {
	lower := strings.ToLower(strings.TrimSpace(fileName))
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "jpg"
	case strings.HasSuffix(lower, ".png"):
		return "png"
	case strings.HasSuffix(lower, ".gif"):
		return "gif"
	default:
		return ""
	}
}

func detectImageTypeFromMime(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0])) {
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	default:
		return ""
	}
}

func drawFittedTextBox(pdfDoc *gofpdf.Fpdf, layout bookTextLayout, text string) {
	text = strings.TrimSpace(text)
	if text == "" || layout.Width <= 0 || layout.Height <= 0 {
		return
	}

	family, style := normalizeFontSelection(layout.FontFamily, layout.FontStyle)
	fontSize := fitTextFontSize(pdfDoc, layout, text, family, style)
	lineHeight := fontSize * layout.LineHeight

	pdfDoc.SetFont(family, style, fontSize)
	r, g, b := parseHexColor(layout.TextColor)
	pdfDoc.SetTextColor(r, g, b)
	lines := pdfDoc.SplitText(text, layout.Width)
	if len(lines) == 0 {
		lines = []string{text}
	}

	contentHeight := float64(len(lines)) * lineHeight
	startY := layout.Y + math.Max((layout.Height-contentHeight)/2, 0)
	pdfDoc.SetXY(layout.X, startY)
	pdfDoc.MultiCell(layout.Width, lineHeight, text, "", normalizePDFTextAlign(layout.TextAlign, "L"), false)
}

func fitTextFontSize(pdfDoc *gofpdf.Fpdf, layout bookTextLayout, text string, family string, style string) float64 {
	fontSize := layout.FontSize
	minFontSize := layout.MinFontSize
	if minFontSize <= 0 {
		minFontSize = math.Max(fontSize*0.7, 8)
	}
	for fontSize >= minFontSize {
		pdfDoc.SetFont(family, style, fontSize)
		lines := pdfDoc.SplitText(text, layout.Width)
		if len(lines) == 0 {
			lines = []string{text}
		}
		if float64(len(lines))*fontSize*layout.LineHeight <= layout.Height {
			return fontSize
		}
		fontSize -= 0.5
	}
	return minFontSize
}

func drawBodyBlocks(pdfDoc *gofpdf.Fpdf, layout bookBodyLayout, blocks []bookBodyBlock) {
	if layout.Width <= 0 || layout.Height <= 0 || len(blocks) == 0 {
		return
	}

	bodyFamily, bodyStyle := normalizeFontSelection(layout.FontFamily, layout.FontStyle)
	labelFamily, labelStyle := normalizeFontSelection(layout.LabelFontFamily, layout.LabelFontStyle)
	bodyFontSize, labelFontSize := fitBodyFontSizes(pdfDoc, layout, blocks, bodyFamily, bodyStyle, labelFamily, labelStyle)
	bodyLineHeight := bodyFontSize * layout.LineHeight
	labelLineHeight := labelFontSize * layout.LineHeight

	y := layout.Y
	for idx, block := range blocks {
		if strings.TrimSpace(block.Label) != "" {
			pdfDoc.SetFont(labelFamily, labelStyle, labelFontSize)
			r, g, b := parseHexColor(layout.LabelTextColor)
			pdfDoc.SetTextColor(r, g, b)
			pdfDoc.SetXY(layout.X, y)
			pdfDoc.MultiCell(layout.Width, labelLineHeight, block.Label, "", "L", false)
			y = pdfDoc.GetY()
		}

		pdfDoc.SetFont(bodyFamily, bodyStyle, bodyFontSize)
		r, g, b := parseHexColor(layout.TextColor)
		pdfDoc.SetTextColor(r, g, b)
		pdfDoc.SetXY(layout.X, y)
		pdfDoc.MultiCell(layout.Width, bodyLineHeight, block.Value, "", normalizePDFTextAlign(layout.TextAlign, "L"), false)
		y = pdfDoc.GetY()

		if idx < len(blocks)-1 {
			y += layout.ParagraphSpacing
		}
	}
}

func fitBodyFontSizes(pdfDoc *gofpdf.Fpdf, layout bookBodyLayout, blocks []bookBodyBlock, bodyFamily string, bodyStyle string, labelFamily string, labelStyle string) (float64, float64) {
	bodySize := layout.FontSize
	labelSize := layout.LabelFontSize
	minBodySize := layout.MinFontSize
	minLabelSize := layout.LabelMinFontSize
	if minBodySize <= 0 {
		minBodySize = math.Max(bodySize*0.72, 8)
	}
	if minLabelSize <= 0 {
		minLabelSize = math.Max(labelSize*0.72, 8)
	}

	for bodySize >= minBodySize && labelSize >= minLabelSize {
		totalHeight := measureBodyBlocksHeight(pdfDoc, layout, blocks, bodyFamily, bodyStyle, labelFamily, labelStyle, bodySize, labelSize)
		if totalHeight <= layout.Height {
			return bodySize, labelSize
		}
		bodySize -= 0.5
		labelSize -= 0.5
	}
	return minBodySize, minLabelSize
}

func measureBodyBlocksHeight(pdfDoc *gofpdf.Fpdf, layout bookBodyLayout, blocks []bookBodyBlock, bodyFamily string, bodyStyle string, labelFamily string, labelStyle string, bodySize float64, labelSize float64) float64 {
	bodyLineHeight := bodySize * layout.LineHeight
	labelLineHeight := labelSize * layout.LineHeight
	total := 0.0

	for idx, block := range blocks {
		if strings.TrimSpace(block.Label) != "" {
			pdfDoc.SetFont(labelFamily, labelStyle, labelSize)
			labelLines := pdfDoc.SplitText(block.Label, layout.Width)
			if len(labelLines) == 0 {
				labelLines = []string{block.Label}
			}
			total += float64(len(labelLines)) * labelLineHeight
		}

		pdfDoc.SetFont(bodyFamily, bodyStyle, bodySize)
		valueLines := pdfDoc.SplitText(block.Value, layout.Width)
		if len(valueLines) == 0 {
			valueLines = []string{block.Value}
		}
		total += float64(len(valueLines)) * bodyLineHeight
		if idx < len(blocks)-1 {
			total += layout.ParagraphSpacing
		}
	}
	return total
}

func normalizeFontSelection(family string, style string) (string, string) {
	family = strings.TrimSpace(family)
	style = strings.ToUpper(strings.TrimSpace(style))
	if family == "" {
		family = "Helvetica"
	}
	switch strings.ToLower(family) {
	case "arial":
		family = "Helvetica"
	case "helvetica", "times", "courier":
	default:
		if strings.Contains(strings.ToLower(family), "times") {
			family = "Times"
		} else if strings.Contains(strings.ToLower(family), "courier") {
			family = "Courier"
		} else {
			family = "Helvetica"
		}
	}
	return family, style
}

func mapPDFFontToBuiltIn(fontName string) (string, string) {
	lower := strings.ToLower(strings.TrimSpace(fontName))
	style := ""
	if strings.Contains(lower, "bold") {
		style += "B"
	}
	if strings.Contains(lower, "italic") || strings.Contains(lower, "oblique") {
		style += "I"
	}

	switch {
	case strings.Contains(lower, "times"):
		return "Times", style
	case strings.Contains(lower, "courier"):
		return "Courier", style
	default:
		return "Helvetica", style
	}
}

func parseHexColor(value string) (int, int, int) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "#"))
	if len(trimmed) != 6 {
		return 0, 0, 0
	}

	parsePart := func(part string) int {
		v, err := strconv.ParseInt(part, 16, 32)
		if err != nil {
			return 0
		}
		return int(v)
	}

	return parsePart(trimmed[0:2]), parsePart(trimmed[2:4]), parsePart(trimmed[4:6])
}

func expandRect(rect bookRect, padX, padY float64) bookRect {
	return bookRect{
		X:      rect.X - padX,
		Y:      rect.Y - padY,
		Width:  rect.Width + (padX * 2),
		Height: rect.Height + (padY * 2),
	}
}

func clampRect(rect bookRect, pageWidth, pageHeight float64) bookRect {
	if rect.X < 0 {
		rect.Width += rect.X
		rect.X = 0
	}
	if rect.Y < 0 {
		rect.Height += rect.Y
		rect.Y = 0
	}
	if rect.X+rect.Width > pageWidth {
		rect.Width = pageWidth - rect.X
	}
	if rect.Y+rect.Height > pageHeight {
		rect.Height = pageHeight - rect.Y
	}
	if rect.Width < 0 {
		rect.Width = 0
	}
	if rect.Height < 0 {
		rect.Height = 0
	}
	return rect
}

func pageOrientation(width, height float64) string {
	if width > height {
		return "L"
	}
	return "P"
}

func normalizePDFTextAlign(value string, fallback string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "L", "C", "R", "J":
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		return strings.ToUpper(strings.TrimSpace(fallback))
	}
}

func choosePositiveFloat(value float64, fallback float64) float64 {
	if value > 0 {
		return value
	}
	return fallback
}

func almostEqual(left, right float64) bool {
	return math.Abs(left-right) < 0.01
}
