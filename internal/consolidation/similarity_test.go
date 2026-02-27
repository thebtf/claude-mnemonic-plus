package consolidation

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type SimilaritySuite struct {
	suite.Suite
}

func TestSimilaritySuite(t *testing.T) {
	suite.Run(t, new(SimilaritySuite))
}

func (s *SimilaritySuite) SetupTest() {}

func (s *SimilaritySuite) TestCosineSimilarity_TableDrivenCases() {
	tests := []struct {
		name      string
		a         []float32
		b         []float32
		expected  float64
		tolerance float64
	}{
		{
			name:      "identical vectors",
			a:         []float32{1, 2, 3},
			b:         []float32{1, 2, 3},
			expected:  1.0,
			tolerance: 1e-9,
		},
		{
			name:      "opposite vectors",
			a:         []float32{1, 2, 3},
			b:         []float32{-1, -2, -3},
			expected:  -1.0,
			tolerance: 1e-9,
		},
		{
			name:      "orthogonal vectors",
			a:         []float32{1, 0},
			b:         []float32{0, 1},
			expected:  0.0,
			tolerance: 1e-9,
		},
		{
			name:      "different lengths",
			a:         []float32{1, 2, 3},
			b:         []float32{1, 2},
			expected:  0.0,
			tolerance: 1e-9,
		},
		{
			name:      "empty slices",
			a:         []float32{},
			b:         []float32{},
			expected:  0.0,
			tolerance: 1e-9,
		},
		{
			name:      "zero vector",
			a:         []float32{0, 0, 0},
			b:         []float32{1, 2, 3},
			expected:  0.0,
			tolerance: 1e-9,
		},
		{
			name:      "known numeric",
			a:         []float32{1, 2, 3},
			b:         []float32{4, 5, 6},
			expected:  32.0 / math.Sqrt(float64(1078)),
			tolerance: 1e-9,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			assert.InDelta(s.T(), tt.expected, CosineSimilarity(tt.a, tt.b), tt.tolerance)
		})
	}
}
