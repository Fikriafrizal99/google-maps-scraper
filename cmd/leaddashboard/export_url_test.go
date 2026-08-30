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
	for _, want := range []string{
		`href="/export/customer.pdf?segment=putri&amp;verification_status=verified"`,
		`href="/export/customer.xlsx?segment=putri&amp;verification_status=verified"`,
		`href="/export.csv?segment=putri&amp;verification_status=verified"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("export link lost filters, missing %s: %s", want, html)
		}
	}
	if strings.Contains(html, `>Customer CSV<`) {
		t.Fatalf("customer CSV should not be exposed in primary toolbar: %s", html)
	}
}

func TestCustomerDeliveryURLRejectsExternalBase(t *testing.T) {
	if got := customerDeliveryURL("https://evil.example/export/customer.csv?segment=putri", "/export/customer.pdf"); got != "/export/customer.pdf" {
		t.Fatalf("unexpected external-base result: %q", got)
	}
	if got := customerDeliveryURL("/export/customer.csv?segment=putri&has_phone=1", "/export/customer.xlsx"); got != "/export/customer.xlsx?segment=putri&has_phone=1" {
		t.Fatalf("filters not preserved: %q", got)
	}
}
