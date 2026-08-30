package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (a *app) handleGeoProvinces(w http.ResponseWriter, r *http.Request) {
	regions, err := a.geo.Provinces(r.Context())
	writeGeoResponse(w, regions, err)
}

func (a *app) handleGeoRegencies(w http.ResponseWriter, r *http.Request) {
	provinceID := strings.TrimSpace(r.URL.Query().Get("province_id"))
	if provinceID == "" {
		http.Error(w, "province_id is required", http.StatusBadRequest)
		return
	}
	regions, err := a.geo.Regencies(r.Context(), provinceID)
	writeGeoResponse(w, regions, err)
}

func (a *app) handleGeoDistricts(w http.ResponseWriter, r *http.Request) {
	regencyID := strings.TrimSpace(r.URL.Query().Get("regency_id"))
	if regencyID == "" {
		http.Error(w, "regency_id is required", http.StatusBadRequest)
		return
	}
	regions, err := a.geo.Districts(r.Context(), regencyID)
	writeGeoResponse(w, regions, err)
}

func (a *app) handleGeoVillages(w http.ResponseWriter, r *http.Request) {
	districtID := strings.TrimSpace(r.URL.Query().Get("district_id"))
	if districtID == "" {
		http.Error(w, "district_id is required", http.StatusBadRequest)
		return
	}
	regions, err := a.geo.Villages(r.Context(), districtID)
	writeGeoResponse(w, regions, err)
}

func writeGeoResponse(w http.ResponseWriter, value any, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_ = json.NewEncoder(w).Encode(value)
}
