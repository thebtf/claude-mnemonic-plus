package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type RRFSuite struct {
	suite.Suite
}

func TestRRFSuite(t *testing.T) {
	suite.Run(t, new(RRFSuite))
}

func (s *RRFSuite) SetupTest() {}

type scoredIDKey struct {
	docType string
	id      int64
}

func expectedRRFContribution(listIndex int, rank int) float64 {
	weight := 1.0
	if listIndex < 2 {
		weight = 2.0
	}

	rankBonus := 0.0
	if rank == 0 {
		rankBonus = 0.05
	} else if rank <= 2 {
		rankBonus = 0.02
	}

	return weight/(60.0+float64(rank)+1.0) + rankBonus
}

func findResultScore(result []ScoredID, key scoredIDKey) (float64, bool) {
	for _, item := range result {
		if item.DocType == key.docType && item.ID == key.id {
			return item.Score, true
		}
	}
	return 0, false
}

func (s *RRFSuite) TestBM25Normalize_ShouldNormalizeInputValues() {
	tests := []struct {
		name     string
		score    float64
		expected float64
	}{
		{name: "zero", score: 0, expected: 0},
		{name: "positive one", score: 1.0, expected: 0.5},
		{name: "negative one", score: -1.0, expected: 0.5},
		{name: "positive half", score: 0.5, expected: 1.0 / 3.0},
		{name: "large value", score: 99, expected: 99.0 / 100.0},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			assert.InDelta(s.T(), tt.expected, BM25Normalize(tt.score), 1e-12)
		})
	}
}

func (s *RRFSuite) TestRRF_EmptyInput_ReturnsEmptyResult() {
	assert.Empty(s.T(), RRF())
}

func (s *RRFSuite) TestRRF_SingleList_ContributionsAndSorting() {
	input := []ScoredID{
		{DocType: "observation", ID: 1},
		{DocType: "observation", ID: 2},
	}
	result := RRF(input)

	assert.Len(s.T(), result, 2)
	assert.Greater(s.T(), result[0].Score, result[1].Score)

	rank0Score, ok := findResultScore(result, scoredIDKey{docType: "observation", id: 1})
	assert.True(s.T(), ok)
	assert.InDelta(s.T(), expectedRRFContribution(0, 0), rank0Score, 1e-12)

	rank1Score, ok := findResultScore(result, scoredIDKey{docType: "observation", id: 2})
	assert.True(s.T(), ok)
	assert.InDelta(s.T(), expectedRRFContribution(0, 1), rank1Score, 1e-12)
}

func (s *RRFSuite) TestRRF_TwoLists_DeduplicateByIDAndDocTypeAndAccumulateScore() {
	result := RRF(
		[]ScoredID{
			{DocType: "observation", ID: 1},
			{DocType: "summary", ID: 2},
		},
		[]ScoredID{
			{DocType: "observation", ID: 1},
			{DocType: "summary", ID: 3},
		},
	)

	assert.Len(s.T(), result, 3)

	score1, ok := findResultScore(result, scoredIDKey{docType: "observation", id: 1})
	assert.True(s.T(), ok)
	assert.InDelta(s.T(), expectedRRFContribution(0, 0)+expectedRRFContribution(1, 0), score1, 1e-12)

	score2, ok := findResultScore(result, scoredIDKey{docType: "summary", id: 2})
	assert.True(s.T(), ok)
	assert.InDelta(s.T(), expectedRRFContribution(0, 1), score2, 1e-12)

	score3, ok := findResultScore(result, scoredIDKey{docType: "summary", id: 3})
	assert.True(s.T(), ok)
	assert.InDelta(s.T(), expectedRRFContribution(1, 1), score3, 1e-12)
}

func (s *RRFSuite) TestRRF_ThirdList_UsesSingleWeight() {
	result := RRF(
		[]ScoredID{
			{DocType: "observation", ID: 1},
		},
		[]ScoredID{
			{DocType: "summary", ID: 2},
		},
		[]ScoredID{
			{DocType: "observation", ID: 1},
		},
	)

	assert.Len(s.T(), result, 2)

	weight3, ok := findResultScore(result, scoredIDKey{docType: "observation", id: 1})
	assert.True(s.T(), ok)
	assert.InDelta(
		s.T(),
		expectedRRFContribution(0, 0)+expectedRRFContribution(2, 0),
		weight3,
		1e-12,
	)
}

func (s *RRFSuite) TestRRF_DifferentDocTypePairsAreNotDeduplicated() {
	result := RRF(
		[]ScoredID{
			{DocType: "observation", ID: 1},
		},
		[]ScoredID{
			{DocType: "summary", ID: 1},
		},
	)

	assert.Len(s.T(), result, 2)
	_, foundObs := findResultScore(result, scoredIDKey{docType: "observation", id: 1})
	_, foundSummary := findResultScore(result, scoredIDKey{docType: "summary", id: 1})
	assert.True(s.T(), foundObs)
	assert.True(s.T(), foundSummary)
}

func (s *RRFSuite) TestRRF_RankBonusByRank() {
	result := RRF([]ScoredID{
		{DocType: "observation", ID: 1},
		{DocType: "observation", ID: 2},
		{DocType: "observation", ID: 3},
		{DocType: "observation", ID: 4},
	})

	tests := []struct {
		name     string
		rank     int
		expected float64
	}{
		{name: "rank 0", rank: 0, expected: expectedRRFContribution(0, 0)},
		{name: "rank 1", rank: 1, expected: expectedRRFContribution(0, 1)},
		{name: "rank 2", rank: 2, expected: expectedRRFContribution(0, 2)},
		{name: "rank 3", rank: 3, expected: expectedRRFContribution(0, 3)},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			score, found := findResultScore(result, scoredIDKey{docType: "observation", id: int64(tt.rank + 1)})
			assert.True(s.T(), found)
			assert.InDelta(s.T(), tt.expected, score, 1e-12)
		})
	}

	for i := 0; i < len(result)-1; i++ {
		assert.Greater(s.T(), result[i].Score, result[i+1].Score)
	}
}
