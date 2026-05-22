package main

import (
	"bufio"
	"embed"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
)

// v0.7.4 dataset loader (Issue 04 — public-data proof).
//
// The simulator can use real founder genotypes instead of a generated synthetic
// population. The expected on-disk format is a simple CSV:
//
//   # comments and metadata may appear as lines starting with '#'
//   # placeholder: true               <-- if present, marks the file as a non-real placeholder
//   accession_id,m1,m2,m3,...,mN
//   1001,0,1,2,0,...,1
//   1002,2,1,0,2,...,0
//
// Values are 0/1/2 (allele dose per locus, diploid biallelic).
// Rows are individual accessions; columns after the ID are SNP markers.
//
// Files are embedded into the binary via //go:embed. The first-class
// dataset name `arabidopsis1001` resolves to `data/arabidopsis1001.csv`
// when present; otherwise to the shipped placeholder
// `data/example_founders.csv` (clearly marked as a placeholder in metadata).

//go:embed data/*.csv
var embeddedDatasets embed.FS

type loadedDataset struct {
	individuals     []organism
	markerCount     int
	accessionIDs    []string
	isPlaceholder   bool
	sourceFile      string
	sourceNotes     []string
}

func loadDataset(name string) (*loadedDataset, error) {
	candidate := "data/" + name + ".csv"
	data, err := embeddedDatasets.ReadFile(candidate)
	if err != nil {
		// Fall back to the placeholder so the UI dropdown still works during early v0.7.4.
		data, err = embeddedDatasets.ReadFile("data/example_founders.csv")
		if err != nil {
			return nil, fmt.Errorf("no embedded dataset %q and no fallback example_founders.csv: %w", name, err)
		}
		candidate = "data/example_founders.csv"
	}
	return parseDatasetCSV(data, candidate)
}

func parseDatasetCSV(raw []byte, sourceFile string) (*loadedDataset, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	out := &loadedDataset{sourceFile: sourceFile}
	headerSeen := false
	var headerCols int
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if strings.Contains(strings.ToLower(line), "placeholder: true") {
				out.isPlaceholder = true
			}
			note := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			if note != "" {
				out.sourceNotes = append(out.sourceNotes, note)
			}
			continue
		}
		fields := strings.Split(line, ",")
		if !headerSeen {
			headerSeen = true
			headerCols = len(fields)
			if headerCols < 2 {
				return nil, fmt.Errorf("dataset %s: header has fewer than 2 columns", sourceFile)
			}
			out.markerCount = headerCols - 1
			continue
		}
		if len(fields) != headerCols {
			return nil, fmt.Errorf("dataset %s: line %d has %d cols, expected %d", sourceFile, lineNum, len(fields), headerCols)
		}
		geno := make([]uint8, out.markerCount)
		for i := 0; i < out.markerCount; i++ {
			v, err := strconv.Atoi(strings.TrimSpace(fields[i+1]))
			if err != nil {
				return nil, fmt.Errorf("dataset %s: line %d col %d: %w", sourceFile, lineNum, i+2, err)
			}
			if v < 0 || v > 2 {
				return nil, fmt.Errorf("dataset %s: line %d col %d value %d outside 0..2", sourceFile, lineNum, i+2, v)
			}
			geno[i] = uint8(v)
		}
		out.accessionIDs = append(out.accessionIDs, strings.TrimSpace(fields[0]))
		out.individuals = append(out.individuals, organism{geno: geno})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("dataset %s scan: %w", sourceFile, err)
	}
	if len(out.individuals) == 0 {
		return nil, fmt.Errorf("dataset %s: no individuals after header", sourceFile)
	}
	return out, nil
}

// subsampleDataset samples up to n accessions without replacement and (optionally)
// up to m markers (taking the first m columns deterministically — markers are
// typically pre-ordered by quality/MAF in the fetch script).
func subsampleDataset(ds *loadedDataset, n, m int, rng *rand.Rand) *loadedDataset {
	if ds == nil {
		return nil
	}
	if n <= 0 || n > len(ds.individuals) {
		n = len(ds.individuals)
	}
	if m <= 0 || m > ds.markerCount {
		m = ds.markerCount
	}
	idx := rng.Perm(len(ds.individuals))[:n]
	out := &loadedDataset{
		markerCount:   m,
		isPlaceholder: ds.isPlaceholder,
		sourceFile:    ds.sourceFile,
		sourceNotes:   ds.sourceNotes,
		individuals:   make([]organism, n),
		accessionIDs:  make([]string, n),
	}
	for i, src := range idx {
		geno := make([]uint8, m)
		copy(geno, ds.individuals[src].geno[:m])
		out.individuals[i] = organism{geno: geno}
		out.accessionIDs[i] = ds.accessionIDs[src]
	}
	return out
}
