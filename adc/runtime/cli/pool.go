package cli

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

type poolRow struct {
	Model       string
	PersonaFile string
	Gene        int
	Cluster     int
}

type poolRecord struct {
	Model        string
	PersonaFile  string
	GeneClusters map[int]map[int]bool
}

func RunPool(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("pool", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc pool [--size N]\n\n")
		fmt.Fprintf(stderr, "Run from adc/.  The command reads ../common/data/personas/persona-clusters.csv relative to the current working directory.\n\n")
		fs.PrintDefaults()
	})
	size := fs.Int("size", 100, "Number of model/persona records to sample")
	verbose := fs.Bool("v", false, "Verbose selection logging")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("pool accepts no positional arguments")
	}
	if *size <= 0 {
		return fmt.Errorf("--size must be > 0")
	}

	path, err := resolvePoolRowsPath()
	if err != nil {
		return err
	}
	rows, err := loadPoolRows(path)
	if err != nil {
		return err
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	trace := io.Writer(io.Discard)
	if *verbose {
		trace = stderr
	}
	selected, err := samplePoolRecords(rows, *size, rng, stderr, trace)
	if err != nil {
		return err
	}
	for _, record := range selected {
		if _, err := fmt.Fprintf(stdout, "%s,%s\n", record.Model, record.PersonaFile); err != nil {
			return err
		}
	}
	return nil
}

func resolvePoolRowsPath() (string, error) {
	const relPath = "../common/data/personas/persona-clusters.csv"
	if fileExists(relPath) {
		return relPath, nil
	}
	return "", fmt.Errorf("cannot find %s from the current working directory", relPath)
}

func loadPoolRows(path string) ([]poolRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.FieldsPerRecord = 4
	reader.TrimLeadingSpace = true
	rawRows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	rows := make([]poolRow, 0, len(rawRows))
	for _, row := range rawRows {
		model := strings.TrimSpace(row[0])
		personaFile := strings.TrimSpace(row[1])
		gene, err := strconv.Atoi(strings.TrimSpace(row[2]))
		if err != nil {
			return nil, fmt.Errorf("parse gene index %q in %s: %w", row[2], path, err)
		}
		cluster, err := strconv.Atoi(strings.TrimSpace(row[3]))
		if err != nil {
			return nil, fmt.Errorf("parse cluster %q in %s: %w", row[3], path, err)
		}
		if model == "" || personaFile == "" {
			return nil, fmt.Errorf("invalid empty model or persona file in %s", path)
		}
		rows = append(rows, poolRow{
			Model:       model,
			PersonaFile: personaFile,
			Gene:        gene,
			Cluster:     cluster,
		})
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s contains no records", path)
	}
	return rows, nil
}

func samplePoolRecords(rows []poolRow, size int, rng *rand.Rand, warnings io.Writer, trace io.Writer) ([]poolRecord, error) {
	if size <= 0 {
		return nil, fmt.Errorf("size must be > 0")
	}
	available := collapsePoolRows(rows)
	if len(available) == 0 {
		return nil, fmt.Errorf("persona cluster records contain no usable model/persona pairs")
	}
	if size > len(available) {
		if _, err := fmt.Fprintf(warnings, "warning: --size=%d exceeds available unique model/persona records (%d)\n", size, len(available)); err != nil {
			return nil, err
		}
	}
	selected := make([]poolRecord, 0, size)
	for len(selected) < size {
		genes := availableGenes(available, map[int]bool{})
		if len(genes) == 0 {
			return nil, fmt.Errorf("pool sampling found no usable genes")
		}
		passLimit := rng.Intn(len(genes)) + 1
		if _, err := fmt.Fprintf(trace, "Selection %d: max passes %d.\n", len(selected)+1, passLimit); err != nil {
			return nil, err
		}
		picked, err := sampleOnePoolRecord(available, map[int]bool{}, len(selected)+1, 0, passLimit, rng, trace)
		if err != nil {
			return nil, err
		}
		selected = append(selected, picked)
	}
	return selected, nil
}

func sampleOnePoolRecord(current []poolRecord, usedGenes map[int]bool, selection int, passes int, passLimit int, rng *rand.Rand, log io.Writer) (poolRecord, error) {
	if len(current) == 0 {
		return poolRecord{}, fmt.Errorf("pool sampling produced no remaining records")
	}
	genes := availableGenes(current, usedGenes)
	if len(genes) == 0 || passes >= passLimit {
		return current[rng.Intn(len(current))], nil
	}
	gene := genes[rng.Intn(len(genes))]
	clusters := availableClusters(current, gene)
	if len(clusters) == 0 {
		nextUsed := cloneUsedGenes(usedGenes)
		nextUsed[gene] = true
		return sampleOnePoolRecord(current, nextUsed, selection, passes, passLimit, rng, log)
	}
	cluster := clusters[rng.Intn(len(clusters))]
	filtered := filterPoolRecords(current, gene, cluster)
	if _, err := fmt.Fprintf(log, "Selection %d: picked gene %d and cluster %d, leaving %d records.\n", selection, gene, cluster, len(filtered)); err != nil {
		return poolRecord{}, err
	}
	nextUsed := cloneUsedGenes(usedGenes)
	nextUsed[gene] = true
	return sampleOnePoolRecord(filtered, nextUsed, selection, passes+1, passLimit, rng, log)
}

func collapsePoolRows(rows []poolRow) []poolRecord {
	byPair := map[string]*poolRecord{}
	order := make([]string, 0)
	for _, row := range rows {
		key := row.Model + "\x00" + row.PersonaFile
		record := byPair[key]
		if record == nil {
			record = &poolRecord{
				Model:        row.Model,
				PersonaFile:  row.PersonaFile,
				GeneClusters: map[int]map[int]bool{},
			}
			byPair[key] = record
			order = append(order, key)
		}
		clusterSet := record.GeneClusters[row.Gene]
		if clusterSet == nil {
			clusterSet = map[int]bool{}
			record.GeneClusters[row.Gene] = clusterSet
		}
		clusterSet[row.Cluster] = true
	}
	records := make([]poolRecord, 0, len(order))
	for _, key := range order {
		records = append(records, *byPair[key])
	}
	return records
}

func availableGenes(records []poolRecord, used map[int]bool) []int {
	seen := map[int]bool{}
	genes := make([]int, 0)
	for _, record := range records {
		for gene := range record.GeneClusters {
			if used[gene] || seen[gene] {
				continue
			}
			seen[gene] = true
			genes = append(genes, gene)
		}
	}
	slices.Sort(genes)
	return genes
}

func availableClusters(records []poolRecord, gene int) []int {
	seen := map[int]bool{}
	clusters := make([]int, 0)
	for _, record := range records {
		clusterSet := record.GeneClusters[gene]
		for cluster := range clusterSet {
			if seen[cluster] {
				continue
			}
			seen[cluster] = true
			clusters = append(clusters, cluster)
		}
	}
	slices.Sort(clusters)
	return clusters
}

func filterPoolRecords(records []poolRecord, gene int, cluster int) []poolRecord {
	filtered := make([]poolRecord, 0, len(records))
	for _, record := range records {
		if record.GeneClusters[gene][cluster] {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func cloneUsedGenes(used map[int]bool) map[int]bool {
	cloned := make(map[int]bool, len(used)+1)
	for gene, present := range used {
		cloned[gene] = present
	}
	return cloned
}
