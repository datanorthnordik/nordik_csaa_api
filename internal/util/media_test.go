package util

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestPrepareUploadForStorageLeavesNonImageDataUntouched(t *testing.T) {
	input := []byte("plain text file")

	prepared := PrepareUploadForStorage(input, "text/plain")

	if prepared.Optimized {
		t.Fatal("expected text payloads to be left unchanged")
	}
	if prepared.ContentType != "text/plain" {
		t.Fatalf("expected content type text/plain, got %q", prepared.ContentType)
	}
	if string(prepared.Data) != string(input) {
		t.Fatalf("expected text payload to remain unchanged, got %q", string(prepared.Data))
	}
}

func TestPrepareUploadForStorageOptimizesLargePNG(t *testing.T) {
	original := buildEncodedPNG(t, 3200, 1800)

	prepared := PrepareUploadForStorage(original, "image/png")

	if !prepared.Optimized {
		t.Fatal("expected large png upload to be optimized")
	}
	if prepared.ContentType != "image/png" {
		t.Fatalf("expected image/png content type, got %q", prepared.ContentType)
	}
	if len(prepared.Data) >= len(original) {
		t.Fatalf("expected optimized png to be smaller, original=%d optimized=%d", len(original), len(prepared.Data))
	}

	img, err := png.Decode(bytes.NewReader(prepared.Data))
	if err != nil {
		t.Fatalf("decode optimized png: %v", err)
	}
	if got := img.Bounds().Dx(); got != defaultMaxOptimizedImageWidth {
		t.Fatalf("expected optimized png width %d, got %d", defaultMaxOptimizedImageWidth, got)
	}
	if got := img.Bounds().Dy(); got != 1440 {
		t.Fatalf("expected optimized png height 1440, got %d", got)
	}
}

func TestPrepareUploadForStorageOptimizesLargeJPEG(t *testing.T) {
	original := buildEncodedJPEG(t, 3200, 1800, 96)

	prepared := PrepareUploadForStorage(original, "image/jpeg")

	if !prepared.Optimized {
		t.Fatal("expected large jpeg upload to be optimized")
	}
	if prepared.ContentType != "image/jpeg" {
		t.Fatalf("expected image/jpeg content type, got %q", prepared.ContentType)
	}
	if len(prepared.Data) >= len(original) {
		t.Fatalf("expected optimized jpeg to be smaller, original=%d optimized=%d", len(original), len(prepared.Data))
	}

	img, err := jpeg.Decode(bytes.NewReader(prepared.Data))
	if err != nil {
		t.Fatalf("decode optimized jpeg: %v", err)
	}
	if got := img.Bounds().Dx(); got != defaultMaxOptimizedImageWidth {
		t.Fatalf("expected optimized jpeg width %d, got %d", defaultMaxOptimizedImageWidth, got)
	}
	if got := img.Bounds().Dy(); got != 1440 {
		t.Fatalf("expected optimized jpeg height 1440, got %d", got)
	}
}

func TestApplyImageOrientationRotate90Clockwise(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	src.SetNRGBA(1, 0, color.NRGBA{B: 255, A: 255})

	rotated := applyImageOrientation(src, 6)
	nrgba := toNRGBA(rotated)

	if nrgba.Bounds().Dx() != 1 || nrgba.Bounds().Dy() != 2 {
		t.Fatalf("expected rotated image bounds 1x2, got %dx%d", nrgba.Bounds().Dx(), nrgba.Bounds().Dy())
	}
	if got := nrgba.NRGBAAt(0, 0); got.R != 255 || got.A != 255 {
		t.Fatalf("expected red pixel at top after rotation, got %#v", got)
	}
	if got := nrgba.NRGBAAt(0, 1); got.B != 255 || got.A != 255 {
		t.Fatalf("expected blue pixel at bottom after rotation, got %#v", got)
	}
}

func buildEncodedPNG(t *testing.T, width int, height int) []byte {
	t.Helper()

	img := buildGradientImage(width, height)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func buildEncodedJPEG(t *testing.T, width int, height int, quality int) []byte {
	t.Helper()

	img := buildGradientImage(width, height)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func buildGradientImage(width int, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8((x * 255) / width),
				G: uint8((y * 255) / height),
				B: uint8(((x + y) * 255) / (width + height)),
				A: 255,
			})
		}
	}
	return img
}
