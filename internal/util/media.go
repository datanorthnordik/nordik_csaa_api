package util

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"net/http"
	"strings"
)

const (
	defaultMaxOptimizedImageWidth  = 2560
	defaultMaxOptimizedImageHeight = 2560
	defaultMinImageOptimizeBytes   = 256 * 1024
	defaultOptimizedJPEGQuality    = 82
)

type PreparedUpload struct {
	Data        []byte
	ContentType string
	Optimized   bool
}

type MediaUploadPolicy struct {
	MaxImageWidth         int
	MaxImageHeight        int
	MinImageOptimizeBytes int
	OptimizedJPEGQuality  int
}

var DefaultMediaUploadPolicy = MediaUploadPolicy{
	MaxImageWidth:         defaultMaxOptimizedImageWidth,
	MaxImageHeight:        defaultMaxOptimizedImageHeight,
	MinImageOptimizeBytes: defaultMinImageOptimizeBytes,
	OptimizedJPEGQuality:  defaultOptimizedJPEGQuality,
}

func PrepareUploadForStorage(data []byte, contentType string) PreparedUpload {
	return PrepareUploadForStorageWithPolicy(data, contentType, DefaultMediaUploadPolicy)
}

func PrepareUploadForStorageWithPolicy(data []byte, contentType string, policy MediaUploadPolicy) PreparedUpload {
	policy = normalizeMediaUploadPolicy(policy)
	contentType = normalizePreparedContentType(contentType, data)

	prepared := PreparedUpload{
		Data:        data,
		ContentType: contentType,
	}
	if len(data) == 0 {
		return prepared
	}

	optimizedData, optimized := optimizeRasterUpload(data, contentType, policy)
	if optimized {
		prepared.Data = optimizedData
		prepared.Optimized = true
	}

	return prepared
}

func normalizeMediaUploadPolicy(policy MediaUploadPolicy) MediaUploadPolicy {
	if policy.MaxImageWidth <= 0 {
		policy.MaxImageWidth = defaultMaxOptimizedImageWidth
	}
	if policy.MaxImageHeight <= 0 {
		policy.MaxImageHeight = defaultMaxOptimizedImageHeight
	}
	if policy.MinImageOptimizeBytes <= 0 {
		policy.MinImageOptimizeBytes = defaultMinImageOptimizeBytes
	}
	if policy.OptimizedJPEGQuality < 1 || policy.OptimizedJPEGQuality > 100 {
		policy.OptimizedJPEGQuality = defaultOptimizedJPEGQuality
	}
	return policy
}

func normalizePreparedContentType(contentType string, data []byte) string {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" || strings.EqualFold(contentType, "application/octet-stream") {
		contentType = http.DetectContentType(data)
	}
	return contentType
}

func optimizeRasterUpload(data []byte, contentType string, policy MediaUploadPolicy) ([]byte, bool) {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if !isOptimizableRasterType(contentType) {
		return nil, false
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}

	if contentType == "image/jpeg" || contentType == "image/jpg" {
		orientation, ok := parseJPEGOrientation(data)
		if !ok {
			return nil, false
		}
		if orientation != 1 {
			src = applyImageOrientation(src, orientation)
		}
	}

	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, false
	}

	resizeNeeded := width > policy.MaxImageWidth || height > policy.MaxImageHeight
	if !resizeNeeded && len(data) < policy.MinImageOptimizeBytes {
		return nil, false
	}

	targetWidth, targetHeight := fitWithinBounds(width, height, policy.MaxImageWidth, policy.MaxImageHeight)
	working := src
	if resizeNeeded {
		working = resizeImageBilinear(src, targetWidth, targetHeight)
	}

	encoded, err := encodeOptimizedRaster(working, contentType, policy)
	if err != nil || len(encoded) == 0 || len(encoded) >= len(data) {
		return nil, false
	}

	return encoded, true
}

func isOptimizableRasterType(contentType string) bool {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/jpg", "image/png":
		return true
	default:
		return false
	}
}

func encodeOptimizedRaster(img image.Image, contentType string, policy MediaUploadPolicy) ([]byte, error) {
	var buf bytes.Buffer

	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/jpg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: policy.OptimizedJPEGQuality}); err != nil {
			return nil, err
		}
	case "image/png":
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		if err := encoder.Encode(&buf, img); err != nil {
			return nil, err
		}
	default:
		return nil, nil
	}

	return buf.Bytes(), nil
}

func fitWithinBounds(width int, height int, maxWidth int, maxHeight int) (int, int) {
	if width <= 0 || height <= 0 {
		return width, height
	}
	if maxWidth <= 0 || maxHeight <= 0 {
		return width, height
	}
	if width <= maxWidth && height <= maxHeight {
		return width, height
	}

	widthRatio := float64(maxWidth) / float64(width)
	heightRatio := float64(maxHeight) / float64(height)
	scale := math.Min(widthRatio, heightRatio)
	if scale >= 1 {
		return width, height
	}

	targetWidth := int(math.Round(float64(width) * scale))
	targetHeight := int(math.Round(float64(height) * scale))
	if targetWidth < 1 {
		targetWidth = 1
	}
	if targetHeight < 1 {
		targetHeight = 1
	}
	return targetWidth, targetHeight
}

// Resize with bilinear sampling so shared upload optimization can stay dependency-free.
func resizeImageBilinear(src image.Image, targetWidth int, targetHeight int) image.Image {
	if targetWidth <= 0 || targetHeight <= 0 {
		return src
	}

	source := toNRGBA(src)
	sourceBounds := source.Bounds()
	sourceWidth := sourceBounds.Dx()
	sourceHeight := sourceBounds.Dy()
	if sourceWidth == targetWidth && sourceHeight == targetHeight {
		return source
	}

	dst := image.NewNRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	scaleX := float64(sourceWidth) / float64(targetWidth)
	scaleY := float64(sourceHeight) / float64(targetHeight)

	for y := 0; y < targetHeight; y++ {
		sourceY := (float64(y)+0.5)*scaleY - 0.5
		y0 := clampInt(int(math.Floor(sourceY)), 0, sourceHeight-1)
		y1 := clampInt(y0+1, 0, sourceHeight-1)
		weightY := clampFloat64(sourceY-float64(y0), 0, 1)

		for x := 0; x < targetWidth; x++ {
			sourceX := (float64(x)+0.5)*scaleX - 0.5
			x0 := clampInt(int(math.Floor(sourceX)), 0, sourceWidth-1)
			x1 := clampInt(x0+1, 0, sourceWidth-1)
			weightX := clampFloat64(sourceX-float64(x0), 0, 1)

			topLeft := source.NRGBAAt(x0, y0)
			topRight := source.NRGBAAt(x1, y0)
			bottomLeft := source.NRGBAAt(x0, y1)
			bottomRight := source.NRGBAAt(x1, y1)

			dst.SetNRGBA(x, y, color.NRGBA{
				R: interpolateBilinear(topLeft.R, topRight.R, bottomLeft.R, bottomRight.R, weightX, weightY),
				G: interpolateBilinear(topLeft.G, topRight.G, bottomLeft.G, bottomRight.G, weightX, weightY),
				B: interpolateBilinear(topLeft.B, topRight.B, bottomLeft.B, bottomRight.B, weightX, weightY),
				A: interpolateBilinear(topLeft.A, topRight.A, bottomLeft.A, bottomRight.A, weightX, weightY),
			})
		}
	}

	return dst
}

func toNRGBA(src image.Image) *image.NRGBA {
	if current, ok := src.(*image.NRGBA); ok && current.Bounds().Min.X == 0 && current.Bounds().Min.Y == 0 {
		return current
	}

	bounds := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			dst.SetNRGBA(x, y, color.NRGBAModel.Convert(src.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA))
		}
	}
	return dst
}

func interpolateBilinear(topLeft uint8, topRight uint8, bottomLeft uint8, bottomRight uint8, weightX float64, weightY float64) uint8 {
	top := interpolateLinear(topLeft, topRight, weightX)
	bottom := interpolateLinear(bottomLeft, bottomRight, weightX)
	return interpolateLinear(top, bottom, weightY)
}

func interpolateLinear(left uint8, right uint8, weight float64) uint8 {
	value := float64(left)*(1-weight) + float64(right)*weight
	return uint8(math.Round(clampFloat64(value, 0, 255)))
}

func clampInt(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func clampFloat64(value float64, min float64, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func parseJPEGOrientation(data []byte) (int, bool) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0, false
	}

	for i := 2; i < len(data); {
		if data[i] != 0xFF {
			i++
			continue
		}
		for i < len(data) && data[i] == 0xFF {
			i++
		}
		if i >= len(data) {
			break
		}

		marker := data[i]
		i++

		switch {
		case marker == 0xD9 || marker == 0xDA:
			return 1, true
		case marker == 0xD8 || marker == 0x01:
			continue
		case marker >= 0xD0 && marker <= 0xD7:
			continue
		}

		if i+2 > len(data) {
			return 0, false
		}

		segmentLength := int(binary.BigEndian.Uint16(data[i : i+2]))
		if segmentLength < 2 || i+segmentLength > len(data) {
			return 0, false
		}

		if marker == 0xE1 {
			segment := data[i+2 : i+segmentLength]
			if len(segment) >= 6 && bytes.Equal(segment[:6], []byte("Exif\x00\x00")) {
				return parseTIFFOrientation(segment[6:])
			}
		}

		i += segmentLength
	}

	return 1, true
}

func parseTIFFOrientation(data []byte) (int, bool) {
	if len(data) < 8 {
		return 0, false
	}

	var (
		byteOrder binary.ByteOrder
	)
	switch string(data[:2]) {
	case "II":
		byteOrder = binary.LittleEndian
	case "MM":
		byteOrder = binary.BigEndian
	default:
		return 0, false
	}

	readUint16 := func(offset int) (uint16, bool) {
		if offset < 0 || offset+2 > len(data) {
			return 0, false
		}
		return byteOrder.Uint16(data[offset : offset+2]), true
	}

	readUint32 := func(offset int) (uint32, bool) {
		if offset < 0 || offset+4 > len(data) {
			return 0, false
		}
		return byteOrder.Uint32(data[offset : offset+4]), true
	}

	tagMark, ok := readUint16(2)
	if !ok || tagMark != 0x002A {
		return 0, false
	}

	ifdOffset, ok := readUint32(4)
	if !ok || ifdOffset < 8 {
		return 0, false
	}

	entryCountOffset := int(ifdOffset)
	entryCount, ok := readUint16(entryCountOffset)
	if !ok {
		return 0, false
	}

	for idx := 0; idx < int(entryCount); idx++ {
		entryOffset := entryCountOffset + 2 + idx*12
		tag, ok := readUint16(entryOffset)
		if !ok {
			return 0, false
		}
		if tag != 0x0112 {
			continue
		}

		valueType, ok := readUint16(entryOffset + 2)
		if !ok || valueType != 3 {
			return 0, false
		}
		valueCount, ok := readUint32(entryOffset + 4)
		if !ok || valueCount == 0 {
			return 0, false
		}

		var orientation uint16
		if valueCount == 1 {
			orientation, ok = readUint16(entryOffset + 8)
		} else {
			valueOffset, ok := readUint32(entryOffset + 8)
			if !ok {
				return 0, false
			}
			orientation, ok = readUint16(int(valueOffset))
		}
		if !ok || orientation < 1 || orientation > 8 {
			return 0, false
		}
		return int(orientation), true
	}

	return 1, true
}

func applyImageOrientation(src image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return src
	}

	source := toNRGBA(src)
	sourceBounds := source.Bounds()
	sourceWidth := sourceBounds.Dx()
	sourceHeight := sourceBounds.Dy()

	targetWidth := sourceWidth
	targetHeight := sourceHeight
	switch orientation {
	case 5, 6, 7, 8:
		targetWidth = sourceHeight
		targetHeight = sourceWidth
	}

	dst := image.NewNRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	for y := 0; y < targetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			sourceX, sourceY := orientedSourcePoint(x, y, sourceWidth, sourceHeight, orientation)
			dst.SetNRGBA(x, y, source.NRGBAAt(sourceX, sourceY))
		}
	}

	return dst
}

func orientedSourcePoint(targetX int, targetY int, sourceWidth int, sourceHeight int, orientation int) (int, int) {
	switch orientation {
	case 2:
		return sourceWidth - 1 - targetX, targetY
	case 3:
		return sourceWidth - 1 - targetX, sourceHeight - 1 - targetY
	case 4:
		return targetX, sourceHeight - 1 - targetY
	case 5:
		return targetY, targetX
	case 6:
		return targetY, sourceHeight - 1 - targetX
	case 7:
		return sourceWidth - 1 - targetY, sourceHeight - 1 - targetX
	case 8:
		return sourceWidth - 1 - targetY, targetX
	default:
		return targetX, targetY
	}
}
