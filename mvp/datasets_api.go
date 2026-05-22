package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

// v0.7.12: dataset metadata page. Reads the embedded
// datasets/MANIFEST.json (curated list of public wheat panels, etc.) and
// merges it with the actual file sizes present on the server in
// <bindir>/data/datasets/. The /api/datasets endpoint returns the combined
// JSON; static/datasets.html renders it as a table.

type datasetManifest struct {
	Version           int               `json:"version"`
	Description       string            `json:"description"`
	DeployTruncateMb  int               `json:"deploy_truncate_mb"`
	Datasets          []datasetManifestEntry `json:"datasets"`
}

type datasetManifestEntry struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Filename        string `json:"filename"`
	Format          string `json:"format"`
	SourceURL       string `json:"source_url"`
	LandingURL      string `json:"landing_url"`
	License         string `json:"license"`
	Accessions      *int   `json:"accessions"`
	Markers         *int   `json:"markers"`
	Ploidy          string `json:"ploidy,omitempty"`
	SizeBytes       *int64 `json:"size_bytes"`
	Category        string `json:"category"`
	DeployStrategy  string `json:"deploy_strategy"`
	ManualReason    string `json:"manual_reason,omitempty"`
	Content         string `json:"content"`
}

type datasetAPIEntry struct {
	datasetManifestEntry
	DeployedBytes *int64 `json:"deployed_bytes"`
	Truncated     bool   `json:"truncated"`
	Status        string `json:"status"` // "missing", "full", "truncated", "manual", "stale"
}

type datasetAPIResponse struct {
	Version           int                  `json:"version"`
	Description       string               `json:"description"`
	DeployTruncateMb  int                  `json:"deploy_truncate_mb"`
	ServerDatasetsDir string               `json:"server_datasets_dir"`
	Datasets          []datasetAPIEntry    `json:"datasets"`
}

func datasetsAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "use GET", http.StatusMethodNotAllowed)
		return
	}
	var manifest datasetManifest
	if err := json.Unmarshal(embeddedDatasetsManifest, &manifest); err != nil {
		http.Error(w, "manifest parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	datasetsDir := serverDatasetsDir()
	resp := datasetAPIResponse{
		Version:           manifest.Version,
		Description:       manifest.Description,
		DeployTruncateMb:  manifest.DeployTruncateMb,
		ServerDatasetsDir: datasetsDir,
		Datasets:          make([]datasetAPIEntry, 0, len(manifest.Datasets)),
	}
	for _, m := range manifest.Datasets {
		entry := datasetAPIEntry{datasetManifestEntry: m, Status: "missing"}
		path := filepath.Join(datasetsDir, m.Filename)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			n := info.Size()
			entry.DeployedBytes = &n
			full := m.SizeBytes
			switch {
			case m.DeployStrategy == "manual":
				if full != nil && n == *full {
					entry.Status = "full"
				} else {
					entry.Status = "manual"
				}
			case full != nil && n == *full:
				entry.Status = "full"
			case full != nil && n < *full:
				entry.Status = "truncated"
				entry.Truncated = true
			default:
				entry.Status = "stale"
			}
		} else if m.DeployStrategy == "manual" {
			entry.Status = "manual"
		}
		resp.Datasets = append(resp.Datasets, entry)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
}

// serverDatasetsDir resolves to <bindir>/data/datasets/ at runtime. Mirrors
// the lookup in dataset.go for external CSVs.
func serverDatasetsDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "data/datasets"
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err == nil {
		exe = resolved
	}
	return filepath.Join(filepath.Dir(exe), "data", "datasets")
}
