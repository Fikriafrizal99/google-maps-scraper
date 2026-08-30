package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"time"
)

var catalogHTTPClient = &http.Client{Timeout: 8 * time.Second}

func downloadCatalogImage(ctx context.Context, imageURL string) (catalogImage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return catalogImage{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 LeadCatalog/1.0")
	resp, err := catalogHTTPClient.Do(req)
	if err != nil {
		return catalogImage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return catalogImage{}, fmt.Errorf("image http status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return catalogImage{}, err
	}
	return normalizeCatalogImage(data)
}

func normalizeCatalogImage(data []byte) (catalogImage, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return catalogImage{}, err
	}
	src = resizeCatalogImage(src, 1000)
	bounds := src.Bounds()
	var out bytes.Buffer
	if err := jpeg.Encode(&out, src, &jpeg.Options{Quality: 72}); err != nil {
		return catalogImage{}, err
	}
	return catalogImage{JPEG: out.Bytes(), Width: bounds.Dx(), Height: bounds.Dy()}, nil
}

func resizeCatalogImage(src image.Image, maxDimension int) image.Image {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if maxDimension <= 0 || (width <= maxDimension && height <= maxDimension) {
		return src
	}
	newWidth, newHeight := width, height
	if width >= height {
		newWidth = maxDimension
		newHeight = max(1, height*maxDimension/width)
	} else {
		newHeight = maxDimension
		newWidth = max(1, width*maxDimension/height)
	}
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	for y := 0; y < newHeight; y++ {
		sy := bounds.Min.Y + y*height/newHeight
		for x := 0; x < newWidth; x++ {
			sx := bounds.Min.X + x*width/newWidth
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}
