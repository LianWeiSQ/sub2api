package admin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildBenchmarkBaselineVsLiteLLMComputesNinetyPercentCacheHit(t *testing.T) {
	rows := []cacheBenchmarkRow{}
	for pass := 1; pass <= 10; pass++ {
		rows = append(rows,
			cacheBenchmarkRow{
				Route:         "baseline",
				SampleID:      "short_system_translate",
				Category:      "short_system",
				Cacheable:     true,
				PassIndex:     pass,
				CacheStatus:   "miss",
				InputTokens:   100,
				OutputTokens:  20,
				TotalTokens:   120,
				DurationMs:    50,
				FirstTokenMs:  10,
				UpstreamDelta: 1,
			},
			cacheBenchmarkRow{
				Route:         "litellm",
				SampleID:      "short_system_translate",
				Category:      "short_system",
				Cacheable:     true,
				PassIndex:     pass,
				CacheStatus:   map[bool]string{true: "hit", false: "miss"}[pass > 1],
				InputTokens:   100,
				OutputTokens:  20,
				TotalTokens:   120,
				DurationMs:    20,
				FirstTokenMs:  4,
				UpstreamDelta: map[bool]int{true: 0, false: 1}[pass > 1],
			},
		)
		if pass > 1 {
			rows[len(rows)-1].GatewaySavedInputTokens = 100
			rows[len(rows)-1].GatewaySavedOutputTokens = 20
			rows[len(rows)-1].GatewaySavedTokens = 120
		}
	}

	results, summary := buildBenchmarkBaselineVsLiteLLM(rows, map[string]cacheBenchmarkFixtureSampleMetadata{
		"short_system_translate": {
			Category:    "short_system",
			Description: "short deterministic translation",
		},
	})

	require.Len(t, results, 1)
	require.Equal(t, 1, summary.SampleSize)
	require.Equal(t, 1200, summary.BaselineTokens)
	require.Equal(t, 120, summary.LiteLLMTokens)
	require.Equal(t, 1080, summary.SavedTokens)
	require.Equal(t, 0.9, summary.CacheHitRate)
	require.Equal(t, 0.9, summary.UpstreamCallReduction)
	require.Equal(t, "hit", results[0].GatewayCacheStatus)
	require.Equal(t, 0.9, results[0].GatewayCacheHitRate)
	require.Equal(t, "short deterministic translation", results[0].Scenario)
}
