package main

import (
	"math"
	"testing"

	"github.com/docker/docker/api/types/container"
)

// TestCalculateCPUPercentWithHistory tests CPU percentage calculation
func TestCalculateCPUPercentWithHistory(t *testing.T) {
	tests := []struct {
		name     string
		current  *container.StatsResponse
		previous *container.StatsResponse
		want     float64
		wantApprox bool // For tests where exact value varies
	}{
		{
			name:     "nil current",
			current:  nil,
			previous: &container.StatsResponse{},
			want:     0.0,
		},
		{
			name:     "nil previous",
			current:  &container.StatsResponse{},
			previous: nil,
			want:     0.0,
		},
		{
			name: "zero total usage",
			current: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 0,
					},
				},
			},
			previous: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 1000000,
					},
				},
			},
			want: 0.0,
		},
		{
			name: "typical usage - 50% on 1 CPU",
			current: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 10000000000, // 10 billion nanoseconds total
					},
					SystemUsage: 20000000000, // 20 billion nanoseconds system
					OnlineCPUs:  1,
				},
			},
			previous: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 5000000000, // 5 billion nanoseconds previous
					},
					SystemUsage: 10000000000, // 10 billion nanoseconds previous system
				},
			},
			want: 50.0, // (10-5)/(20-10) * 1 * 100 = 5/10 * 100 = 50%
		},
		{
			name: "full usage - 100% on 1 CPU",
			current: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 20000000000, // 20 billion total
					},
					SystemUsage: 20000000000, // 20 billion system
					OnlineCPUs:  1,
				},
			},
			previous: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 10000000000, // 10 billion previous
					},
					SystemUsage: 10000000000, // 10 billion previous system
				},
			},
			want: 100.0, // (20-10)/(20-10) * 1 * 100 = 10/10 * 100 = 100%
		},
		{
			name: "multi-core - 200% on 4 CPUs",
			current: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 40000000000, // 40 billion total
					},
					SystemUsage: 80000000000, // 80 billion system (4 CPUs)
					OnlineCPUs:  4,
				},
			},
			previous: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 20000000000, // 20 billion previous
					},
					SystemUsage: 40000000000, // 40 billion previous system
				},
			},
			want: 200.0, // (40-20)/(80-40) * 4 * 100 = 20/40 * 4 * 100 = 0.5 * 4 * 100 = 200%
		},
		{
			name: "zero system delta",
			current: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 5000000000,
					},
					SystemUsage: 10000000000,
					OnlineCPUs:  1,
				},
			},
			previous: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 5000000000,
					},
					SystemUsage: 10000000000, // Same as current - no delta
				},
			},
			want: 0.0,
		},
		{
			name: "negative cpu delta (time went backwards - uint64 underflow)",
			current: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 1000000000, // Current is lower
					},
					SystemUsage: 20000000000,
					OnlineCPUs:  1,
				},
			},
			previous: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 5000000000, // Previous was higher
					},
					SystemUsage: 10000000000,
				},
			},
			want: 999.0, // uint64 underflow creates huge number, capped to 999%
		},
		{
			name: "OnlineCPUs zero - fallback to PercpuUsage",
			current: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage:  20000000000, // 20 billion total
						PercpuUsage: []uint64{1, 2, 3, 4}, // 4 CPUs
					},
					SystemUsage: 40000000000, // 40 billion system
					OnlineCPUs:  0, // Not set, will use len(PercpuUsage) = 4
				},
			},
			previous: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 10000000000, // 10 billion previous
					},
					SystemUsage: 20000000000, // 20 billion previous system
				},
			},
			want: 200.0, // (20-10)/(40-20) * 4 * 100 = 10/20 * 4 * 100 = 0.5 * 4 * 100 = 200%
		},
		{
			name: "very high CPU - capped at 999%",
			current: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 120000000000, // 120 billion (very high)
					},
					SystemUsage: 20000000000, // 20 billion system
					OnlineCPUs:  1,
				},
			},
			previous: &container.StatsResponse{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 10000000000, // 10 billion previous
					},
					SystemUsage: 10000000000, // 10 billion previous system
				},
			},
			want: 999.0, // (120-10)/(20-10) * 1 * 100 = 110/10 * 100 = 1100% -> capped to 999%
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateCPUPercentWithHistory(tt.current, tt.previous)

			if tt.wantApprox {
				// For approximate comparisons (within 0.1%)
				if math.Abs(got-tt.want) > 0.1 {
					t.Errorf("calculateCPUPercentWithHistory() = %v, want approximately %v", got, tt.want)
				}
			} else {
				// Exact comparison
				if got != tt.want {
					t.Errorf("calculateCPUPercentWithHistory() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// TestCalculateCPUFormula verifies the Docker CPU formula is correct
func TestCalculateCPUFormula(t *testing.T) {
	// Test the formula: (cpuDelta / systemDelta) * numCPUs * 100
	//
	// Scenario: Container used 2.5 billion nanoseconds of CPU time during measurement
	// System had 10 billion nanoseconds of total CPU time across 4 CPUs during same period
	// That means each CPU had 2.5 billion nanoseconds of time available
	//
	// Container used 2.5B out of 2.5B per-CPU available = 100% of one core
	// Result should be: (2.5B / 10B) * 4 * 100 = 0.25 * 4 * 100 = 100%

	current := &container.StatsResponse{
		CPUStats: container.CPUStats{
			CPUUsage: container.CPUUsage{
				TotalUsage: 5000000000, // 5 billion total (delta will be 2.5B)
			},
			SystemUsage: 20000000000, // 20 billion system (delta will be 10B)
			OnlineCPUs:  4,
		},
	}

	previous := &container.StatsResponse{
		CPUStats: container.CPUStats{
			CPUUsage: container.CPUUsage{
				TotalUsage: 2500000000, // 2.5 billion previous
			},
			SystemUsage: 10000000000, // 10 billion previous system
		},
	}

	got := calculateCPUPercentWithHistory(current, previous)
	want := 100.0

	if got != want {
		t.Errorf("calculateCPUPercentWithHistory() = %v, want %v", got, want)
	}
}

// BenchmarkCalculateCPUPercent benchmarks CPU calculation performance
func BenchmarkCalculateCPUPercent(b *testing.B) {
	current := &container.StatsResponse{
		CPUStats: container.CPUStats{
			CPUUsage: container.CPUUsage{
				TotalUsage: 5000000000,
			},
			SystemUsage: 10000000000,
			OnlineCPUs:  4,
		},
	}

	previous := &container.StatsResponse{
		CPUStats: container.CPUStats{
			CPUUsage: container.CPUUsage{
				TotalUsage: 0,
			},
			SystemUsage: 0,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculateCPUPercentWithHistory(current, previous)
	}
}
