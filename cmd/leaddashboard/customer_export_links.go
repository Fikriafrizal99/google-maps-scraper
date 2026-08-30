package main

import "net/url"

func (d dashboardPageData) CustomerPDFURL() string {
	return customerDeliveryURL(d.CustomerExportURL, "/export/customer.pdf")
}

func (d dashboardPageData) CustomerExcelURL() string {
	return customerDeliveryURL(d.CustomerExportURL, "/export/customer.xlsx")
}

func customerDeliveryURL(base, path string) string {
	parsed, err := url.Parse(base)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return path
	}
	parsed.Path = path
	parsed.RawPath = ""
	return parsed.RequestURI()
}
