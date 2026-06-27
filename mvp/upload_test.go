package main

// v0.7.26 — Issue 05. Upload-workflow tests.
//
// Coverage:
//   - Each parser on its happy path against the shipped fixtures.
//   - Each parser on a hand-built malformed input that should fail with
//     a useful error.
//   - Upload cache: put/get round-trip + TTL eviction + capacity eviction.
//   - End-to-end: putUpload + runSimulation reads the upload genotype as
//     the founder population.

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return b
}

func TestParseUploadGenotype_Fixture(t *testing.T) {
	g, err := parseUploadGenotype(mustRead(t, "fixtures/upload-toy/genotype.csv"))
	if err != nil {
		t.Fatalf("parse genotype: %v", err)
	}
	if g.Individuals != 30 {
		t.Errorf("Individuals = %d, want 30", g.Individuals)
	}
	if g.Markers != 80 {
		t.Errorf("Markers = %d, want 80", g.Markers)
	}
	if len(g.SampleIDs) != 5 {
		t.Errorf("SampleIDs = %d, want 5", len(g.SampleIDs))
	}
	if g.dataset == nil {
		t.Fatalf("dataset not populated")
	}
}

func TestParseUploadGenotype_BadValue(t *testing.T) {
	bad := []byte("id,marker_1,marker_2\nplant_001,0,3\n")
	_, err := parseUploadGenotype(bad)
	if err == nil || !strings.Contains(err.Error(), "outside 0..2") {
		t.Fatalf("expected out-of-range error, got %v", err)
	}
}

func TestParseUploadGenotype_NonInteger(t *testing.T) {
	bad := []byte("id,marker_1\nplant_001,A\n")
	_, err := parseUploadGenotype(bad)
	if err == nil {
		t.Fatalf("expected parse error for non-integer marker")
	}
}

func TestParseUploadGenotype_ColumnMismatch(t *testing.T) {
	bad := []byte("id,marker_1,marker_2\nplant_001,0,1,2\n")
	_, err := parseUploadGenotype(bad)
	if err == nil || !strings.Contains(err.Error(), "expected") {
		t.Fatalf("expected column-count error, got %v", err)
	}
}

func TestParseUploadPhenotype_Fixture(t *testing.T) {
	p, err := parseUploadPhenotype(mustRead(t, "fixtures/upload-toy/phenotype.csv"))
	if err != nil {
		t.Fatalf("parse phenotype: %v", err)
	}
	if p.Rows != 30 {
		t.Errorf("Rows = %d, want 30", p.Rows)
	}
	if p.TraitName != "trait_value" {
		t.Errorf("TraitName = %q, want trait_value", p.TraitName)
	}
	if p.Min >= p.Max {
		t.Errorf("Min %f >= Max %f", p.Min, p.Max)
	}
}

func TestParseUploadPhenotype_WrongHeader(t *testing.T) {
	bad := []byte("plant,trait\nplant_001,1.0\n")
	_, err := parseUploadPhenotype(bad)
	if err == nil || !strings.Contains(err.Error(), "must be 'id'") {
		t.Fatalf("expected 'id' header error, got %v", err)
	}
}

func TestParseUploadPhenotype_NonNumeric(t *testing.T) {
	bad := []byte("id,trait\nplant_001,abc\n")
	_, err := parseUploadPhenotype(bad)
	if err == nil {
		t.Fatalf("expected non-numeric trait error")
	}
}

func TestParseUploadPedigree_Fixture(t *testing.T) {
	p, err := parseUploadPedigree(mustRead(t, "fixtures/upload-toy/pedigree.csv"))
	if err != nil {
		t.Fatalf("parse pedigree: %v", err)
	}
	if p.Rows != 30 {
		t.Errorf("Rows = %d, want 30", p.Rows)
	}
	if p.UniqueSires == 0 || p.UniqueDams == 0 {
		t.Errorf("UniqueSires=%d UniqueDams=%d, both must be >0", p.UniqueSires, p.UniqueDams)
	}
}

func TestParseUploadPedigree_MissingDam(t *testing.T) {
	bad := []byte("id,sire\nplant_001,sire_a\n")
	_, err := parseUploadPedigree(bad)
	if err == nil || !strings.Contains(err.Error(), "3 columns") {
		t.Fatalf("expected column-count error, got %v", err)
	}
}

func TestParseUploadEdits_Fixture(t *testing.T) {
	e, err := parseUploadEdits(mustRead(t, "fixtures/upload-toy/edits.csv"))
	if err != nil {
		t.Fatalf("parse edits: %v", err)
	}
	if e.Rows != 3 {
		t.Errorf("Rows = %d, want 3", e.Rows)
	}
	if e.Entries[0].MarkerID != "marker_5" {
		t.Errorf("Entries[0].MarkerID = %q, want marker_5", e.Entries[0].MarkerID)
	}
}

func TestParseUploadEdits_BadAllele(t *testing.T) {
	bad := []byte("marker_id,target_allele,expected_effect\nmarker_1,3,0.1\n")
	_, err := parseUploadEdits(bad)
	if err == nil || !strings.Contains(err.Error(), "outside 0..2") {
		t.Fatalf("expected out-of-range error, got %v", err)
	}
}

func TestUploadCache_PutGetRoundtrip(t *testing.T) {
	resetUploadCache()
	d := &UploadedDataset{}
	id := putUpload(d)
	if id == "" {
		t.Fatalf("putUpload returned empty id")
	}
	got, ok := getUpload(id)
	if !ok || got != d {
		t.Fatalf("getUpload roundtrip failed: ok=%v got=%v want=%v", ok, got, d)
	}
}

func TestUploadCache_TTLEviction(t *testing.T) {
	resetUploadCache()
	d := &UploadedDataset{}
	id := putUpload(d)
	// Backdate creation past the TTL.
	uploadCache.Lock()
	uploadCache.items[id].CreatedAt = time.Now().Add(-2 * uploadTTL)
	uploadCache.Unlock()
	if _, ok := getUpload(id); ok {
		t.Fatalf("expected upload to be expired and evicted")
	}
}

func TestUploadCache_CapacityEviction(t *testing.T) {
	resetUploadCache()
	// Fill past cap; oldest should be dropped.
	ids := make([]string, 0, uploadCacheMax+5)
	for i := 0; i < uploadCacheMax+5; i++ {
		ids = append(ids, putUpload(&UploadedDataset{}))
		// Force monotonically increasing creation times so eviction is deterministic.
		uploadCache.Lock()
		uploadCache.items[ids[len(ids)-1]].CreatedAt = time.Now().Add(time.Duration(i) * time.Microsecond)
		uploadCache.Unlock()
	}
	uploadCache.Lock()
	n := len(uploadCache.items)
	uploadCache.Unlock()
	if n > uploadCacheMax {
		t.Fatalf("cache held %d items, want ≤ %d", n, uploadCacheMax)
	}
	// The very first id (oldest) should be gone.
	if _, ok := getUpload(ids[0]); ok {
		t.Fatalf("expected oldest upload to be evicted past capacity")
	}
}

func TestRunSimulation_UsesUploadedGenotype(t *testing.T) {
	resetUploadCache()
	g, err := parseUploadGenotype(mustRead(t, "fixtures/upload-toy/genotype.csv"))
	if err != nil {
		t.Fatalf("parse genotype: %v", err)
	}
	id := putUpload(&UploadedDataset{Genotype: g})

	req := SimRequest{
		Seed:               1,
		PopulationSize:     30,
		Markers:            80,
		QTLCount:           10,
		Generations:        3,
		SelectionPercent:   20,
		Heritability:       0.5,
		MutationRate:       0.001,
		StrategySet:        "core",
		Replicates:         1,
		InbreedingLimit:    0.25,
		DiversityLossLimit: 0.3,
		Upload:             id,
	}
	resp, err := runSimulation(req)
	if err != nil {
		t.Fatalf("runSimulation: %v", err)
	}
	if resp.Request.PopulationSize != 30 {
		t.Errorf("PopulationSize after run = %d, want 30 (from upload)", resp.Request.PopulationSize)
	}
	if resp.Request.Markers != 80 {
		t.Errorf("Markers after run = %d, want 80 (from upload)", resp.Request.Markers)
	}
	// Notes should explicitly call out the upload path.
	foundNote := false
	for _, n := range resp.Notes {
		if strings.Contains(n, "Founder population came from upload") {
			foundNote = true
			break
		}
	}
	if !foundNote {
		t.Errorf("expected 'Founder population came from upload' note in resp.Notes")
	}
}

func TestRunSimulation_MissingUploadRejected(t *testing.T) {
	resetUploadCache()
	req := SimRequest{
		Seed:               1,
		PopulationSize:     20,
		Markers:            40,
		QTLCount:           5,
		Generations:        3,
		SelectionPercent:   20,
		Heritability:       0.5,
		MutationRate:       0.001,
		StrategySet:        "core",
		Replicates:         1,
		InbreedingLimit:    0.25,
		DiversityLossLimit: 0.3,
		Upload:             "does-not-exist",
	}
	_, err := runSimulation(req)
	if err == nil || !strings.Contains(err.Error(), "not found or expired") {
		t.Fatalf("expected not-found error for unknown upload, got %v", err)
	}
}

func TestUploadHandler_HappyPath(t *testing.T) {
	resetUploadCache()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("genotype", "genotype.csv")
	fw.Write(mustRead(t, "fixtures/upload-toy/genotype.csv"))
	fw, _ = mw.CreateFormFile("phenotype", "phenotype.csv")
	fw.Write(mustRead(t, "fixtures/upload-toy/phenotype.csv"))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()
	uploadHandler(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "upload_id") {
		t.Errorf("response missing upload_id: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "phenotype") {
		t.Errorf("response missing phenotype block: %s", rr.Body.String())
	}
}

func TestUploadHandler_MissingGenotype(t *testing.T) {
	resetUploadCache()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.Close() // no files.

	req := httptest.NewRequest(http.MethodPost, "/api/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()
	uploadHandler(rr, req)
	if rr.Code != 400 {
		t.Fatalf("status = %d (want 400), body = %s", rr.Code, rr.Body.String())
	}
}
