package image

import (
	"fmt"
	"image"
	"log"
	"os"

	_ "image/gif" // register gif
	"image/jpeg"
	//_ "image/jpeg" // register jpeg
	_ "image/png"  // register png

	"golang.org/x/image/draw"
)

// CompressAndSaveImage resizes & compresses the image before saving
func CompressAndSaveImage(img image.Image, savePath string, width, height, quality int) error {
	log.Printf("[INFO] Starting compression for image -> savePath=%s, targetSize=%dx%d, quality=%d",
		savePath, width, height, quality)

	// Resize to fit within width x height
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	log.Printf("[DEBUG] Image resized successfully -> newSize=%dx%d", width, height)

	// Create output file
	out, err := os.Create(savePath)
	if err != nil {
		log.Printf("[ERROR] Failed to create file -> path=%s, err=%v", savePath, err)
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() {
		if cerr := out.Close(); cerr != nil {
			log.Printf("[WARN] Failed to close file -> path=%s, err=%v", savePath, cerr)
		}
	}()

	// Save as JPEG with compression
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(out, dst, opts); err != nil {
		log.Printf("[ERROR] Failed to encode image -> path=%s, err=%v", savePath, err)
		return fmt.Errorf("failed to encode image: %w", err)
	}

	log.Printf("[INFO] Successfully compressed & saved image -> path=%s", savePath)
	return nil
}
