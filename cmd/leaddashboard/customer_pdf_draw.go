package main

import (
	"fmt"
	"strings"
)

func (p *catalogPDFPage) fillRect(x, top, w, h float64, color [3]float64) {
	y := catalogPDFHeight - top - h
	fmt.Fprintf(&p.content, "%.3f %.3f %.3f rg %.2f %.2f %.2f %.2f re f\n", color[0], color[1], color[2], x, y, w, h)
}
func (p *catalogPDFPage) rectStroke(x, top, w, h float64, color [3]float64, lineWidth float64) {
	y := catalogPDFHeight - top - h
	fmt.Fprintf(&p.content, "%.3f %.3f %.3f RG %.2f w %.2f %.2f %.2f %.2f re S\n", color[0], color[1], color[2], lineWidth, x, y, w, h)
}
func (p *catalogPDFPage) line(x1, top1, x2, top2 float64, color [3]float64, lineWidth float64) {
	fmt.Fprintf(&p.content, "%.3f %.3f %.3f RG %.2f w %.2f %.2f m %.2f %.2f l S\n", color[0], color[1], color[2], lineWidth, x1, catalogPDFHeight-top1, x2, catalogPDFHeight-top2)
}
func (p *catalogPDFPage) text(x, top, size float64, bold bool, color [3]float64, value string) {
	font := "F1"
	if bold {
		font = "F2"
	}
	fmt.Fprintf(&p.content, "BT /%s %.2f Tf %.3f %.3f %.3f rg 1 0 0 1 %.2f %.2f Tm (%s) Tj ET\n", font, size, color[0], color[1], color[2], x, catalogPDFHeight-top-size, pdfEscapeText(value))
}
func (p *catalogPDFPage) textRight(right, top, size float64, bold bool, color [3]float64, value string) {
	p.text(right-pdfApproxTextWidth(value, size, bold), top, size, bold, color, value)
}
func (p *catalogPDFPage) textCenter(center, top, size float64, bold bool, color [3]float64, value string) {
	p.text(center-pdfApproxTextWidth(value, size, bold)/2, top, size, bold, color, value)
}
func (p *catalogPDFPage) wrappedText(x, top, width, size, lineHeight float64, maxLines int, bold bool, color [3]float64, value string) float64 {
	lines := pdfWrapText(value, width, size, bold, maxLines)
	for i, line := range lines {
		p.text(x, top+float64(i)*lineHeight, size, bold, color, line)
	}
	return float64(len(lines)) * lineHeight
}
func (p *catalogPDFPage) infoPair(x, top, width float64, label, value string) {
	p.text(x, top, 8.5, true, pdfMuted, strings.ToUpper(label))
	p.wrappedText(x, top+17, width, 10.5, 14, 2, false, pdfDark, blankDash(value))
}
func (p *catalogPDFPage) badge(x, top float64, value string, fill, textColor [3]float64) {
	w := pdfApproxTextWidth(value, 8.5, true) + 20
	if w < 70 {
		w = 70
	}
	p.fillRect(x, top, w, 22, fill)
	p.textCenter(x+w/2, top+6, 8.5, true, textColor, value)
}
func (p *catalogPDFPage) imageCover(img catalogImage, x, top, w, h float64) {
	if len(img.JPEG) == 0 || img.Width <= 0 || img.Height <= 0 {
		return
	}
	name := fmt.Sprintf("Im%d", len(p.images)+1)
	p.images = append(p.images, catalogPDFPageImage{Name: name, Image: img, X: x, Top: top, W: w, H: h})
	scale := max(w/float64(img.Width), h/float64(img.Height))
	drawW := float64(img.Width) * scale
	drawH := float64(img.Height) * scale
	drawX := x + (w-drawW)/2
	drawTop := top + (h-drawH)/2
	fmt.Fprintf(&p.content, "q %.2f %.2f %.2f %.2f re W n %.2f 0 0 %.2f %.2f %.2f cm /%s Do Q\n", x, catalogPDFHeight-top-h, w, h, drawW, drawH, drawX, catalogPDFHeight-drawTop-drawH, name)
}

func pdfWrapText(value string, width, size float64, bold bool, maxLines int) []string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
	if value == "" {
		return nil
	}
	words := strings.Fields(value)
	lines := make([]string, 0, maxLines)
	current := ""
	for _, word := range words {
		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if pdfApproxTextWidth(candidate, size, bold) <= width || current == "" {
			current = candidate
			continue
		}
		lines = append(lines, current)
		current = word
		if maxLines > 0 && len(lines) >= maxLines {
			break
		}
	}
	if (maxLines <= 0 || len(lines) < maxLines) && current != "" {
		lines = append(lines, current)
	}
	if maxLines > 0 && len(lines) == maxLines && len(strings.Fields(strings.Join(lines, " "))) < len(words) {
		last := strings.TrimSpace(lines[len(lines)-1])
		for pdfApproxTextWidth(last+"...", size, bold) > width && len(last) > 3 {
			last = strings.TrimSpace(last[:len(last)-1])
		}
		lines[len(lines)-1] = last + "..."
	}
	return lines
}
func pdfApproxTextWidth(value string, size float64, bold bool) float64 {
	factor := 0.50
	if bold {
		factor = 0.53
	}
	return float64(len([]rune(pdfSafeText(value)))) * size * factor
}
func pdfSafeText(value string) string {
	value = strings.NewReplacer("•", "-", "–", "-", "—", "-", "“", `"`, "”", `"`, "’", "'", "‘", "'", "…", "...").Replace(value)
	var out strings.Builder
	for _, r := range value {
		if r == '\n' || r == '\r' || r == '\t' {
			out.WriteByte(' ')
			continue
		}
		if r >= 32 && r <= 255 {
			out.WriteByte(byte(r))
		} else if r > 255 {
			out.WriteByte('?')
		}
	}
	return out.String()
}
func pdfEscapeText(value string) string {
	value = pdfSafeText(value)
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `(`, `\(`)
	return strings.ReplaceAll(value, `)`, `\)`)
}
