package sports

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

func TestCollectGamesWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock GetGames activity to return test games
	testGames := []Game{
		{
			ID:        "game-1",
			StartTime: time.Now().Add(time.Hour),
			Status:    "pre",
			HomeTeam: Team{
				ID:          "130",
				DisplayName: "Michigan Wolverines",
			},
			AwayTeam: Team{
				ID:          "264",
				DisplayName: "Washington Huskies",
			},
		},
		{
			ID:        "game-2",
			StartTime: time.Now().Add(2 * time.Hour),
			Status:    "pre",
			HomeTeam: Team{
				ID:          "194",
				DisplayName: "Northwestern Wildcats",
			},
			AwayTeam: Team{
				ID:          "275",
				DisplayName: "Wisconsin Badgers",
			},
		},
	}

	env.OnActivity(GetGamesActivity, mock.Anything).Return(testGames, nil)
	env.OnActivity(StartGameWorkflowActivity, mock.Anything).Return(nil)

	// Create tracking request
	trackingRequest := TrackingRequest{
		Sport:       "football",
		League:      "college-football",
		Teams:       []string{"130", "264"},
		Conferences: []string{"5"},
	}

	// Execute workflow
	env.ExecuteWorkflow(CollectGamesWorkflow, trackingRequest)

	// Verify workflow completed
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify activities were called
	env.AssertExpectations(t)
}

func TestCollectGamesWorkflow_NoGames(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock GetGamesActivity to return empty slice
	env.OnActivity(GetGamesActivity, mock.Anything).Return([]Game{}, nil)

	trackingRequest := TrackingRequest{
		Sport:       "football",
		League:      "college-football",
		Teams:       []string{"130"},
		Conferences: []string{"5"},
	}

	// Execute workflow
	env.ExecuteWorkflow(CollectGamesWorkflow, trackingRequest)

	// Verify workflow completed
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// StartGameWorkflow should not be called since no games
	env.AssertExpectations(t)
}

func TestCollectGamesWorkflow_GetGamesFailure(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock GetGamesActivity to fail
	env.OnActivity(GetGamesActivity, mock.Anything).Return(nil, assert.AnError)

	trackingRequest := TrackingRequest{
		Sport:       "football",
		League:      "college-football",
		Teams:       []string{"130"},
		Conferences: []string{"5"},
	}

	// Execute workflow
	env.ExecuteWorkflow(CollectGamesWorkflow, trackingRequest)

	// Verify workflow failed
	assert.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestCollectGamesWorkflow_StartGameWorkflowFailure(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock GetGames to return one game
	testGame := Game{
		ID:        "game-1",
		StartTime: time.Now().Add(time.Hour),
		Status:    "pre",
		HomeTeam: Team{
			ID:          "130",
			DisplayName: "Michigan Wolverines",
		},
		AwayTeam: Team{
			ID:          "264",
			DisplayName: "Washington Huskies",
		},
	}

	env.OnActivity(GetGamesActivity, mock.Anything).Return([]Game{testGame}, nil)
	env.OnActivity(StartGameWorkflowActivity, mock.Anything).Return(assert.AnError)

	trackingRequest := TrackingRequest{
		Sport:       "football",
		League:      "college-football",
		Teams:       []string{"130"},
		Conferences: []string{"5"},
	}

	// Execute workflow
	env.ExecuteWorkflow(CollectGamesWorkflow, trackingRequest)

	// Verify workflow failed due to StartGameWorkflow failure
	assert.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestCollectGamesWorkflow_FiltersPastGames(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock games with mixed statuses and times
	testGames := []Game{
		{
			ID:        "game-past",
			StartTime: time.Now().Add(-time.Hour), // Past game
			Status:    "final",
			HomeTeam: Team{
				ID:          "130",
				DisplayName: "Michigan Wolverines",
			},
			AwayTeam: Team{
				ID:          "264",
				DisplayName: "Washington Huskies",
			},
		},
		{
			ID:        "game-in-progress",
			StartTime: time.Now().Add(-30 * time.Minute), // Started but in progress
			Status:    "in",
			HomeTeam: Team{
				ID:          "194",
				DisplayName: "Northwestern Wildcats",
			},
			AwayTeam: Team{
				ID:          "275",
				DisplayName: "Wisconsin Badgers",
			},
		},
		{
			ID:        "game-future",
			StartTime: time.Now().Add(time.Hour), // Future game
			Status:    "pre",
			HomeTeam: Team{
				ID:          "213",
				DisplayName: "Minnesota Golden Gophers",
			},
			AwayTeam: Team{
				ID:          "356",
				DisplayName: "Iowa Hawkeyes",
			},
		},
	}

	env.OnActivity(GetGamesActivity, mock.Anything).Return(testGames, nil)
	// Only the future game should trigger StartGameWorkflowActivity
	env.OnActivity(StartGameWorkflowActivity, mock.MatchedBy(func(game Game) bool {
		return game.ID == "game-future"
	})).Return(nil).Once()

	trackingRequest := TrackingRequest{
		Sport:       "football",
		League:      "college-football",
		Conferences: []string{"5"},
	}

	// Execute workflow
	env.ExecuteWorkflow(CollectGamesWorkflow, trackingRequest)

	// Verify workflow completed
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify only one StartGameWorkflowActivity was called (for the future game)
	env.AssertExpectations(t)
}

func TestCollectGamesWorkflow_MultipleTeams(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock multiple games
	testGames := []Game{
		{
			ID:        "game-1",
			StartTime: time.Now().Add(time.Hour),
			Status:    "pre",
			HomeTeam: Team{
				ID:          "130",
				DisplayName: "Michigan Wolverines",
			},
			AwayTeam: Team{
				ID:          "264",
				DisplayName: "Washington Huskies",
			},
		},
		{
			ID:        "game-2",
			StartTime: time.Now().Add(2 * time.Hour),
			Status:    "pre",
			HomeTeam: Team{
				ID:          "194",
				DisplayName: "Northwestern Wildcats",
			},
			AwayTeam: Team{
				ID:          "275",
				DisplayName: "Wisconsin Badgers",
			},
		},
		{
			ID:        "game-3",
			StartTime: time.Now().Add(3 * time.Hour),
			Status:    "pre",
			HomeTeam: Team{
				ID:          "213",
				DisplayName: "Minnesota Golden Gophers",
			},
			AwayTeam: Team{
				ID:          "356",
				DisplayName: "Iowa Hawkeyes",
			},
		},
	}

	env.OnActivity(GetGamesActivity, mock.Anything).Return(testGames, nil)
	env.OnActivity(StartGameWorkflowActivity, mock.Anything).Return(nil).Times(3)

	trackingRequest := TrackingRequest{
		Sport:       "football",
		League:      "college-football",
		Teams:       []string{"130", "194", "213"},
		Conferences: []string{"5"},
	}

	// Execute workflow
	env.ExecuteWorkflow(CollectGamesWorkflow, trackingRequest)

	// Verify workflow completed
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify all games triggered StartGameWorkflowActivity
	env.AssertExpectations(t)
}

func TestCollectGamesWorkflow_EmptyTrackingRequest(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock GetGamesActivity to return empty slice
	env.OnActivity(GetGamesActivity, mock.Anything).Return([]Game{}, nil)

	// Empty tracking request
	trackingRequest := TrackingRequest{
		Sport:       "football",
		League:      "college-football",
		Teams:       []string{},
		Conferences: []string{},
	}

	// Execute workflow
	env.ExecuteWorkflow(CollectGamesWorkflow, trackingRequest)

	// Verify workflow completed
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify GetGamesActivity was still called
	env.AssertExpectations(t)
}

func TestCollectGamesWorkflow_DifferentSports(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}

	testCases := []struct {
		name            string
		trackingRequest TrackingRequest
	}{
		{
			name: "basketball",
			trackingRequest: TrackingRequest{
				Sport:       "basketball",
				League:      "mens-college-basketball",
				Conferences: []string{"5"},
			},
		},
		{
			name: "football",
			trackingRequest: TrackingRequest{
				Sport:       "football",
				League:      "college-football",
				Conferences: []string{"5"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := testSuite.NewTestWorkflowEnvironment()
			
			env.OnActivity(GetGamesActivity, mock.Anything).Return([]Game{}, nil)

			// Execute workflow
			env.ExecuteWorkflow(CollectGamesWorkflow, tc.trackingRequest)

			// Verify workflow completed
			assert.True(t, env.IsWorkflowCompleted())
			assert.NoError(t, env.GetWorkflowError())
		})
	}
}

// Benchmark test for collect games workflow
func BenchmarkCollectGamesWorkflow(b *testing.B) {
	testSuite := &testsuite.WorkflowTestSuite{}

	trackingRequest := TrackingRequest{
		Sport:       "football",
		League:      "college-football",
		Conferences: []string{"5"},
	}

	testGames := []Game{
		{
			ID:        "benchmark-game",
			StartTime: time.Now().Add(time.Hour),
			Status:    "pre",
			HomeTeam: Team{
				ID:          "130",
				DisplayName: "Michigan Wolverines",
			},
			AwayTeam: Team{
				ID:          "264",
				DisplayName: "Washington Huskies",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env := testSuite.NewTestWorkflowEnvironment()
		env.OnActivity(GetGamesActivity, mock.Anything).Return(testGames, nil)
		env.OnActivity(StartGameWorkflowActivity, mock.Anything).Return(nil)

		env.ExecuteWorkflow(CollectGamesWorkflow, trackingRequest)
	}
}
