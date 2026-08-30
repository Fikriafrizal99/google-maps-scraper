package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func (a *app) handleCollect(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	preset := strings.TrimSpace(r.FormValue("preset"))
	area := strings.TrimSpace(r.FormValue("area"))
	subarea := strings.TrimSpace(r.FormValue("subarea"))
	if !safeConfigName(preset) || !safeConfigName(area) {
		http.Error(w, "preset/area hanya boleh huruf, angka, dash, underscore", http.StatusBadRequest)
		return
	}
	if len(subarea) > 120 {
		http.Error(w, "subarea terlalu panjang", http.StatusBadRequest)
		return
	}
	depth := parseBoundedInt(r.FormValue("depth"), 5, 1, 30)
	concurrency := parseBoundedInt(r.FormValue("concurrency"), 2, 1, 8)

	a.collectMu.Lock()
	if a.collect.Running {
		a.collectMu.Unlock()
		http.Error(w, "collector masih berjalan", http.StatusConflict)
		return
	}
	a.collect = collectState{
		Running:   true,
		Message:   fmt.Sprintf("Collect %s / %s dimulai", preset, area),
		StartedAt: time.Now().Format("2006-01-02 15:04:05"),
	}
	a.collectMu.Unlock()

	go a.runCollector(preset, area, subarea, depth, concurrency)
	http.Redirect(w, r, "/?collect=started", http.StatusSeeOther)
}

func (a *app) runCollector(preset, area, subarea string, depth, concurrency int) {
	stamp := time.Now().Format("20060102-150405")
	output := filepath.Join("data", fmt.Sprintf("latest-%s-%s.csv", preset, area))
	logPath := filepath.Join("data", "collector-last.log")
	args := []string{
		"-preset", preset,
		"-area", area,
		"-config-dir", a.configDir,
		"-engine", a.enginePath,
		"-output", output,
		"-db", a.dbPath,
	}
	if subarea != "" {
		args = append(args, "-subarea", subarea)
	}
	args = append(args, "--", "-c", strconv.Itoa(concurrency), "-depth", strconv.Itoa(depth))

	logFile, err := os.Create(logPath)
	if err != nil {
		a.finishCollect("Gagal membuat collector log: " + err.Error())
		return
	}
	defer logFile.Close()

	cmd := exec.CommandContext(context.Background(), a.collectorPath, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Run(); err != nil {
		a.finishCollect(fmt.Sprintf("Collect gagal (%s). Lihat %s", err, logPath))
		return
	}
	a.finishCollect(fmt.Sprintf("Collect selesai %s · output %s · log %s", stamp, output, logPath))
}

func (a *app) finishCollect(message string) {
	a.collectMu.Lock()
	a.collect = collectState{Running: false, Message: message}
	a.collectMu.Unlock()
}

func (a *app) collectStatus() collectState {
	a.collectMu.RLock()
	defer a.collectMu.RUnlock()
	return a.collect
}

func (a *app) handleImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "invalid upload: "+err.Error(), http.StatusBadRequest)
		return
	}
	preset := strings.TrimSpace(r.FormValue("preset"))
	area := strings.TrimSpace(r.FormValue("area"))
	subarea := strings.TrimSpace(r.FormValue("subarea"))
	if preset == "" || area == "" {
		http.Error(w, "preset and area are required", http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "csv file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tmp, err := os.CreateTemp("", "lead-import-*.csv")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.ReadFrom(file); err != nil {
		tmp.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmp.Close(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	count, err := a.store.ImportCSV(context.Background(), path, preset, area, subarea)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/?imported="+strconv.Itoa(count), http.StatusSeeOther)
}

func safeConfigName(value string) bool {
	return regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(value)
}

func parsePositiveInt(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

func parseBoundedInt(value string, fallback, min, max int) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func waNumber(phone string) string {
	var digits strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	value := digits.String()
	if strings.HasPrefix(value, "0") {
		return "62" + strings.TrimPrefix(value, "0")
	}
	return value
}

func shortTime(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return t.Local().Format("02 Jan 2006 15:04")
}

func reviewLabel(status string) string {
	switch status {
	case "valid":
		return "Valid"
	case "needs_review":
		return "Needs Review"
	case "exclude":
		return "Exclude"
	default:
		return "Unreviewed"
	}
}

func leadImages(raw, thumbnail string, limit int) []string {
	if limit <= 0 {
		limit = 5
	}
	seen := make(map[string]struct{})
	images := make([]string, 0, limit)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || len(images) >= limit {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		images = append(images, value)
	}

	var records []imageRecord
	if json.Unmarshal([]byte(raw), &records) == nil {
		for _, record := range records {
			add(record.Image)
		}
	}
	add(thumbnail)
	return images
}
