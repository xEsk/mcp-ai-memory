package search

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"mcp-ai-memory/internal/enrich"
	"mcp-ai-memory/internal/memory"
	"mcp-ai-memory/internal/platform"
	"mcp-ai-memory/internal/project"
	"mcp-ai-memory/internal/storage"
)

//go:embed testdata/retrieval_quality_corpus_v2.json
var retrievalQualityCorpusV1 []byte

const retrievalQualityVersion = "2026-08-24-v2"

type qualityCorpus struct {
	CorpusVersion string           `json:"corpus_version"`
	QueryVersion  string           `json:"query_version"`
	Slices        []string         `json:"slices"`
	Projects      []qualityProject `json:"projects"`
	Memories      []qualityMemory  `json:"memories"`
	Queries       []qualityQuery   `json:"queries"`
	Baseline      qualityBaseline  `json:"baseline"`
}

type qualityProject struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type qualityMemory struct {
	ID         string   `json:"id"`
	Project    string   `json:"project"`
	Type       string   `json:"type"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Tags       []string `json:"tags"`
	Importance int      `json:"importance"`
}

type qualityQuery struct {
	ID        string            `json:"id"`
	Project   string            `json:"project"`
	Text      string            `json:"text"`
	Mode      string            `json:"mode"`
	Slice     string            `json:"slice"`
	Critical  bool              `json:"critical"`
	NoMatch   bool              `json:"no_match"`
	Judgments []qualityJudgment `json:"judgments"`
}

type qualityJudgment struct {
	MemoryID string `json:"memory_id"`
	Grade    int    `json:"grade"`
}

type qualityBaseline struct {
	Global qualityScores            `json:"global"`
	Slices map[string]qualityScores `json:"slices"`
}

type qualityScores struct {
	HitAt1    float64 `json:"hit_at_1"`
	RecallAt5 float64 `json:"recall_at_5"`
	MRRAt10   float64 `json:"mrr_at_10"`
	NDCGAt5   float64 `json:"ndcg_at_5"`
}

type qualitySummary struct {
	qualityScores
	Evaluable          int
	NoMatch            int
	FalsePositives     int
	Leakage            int
	CriticalGrade3Miss int
}

func TestEvaluateQuality_PerfectRanking(t *testing.T) {
	query := qualityQuery{
		ID:      "metric-perfect",
		Project: "atlas-api",
		Judgments: []qualityJudgment{
			{MemoryID: "direct", Grade: 3},
			{MemoryID: "support", Grade: 2},
		},
	}

	got := evaluateQuality(query, []string{"direct", "support"})
	if got.HitAt1 != 1 || got.RecallAt5 != 1 || got.MRRAt10 != 1 || got.NDCGAt5 != 1 {
		t.Fatalf("perfect ranking metrics = %+v, want all 1", got)
	}
}

func TestEvaluateQuality_DegradedRanking(t *testing.T) {
	query := qualityQuery{
		ID:      "metric-degraded",
		Project: "atlas-api",
		Judgments: []qualityJudgment{
			{MemoryID: "direct", Grade: 3},
			{MemoryID: "support", Grade: 2},
		},
	}

	got := evaluateQuality(query, []string{"irrelevant", "support", "direct"})
	if got.HitAt1 != 0 || got.RecallAt5 != 1 || got.MRRAt10 != 0.5 {
		t.Fatalf("degraded ranking metrics = %+v, want Hit@1=0 Recall@5=1 MRR@10=0.5", got)
	}
	if got.NDCGAt5 >= 1 {
		t.Fatalf("degraded nDCG@5 = %v, want less than 1", got.NDCGAt5)
	}
}

func TestRetrievalQualityCorpus_ValidatesBeforeSearch(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*qualityCorpus)
	}{
		{
			name: "missing judgment memory",
			mutate: func(corpus *qualityCorpus) {
				corpus.Queries[0].Judgments[0].MemoryID = "missing-memory"
			},
		},
		{
			name: "invalid judgment grade",
			mutate: func(corpus *qualityCorpus) {
				corpus.Queries[0].Judgments[0].Grade = 4
			},
		},
		{
			name: "unknown query slice",
			mutate: func(corpus *qualityCorpus) {
				corpus.Queries[0].Slice = "unknown"
			},
		},
		{
			name: "cross project judgment",
			mutate: func(corpus *qualityCorpus) {
				corpus.Queries[0].Judgments[0].MemoryID = "ui-route-registry-copy"
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			corpus := loadQualityCorpus(t)
			test.mutate(&corpus)
			if err := validateQualityCorpus(corpus); err == nil {
				t.Fatal("validateQualityCorpus accepted invalid corpus data")
			}
		})
	}
}

func TestRetrievalQuality_VersionedSyntheticGate(t *testing.T) {
	ctx := context.Background()
	corpus := loadQualityCorpus(t)
	if err := validateQualityCorpus(corpus); err != nil {
		t.Fatalf("validate corpus: %v", err)
	}

	db, err := storage.OpenSQLite(ctx, filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatalf("open temporary sqlite: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close sqlite: %v", err)
		}
	})

	clock := fixedQualityClock{now: time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC)}
	projects := project.NewService(project.NewRepository(db), clock)
	memories := memory.NewService(memory.NewRepository(db), projects, enrich.NewLocalHeuristicEnricher(), clock)
	searchService := NewService(NewFTS(db))
	storedProjects := make(map[string]*project.Project, len(corpus.Projects))
	for _, fixture := range corpus.Projects {
		stored, err := projects.Upsert(ctx, project.UpsertInput{Key: fixture.Key, Name: fixture.Name})
		if err != nil {
			t.Fatalf("upsert fixture project %q: %v", fixture.Key, err)
		}
		storedProjects[fixture.Key] = stored
	}
	for _, fixture := range corpus.Memories {
		if _, err := memories.Upsert(ctx, memory.UpsertInput{
			ID: fixture.ID, Project: fixture.Project, Type: fixture.Type, Title: fixture.Title,
			Content: fixture.Content, Tags: fixture.Tags, Importance: fixture.Importance, Enrich: true,
		}); err != nil {
			t.Fatalf("upsert fixture memory %q: %v", fixture.ID, err)
		}
	}

	global := qualitySummary{}
	bySlice := map[string]qualitySummary{}
	byMode := map[string]qualitySummary{}
	for _, query := range corpus.Queries {
		fixtureProject := storedProjects[query.Project]
		results, err := searchService.Search(ctx, fixtureProject.ID, fixtureProject.Key, Input{
			Project: query.Project, Query: query.Text, Mode: query.Mode, Limit: 10,
		})
		if err != nil {
			t.Fatalf("search fixture query %q: %v", query.ID, err)
		}
		ids := make([]string, 0, len(results))
		leakage := 0
		for _, result := range results {
			ids = append(ids, result.Memory.ID)
			if result.Memory.ProjectID != fixtureProject.ID || result.Memory.ProjectKey != fixtureProject.Key {
				leakage++
			}
		}
		score := evaluateQuality(query, ids)
		score.Leakage = leakage
		global.add(score)
		slice := bySlice[query.Slice]
		slice.add(score)
		bySlice[query.Slice] = slice
		mode := byMode[query.Mode]
		mode.add(score)
		byMode[query.Mode] = mode
		t.Logf("query=%s slice=%s mode=%s ids=%v metrics=%s", query.ID, query.Slice, query.Mode, ids, score.report())
	}

	global.finalize()
	for slice, score := range bySlice {
		score.finalize()
		bySlice[slice] = score
	}
	for mode, score := range byMode {
		score.finalize()
		byMode[mode] = score
	}
	t.Logf("quality corpus=%s queries=%s global=%s", corpus.CorpusVersion, corpus.QueryVersion, global.report())
	for _, key := range sortedQualityKeys(bySlice) {
		t.Logf("quality slice=%s %s", key, bySlice[key].report())
	}
	for _, key := range sortedQualityKeys(byMode) {
		t.Logf("quality mode=%s %s", key, byMode[key].report())
	}

	assertQualityGate(t, corpus, global, bySlice)
}

func loadQualityCorpus(t *testing.T) qualityCorpus {
	t.Helper()
	var corpus qualityCorpus
	if err := json.Unmarshal(retrievalQualityCorpusV1, &corpus); err != nil {
		t.Fatalf("decode retrieval quality corpus: %v", err)
	}
	return corpus
}

func validateQualityCorpus(corpus qualityCorpus) error {
	if corpus.CorpusVersion != retrievalQualityVersion || corpus.QueryVersion != retrievalQualityVersion {
		return fmt.Errorf("corpus and query versions must both be %q", retrievalQualityVersion)
	}
	slices := make(map[string]struct{}, len(corpus.Slices))
	for _, slice := range corpus.Slices {
		if slice == "" {
			return fmt.Errorf("empty slice")
		}
		if _, exists := slices[slice]; exists {
			return fmt.Errorf("duplicate slice %q", slice)
		}
		slices[slice] = struct{}{}
	}
	projects := make(map[string]struct{}, len(corpus.Projects))
	for _, fixture := range corpus.Projects {
		if fixture.Key == "" || fixture.Name == "" {
			return fmt.Errorf("project key and name are required")
		}
		if _, exists := projects[fixture.Key]; exists {
			return fmt.Errorf("duplicate project %q", fixture.Key)
		}
		projects[fixture.Key] = struct{}{}
	}
	memories := make(map[string]qualityMemory, len(corpus.Memories))
	for _, fixture := range corpus.Memories {
		if fixture.ID == "" || fixture.Title == "" || fixture.Content == "" || !memory.IsValidType(fixture.Type) {
			return fmt.Errorf("memory %q has invalid required fields", fixture.ID)
		}
		if _, exists := projects[fixture.Project]; !exists {
			return fmt.Errorf("memory %q references unknown project %q", fixture.ID, fixture.Project)
		}
		if _, exists := memories[fixture.ID]; exists {
			return fmt.Errorf("duplicate memory ID %q", fixture.ID)
		}
		memories[fixture.ID] = fixture
	}
	evaluableBySlice := make(map[string]int, len(slices))
	queries := make(map[string]struct{}, len(corpus.Queries))
	for _, query := range corpus.Queries {
		if query.ID == "" || strings.TrimSpace(query.Text) == "" {
			return fmt.Errorf("query has empty ID or text")
		}
		if _, exists := queries[query.ID]; exists {
			return fmt.Errorf("duplicate query ID %q", query.ID)
		}
		queries[query.ID] = struct{}{}
		if _, exists := projects[query.Project]; !exists {
			return fmt.Errorf("query %q references unknown project %q", query.ID, query.Project)
		}
		if _, exists := slices[query.Slice]; !exists {
			return fmt.Errorf("query %q references unknown slice %q", query.ID, query.Slice)
		}
		if _, err := normalizeMode(query.Mode); err != nil {
			return fmt.Errorf("query %q has invalid mode: %w", query.ID, err)
		}
		if query.NoMatch && len(query.Judgments) != 0 {
			return fmt.Errorf("no-match query %q has judgments", query.ID)
		}
		if !query.NoMatch && len(query.Judgments) == 0 {
			return fmt.Errorf("evaluable query %q has no judgments", query.ID)
		}
		gradeThree := false
		hasRelevant := false
		judged := map[string]struct{}{}
		for _, judgment := range query.Judgments {
			fixture, exists := memories[judgment.MemoryID]
			if !exists {
				return fmt.Errorf("query %q references missing memory %q", query.ID, judgment.MemoryID)
			}
			if fixture.Project != query.Project {
				return fmt.Errorf("query %q judgment %q crosses projects", query.ID, judgment.MemoryID)
			}
			if judgment.Grade < 0 || judgment.Grade > 3 {
				return fmt.Errorf("query %q judgment %q has invalid grade %d", query.ID, judgment.MemoryID, judgment.Grade)
			}
			if _, exists := judged[judgment.MemoryID]; exists {
				return fmt.Errorf("query %q duplicates judgment %q", query.ID, judgment.MemoryID)
			}
			judged[judgment.MemoryID] = struct{}{}
			gradeThree = gradeThree || judgment.Grade == 3
			hasRelevant = hasRelevant || judgment.Grade >= 2
		}
		if !query.NoMatch && !hasRelevant {
			return fmt.Errorf("query %q has no grade >=2 judgment", query.ID)
		}
		if query.Critical && !query.NoMatch && !gradeThree {
			return fmt.Errorf("critical query %q has no grade-3 judgment", query.ID)
		}
		if hasRelevant {
			evaluableBySlice[query.Slice]++
		}
	}
	for slice := range slices {
		if slice == "no_match" {
			continue
		}
		if evaluableBySlice[slice] == 0 {
			return fmt.Errorf("slice %q has zero evaluable queries", slice)
		}
	}
	if err := validateBaseline(corpus.Baseline, slices); err != nil {
		return err
	}
	return nil
}

func validateBaseline(baseline qualityBaseline, slices map[string]struct{}) error {
	if err := validateQualityScores(baseline.Global); err != nil {
		return fmt.Errorf("invalid global baseline: %w", err)
	}
	for slice := range slices {
		scores, exists := baseline.Slices[slice]
		if !exists {
			return fmt.Errorf("missing baseline for slice %q", slice)
		}
		if err := validateQualityScores(scores); err != nil {
			return fmt.Errorf("invalid baseline for slice %q: %w", slice, err)
		}
	}
	for slice := range baseline.Slices {
		if _, exists := slices[slice]; !exists {
			return fmt.Errorf("baseline references unknown slice %q", slice)
		}
	}
	return nil
}

func validateQualityScores(scores qualityScores) error {
	for _, metric := range []struct {
		name  string
		value float64
	}{
		{"Hit@1", scores.HitAt1},
		{"Recall@5", scores.RecallAt5},
		{"MRR@10", scores.MRRAt10},
		{"nDCG@5", scores.NDCGAt5},
	} {
		if metric.value < 0 || metric.value > 1 {
			return fmt.Errorf("%s is outside [0, 1]", metric.name)
		}
	}
	return nil
}

func evaluateQuality(query qualityQuery, ids []string) qualitySummary {
	summary := qualitySummary{}
	if query.NoMatch {
		summary.NoMatch = 1
		if len(ids) > 0 {
			summary.FalsePositives = 1
		}
		return summary
	}
	grades := make(map[string]int, len(query.Judgments))
	relevant := 0
	for _, judgment := range query.Judgments {
		grades[judgment.MemoryID] = judgment.Grade
		if judgment.Grade >= 2 {
			relevant++
		}
	}
	summary.Evaluable = 1
	if len(ids) > 0 && grades[ids[0]] >= 2 {
		summary.HitAt1 = 1
	}
	for rank, id := range ids {
		position := rank + 1
		grade := grades[id]
		if position <= 5 && grade >= 2 {
			summary.RecallAt5++
		}
		if position <= 10 && summary.MRRAt10 == 0 && grade >= 2 {
			summary.MRRAt10 = 1 / float64(position)
		}
		if position <= 5 {
			summary.NDCGAt5 += (math.Pow(2, float64(grade)) - 1) / math.Log2(float64(position)+1)
		}
	}
	summary.RecallAt5 /= float64(relevant)
	ideal := make([]int, 0, len(query.Judgments))
	for _, judgment := range query.Judgments {
		ideal = append(ideal, judgment.Grade)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ideal)))
	idealDCG := 0.0
	for rank, grade := range ideal {
		if rank == 5 {
			break
		}
		idealDCG += (math.Pow(2, float64(grade)) - 1) / math.Log2(float64(rank)+2)
	}
	if idealDCG > 0 {
		summary.NDCGAt5 /= idealDCG
	}
	if query.Critical {
		for rank, id := range ids {
			if rank == 5 {
				break
			}
			if grades[id] == 3 {
				return summary
			}
		}
		summary.CriticalGrade3Miss = 1
	}
	return summary
}

func (summary *qualitySummary) add(other qualitySummary) {
	summary.HitAt1 += other.HitAt1
	summary.RecallAt5 += other.RecallAt5
	summary.MRRAt10 += other.MRRAt10
	summary.NDCGAt5 += other.NDCGAt5
	summary.Evaluable += other.Evaluable
	summary.NoMatch += other.NoMatch
	summary.FalsePositives += other.FalsePositives
	summary.Leakage += other.Leakage
	summary.CriticalGrade3Miss += other.CriticalGrade3Miss
}

func (summary *qualitySummary) finalize() {
	if summary.Evaluable == 0 {
		return
	}
	denominator := float64(summary.Evaluable)
	summary.HitAt1 /= denominator
	summary.RecallAt5 /= denominator
	summary.MRRAt10 /= denominator
	summary.NDCGAt5 /= denominator
}

func (summary qualitySummary) report() string {
	return fmt.Sprintf("Hit@1=%.2f Recall@5=%.2f MRR@10=%.2f nDCG@5=%.2f evaluable=%d no_match=%d false_positive=%d leakage=%d critical_grade3_miss=%d", summary.HitAt1, summary.RecallAt5, summary.MRRAt10, summary.NDCGAt5, summary.Evaluable, summary.NoMatch, summary.FalsePositives, summary.Leakage, summary.CriticalGrade3Miss)
}

func assertQualityGate(t *testing.T, corpus qualityCorpus, global qualitySummary, bySlice map[string]qualitySummary) {
	t.Helper()
	if global.Leakage != 0 {
		t.Errorf("project leakage = %d, want 0", global.Leakage)
	}
	if global.FalsePositives != 0 {
		t.Errorf("no-match false positives = %d, want 0", global.FalsePositives)
	}
	if global.CriticalGrade3Miss != 0 {
		t.Errorf("critical grade-3 targets outside top 5 = %d, want 0", global.CriticalGrade3Miss)
	}
	if global.RecallAt5 < 0.90 || global.MRRAt10 < 0.80 || global.NDCGAt5 < 0.80 {
		t.Errorf("global thresholds failed: %s", global.report())
	}
	identifier := bySlice["identifier_path"]
	if identifier.HitAt1 != 1 {
		t.Errorf("identifier/path Hit@1 = %.2f, want 1.00", identifier.HitAt1)
	}
	shortTechnical := bySlice["short_technical"]
	if shortTechnical.HitAt1 != 1 {
		t.Errorf("short-technical Hit@1 = %.2f, want 1.00", shortTechnical.HitAt1)
	}
	if hardNegative := bySlice["heuristic_hard_negative"]; hardNegative.FalsePositives != 0 {
		t.Errorf("heuristic hard-negative false positives = %d, want 0", hardNegative.FalsePositives)
	}
	for slice, score := range bySlice {
		if slice == "no_match" {
			continue
		}
		if score.RecallAt5 < 0.80 {
			t.Errorf("slice %q Recall@5 = %.2f, want >= 0.80", slice, score.RecallAt5)
		}
		baseline, exists := corpus.Baseline.Slices[slice]
		if !exists {
			t.Errorf("missing frozen baseline for slice %q", slice)
			continue
		}
		if score.RecallAt5 < baseline.RecallAt5-0.05 {
			t.Errorf("slice %q Recall@5 regressed from %.2f to %.2f (limit 0.05)", slice, baseline.RecallAt5, score.RecallAt5)
		}
	}
	assertGlobalRegression(t, corpus.Baseline.Global, global.qualityScores)
}

func assertGlobalRegression(t *testing.T, baseline, current qualityScores) {
	t.Helper()
	for _, metric := range []struct {
		name     string
		baseline float64
		current  float64
	}{
		{"Hit@1", baseline.HitAt1, current.HitAt1},
		{"Recall@5", baseline.RecallAt5, current.RecallAt5},
		{"MRR@10", baseline.MRRAt10, current.MRRAt10},
		{"nDCG@5", baseline.NDCGAt5, current.NDCGAt5},
	} {
		if metric.current < metric.baseline-0.02 {
			t.Errorf("global %s regressed from %.2f to %.2f (limit 0.02)", metric.name, metric.baseline, metric.current)
		}
	}
}

func sortedQualityKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

type fixedQualityClock struct{ now time.Time }

func (clock fixedQualityClock) Now() time.Time { return clock.now }

var _ platform.Clock = fixedQualityClock{}
