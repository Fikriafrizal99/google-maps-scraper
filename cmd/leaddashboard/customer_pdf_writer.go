package main

import (
	"bytes"
	"fmt"
	"strings"
)

func (d *catalogPDFDocument) bytes() ([]byte, error) {
	if len(d.pages) == 0 {
		return nil, fmt.Errorf("pdf has no pages")
	}
	objects := [][]byte{nil}
	reserve := func() int { objects = append(objects, nil); return len(objects) - 1 }
	add := func(body []byte) int { id := reserve(); objects[id] = body; return id }
	catalogID := reserve()
	pagesID := reserve()
	fontRegularID := add([]byte(`<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>`))
	fontBoldID := add([]byte(`<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>`))
	pageIDs := make([]int, 0, len(d.pages))
	for _, page := range d.pages {
		imageIDs := make([]int, len(page.images))
		for i, img := range page.images {
			dict := fmt.Sprintf(`<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>`, img.Image.Width, img.Image.Height, len(img.Image.JPEG))
			body := append([]byte(dict+"\nstream\n"), img.Image.JPEG...)
			body = append(body, []byte("\nendstream")...)
			imageIDs[i] = add(body)
		}
		contentData := []byte(page.content.String())
		contentID := add([]byte(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(contentData), contentData)))
		annotationIDs := make([]int, 0, len(page.links))
		for _, link := range page.links {
			if safeHTTPURL(link.URL) == "" {
				continue
			}
			y1 := catalogPDFHeight - link.Top - link.H
			y2 := catalogPDFHeight - link.Top
			body := fmt.Sprintf(`<< /Type /Annot /Subtype /Link /Rect [%.2f %.2f %.2f %.2f] /Border [0 0 0] /A << /S /URI /URI (%s) >> >>`, link.X, y1, link.X+link.W, y2, pdfEscapeText(link.URL))
			annotationIDs = append(annotationIDs, add([]byte(body)))
		}
		var resources strings.Builder
		fmt.Fprintf(&resources, `<< /Font << /F1 %d 0 R /F2 %d 0 R >>`, fontRegularID, fontBoldID)
		if len(imageIDs) > 0 {
			resources.WriteString(` /XObject <<`)
			for i, id := range imageIDs {
				fmt.Fprintf(&resources, ` /%s %d 0 R`, page.images[i].Name, id)
			}
			resources.WriteString(` >>`)
		}
		resources.WriteString(` >>`)
		var annots string
		if len(annotationIDs) > 0 {
			var refs strings.Builder
			refs.WriteString(" /Annots [")
			for _, id := range annotationIDs {
				fmt.Fprintf(&refs, "%d 0 R ", id)
			}
			refs.WriteString("]")
			annots = refs.String()
		}
		pageBody := fmt.Sprintf(`<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %.2f %.2f] /Resources %s /Contents %d 0 R%s >>`, pagesID, catalogPDFWidth, catalogPDFHeight, resources.String(), contentID, annots)
		pageIDs = append(pageIDs, add([]byte(pageBody)))
	}
	var kids strings.Builder
	for _, id := range pageIDs {
		fmt.Fprintf(&kids, "%d 0 R ", id)
	}
	objects[pagesID] = []byte(fmt.Sprintf(`<< /Type /Pages /Kids [%s] /Count %d >>`, kids.String(), len(pageIDs)))
	objects[catalogID] = []byte(fmt.Sprintf(`<< /Type /Catalog /Pages %d 0 R >>`, pagesID))
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n")
	offsets := make([]int, len(objects))
	for id := 1; id < len(objects); id++ {
		offsets[id] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n", id)
		out.Write(objects[id])
		out.WriteString("\nendobj\n")
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n", len(objects))
	out.WriteString("0000000000 65535 f \n")
	for id := 1; id < len(objects); id++ {
		fmt.Fprintf(&out, "%010d 00000 n \n", offsets[id])
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root %d 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects), catalogID, xref)
	return out.Bytes(), nil
}
