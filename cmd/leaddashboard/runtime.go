package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	collectorLogPath  = "data/collector-last.log"
	collectTimeLayout = "2006-01-02 15:04:05"
)

func (a *app) handleCollect(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	preset := strings.TrimSpace(r.FormValue("preset"))
	area := strings.TrimSpace(r.FormValue("area"))
	subarea := strings.TrimSpace(r.FormValue("subarea"))
	location := strings.TrimSpace(r.FormValue("location"))
	if !safeConfigName(preset) || !safeConfigName(area) {
		http.Error(w, "preset/area hanya boleh huruf, angka, dash, underscore", http.StatusBadRequest)
		return
	}
	if len(subarea) > 120 {
		http.Error(w, "subarea terlalu panjang", http.StatusBadRequest)
		return
	}
	if len(location) > 300 {
		http.Error(w, "location terlalu panjang", http.StatusBadRequest)
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
	label := area
	if location != "" {
		label = location
	} else if subarea != "" {
		label = subarea
	}
	a.collect = collectState{
		Running:     true,
		Stage:       "starting",
		Message:     fmt.Sprintf("Collect %s / %s dimulai", preset, label),
		StartedAt:   time.Now().Format(collectTimeLayout),
		Location:    label,
		Depth:       depth,
		Concurrency: concurrency,
	}
	a.collectProcess = nil
	a.collectMu.Unlock()

	go a.runCollector(preset, area, subarea, location, depth, concurrency)
	http.Redirect(w, r, "/?collect=started", http.StatusSeeOther)
}

func (a *app) runCollector(preset, area, subarea, location string, depth, concurrency int) {
	stamp := time.Now().Format("20060102-150405")
	output := filepath.Join("data", fmt.Sprintf("latest-%s-%s.csv", preset, area))
	logPath := filepath.FromSlash(collectorLogPath)
	args := []string{
		"-preset", preset,
		"-area", area,
		"-config-dir", a.configDir,
		"-engine", a.enginePath,
		"-output", output,
		"-db", a.dbPath,
	}
	if location != "" {
		args = append(args, "-location", location)
	} else if subarea != "" {
		args = append(args, "-subarea", subarea)
	}
	args = append(args, "--", "-c", strconv.Itoa(concurrency), "-depth", strconv.Itoa(depth))

	logFile, err := os.Create(logPath)
	if err != nil {
		a.finishCollect("failed", "Gagal membuat collector log: "+err.Error())
		return
	}
	defer logFile.Close()

	cmd := exec.Command(a.collectorPath, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		a.finishCollect("failed", fmt.Sprintf("Collect gagal dimulai (%s)", err))
		return
	}

	a.collectMu.Lock()
	a.collectProcess = cmd.Process
	cancelNow := a.collect.CancelRequested
	if !cancelNow {
		a.collect.Stage = "running"
	}
	a.collectMu.Unlock()

	if cancelNow && cmd.Process != nil {
		_ = cmd.Process.Signal(os.Interrupt)
	}

	err = cmd.Wait()
	a.collectMu.RLock()
	cancelled := a.collect.CancelRequested
	a.collectMu.RUnlock()

	if cancelled {
		a.finishCollect("cancelled", "Collect dibatalkan oleh pengguna")
		return
	}
	if err != nil {
		a.finishCollect("failed", fmt.Sprintf("Collect gagal (%s). Lihat %s", err, logPath))
		return
	}

	finalLog := tailTextFile(logPath, 64<<10)
	finalProgress := collectState{}
	applyCollectProgress(&finalProgress, finalLog)
	if finalProgress.Stage == "partial" {
		a.finishCollect("partial", fmt.Sprintf("Collect selesai sebagian %s · %d raw → %d final → %d diproses ke DB", stamp, finalProgress.RawRows, finalProgress.FinalRows, finalProgress.ImportedRows))
		return
	}

	a.finishCollect("completed", fmt.Sprintf("Collect selesai %s · output %s · log %s", stamp, output, logPath))
}

func (a *app) finishCollect(stage, message string) {
	a.collectMu.Lock()
	state := a.collect
	state.Running = false
	state.Stage = stage
	state.Message = message
	state.FinishedAt = time.Now().Format(collectTimeLayout)
	a.collect = state
	a.collectProcess = nil
	a.collectMu.Unlock()
}

func (a *app) collectStatus() collectState {
	a.collectMu.RLock()
	state := a.collect
	a.collectMu.RUnlock()

	state.Log = tailTextFile(filepath.FromSlash(collectorLogPath), 64<<10)
	applyCollectProgress(&state, state.Log)
	state.Elapsed = collectElapsed(state.StartedAt, state.FinishedAt, state.Running)
	if strings.TrimSpace(state.Stage) == "" {
		state.Stage = "idle"
	}
	if state.Running && state.Stage == "stalled" {
		state.Message = fmt.Sprintf("Scraper tidak menghasilkan data baru selama %s. Hasil sementara sedang diselamatkan.", formatIdleAge(state.IdleSeconds))
	}
	return state
}

func (a *app) handleCollectStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(a.collectStatus())
}

func (a *app) handleCollectCancel(w http.ResponseWriter, r *http.Request) {
	a.collectMu.Lock()
	if !a.collect.Running {
		a.collectMu.Unlock()
		http.Error(w, "tidak ada collector yang sedang berjalan", http.StatusConflict)
		return
	}
	a.collect.CancelRequested = true
	a.collect.Stage = "cancelling"
	a.collect.Message = "Permintaan batal dikirim. Menunggu scraper berhenti..."
	process := a.collectProcess
	a.collectMu.Unlock()

	if process != nil {
		if err := process.Signal(os.Interrupt); err != nil {
			a.collectMu.Lock()
			a.collect.Message = "Permintaan batal tercatat; menunggu proses collector berhenti"
			a.collectMu.Unlock()
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(a.collectStatus())
}

func applyCollectProgress(state *collectState, logText string) {
	if state == nil || strings.TrimSpace(logText) == "" {
		return
	}
	var latest map[string]string
	for _, line := range strings.Split(logText, "\n") {
		index := strings.Index(line, "KOST_PROGRESS ")
		if index < 0 {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(line[index+len("KOST_PROGRESS "):]))
		values := make(map[string]string, len(fields))
		for _, field := range fields {
			key, value, ok := strings.Cut(field, "=")
			if ok {
				values[key] = value
			}
		}
		if len(values) > 0 {
			latest = values
		}
	}
	if latest == nil {
		return
	}
	parseInt := func(key string, target *int) {
		if value := latest[key]; value != "" {
			if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
				*target = parsed
			}
		}
	}
	parseInt("queries", &state.QueryCount)
	parseInt("raw_rows", &state.RawRows)
	parseInt("final_rows", &state.FinalRows)
	parseInt("imported_rows", &state.ImportedRows)
	parseInt("idle_seconds", &state.IdleSeconds)

	if stage := strings.TrimSpace(latest["stage"]); stage != "" {
		if state.Running || !terminalCollectStage(state.Stage) {
			state.Stage = stage
		}
	}
}

func terminalCollectStage(stage string) bool {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "completed", "partial", "cancelled", "failed":
		return true
	default:
		return false
	}
}

func tailTextFile(path string, maxBytes int64) string {
	if maxBytes <= 0 {
		maxBytes = 64 << 10
	}
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return ""
	}
	start := stat.Size() - maxBytes
	if start < 0 {
		start = 0
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes))
	if err != nil {
		return ""
	}
	text := string(data)
	if start > 0 {
		if index := strings.IndexByte(text, '\n'); index >= 0 {
			text = text[index+1:]
		}
	}
	return strings.TrimSpace(text)
}

func collectElapsed(startRaw, finishRaw string, running bool) string {
	start, err := time.ParseInLocation(collectTimeLayout, strings.TrimSpace(startRaw), time.Local)
	if err != nil {
		return "-"
	}
	end := time.Now()
	if !running && strings.TrimSpace(finishRaw) != "" {
		if parsed, parseErr := time.ParseInLocation(collectTimeLayout, strings.TrimSpace(finishRaw), time.Local); parseErr == nil {
			end = parsed
		}
	}
	duration := end.Sub(start)
	if duration < 0 {
		duration = 0
	}
	seconds := int(duration.Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %02ds", minutes, seconds%60)
	}
	hours := minutes / 60
	return fmt.Sprintf("%dh %02dm", hours, minutes%60)
}

func formatIdleAge(seconds int) string {
	if seconds <= 0 {
		return "baru saja"
	}
	if seconds < 60 {
		return fmt.Sprintf("%dd", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %02dd", minutes, seconds%60)
	}
	hours := minutes / 60
	return fmt.Sprintf("%dj %02dm", hours, minutes%60)
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
