package main

import (
	"strings"
	"testing"
)

func TestDashboardExportLinksPreserveFilters(t *testing.T) {
	t.Parallel()
	data := dashboardPageData{
		InternalExportURL: "/export.csv?segment=putri&verification_status=verified",
		CustomerExportURL: "/export/customer.csv?segment=putri&verification_status=verified",
		Page:              1,
		TotalPages:        1,
	}
	var out strings.Builder
	if err := dashboardV2Tmpl.Execute(&out, data); err != nil {
		t.Fatalf("render dashboard: %v", err)
	}
	html := out.String()
	if !strings.Contains(html, `href="/export/customer.csv?segment=putri&amp;verification_status=verified"`) {
		t.Fatalf("customer export link lost filters: %s", html)
	}
	if !strings.Contains(html, `href="/export.csv?segment=putri&amp;verification_status=verified"`) {
		t.Fatalf("internal export link lost filters: %s", html)
	}
}
