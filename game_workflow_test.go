package sports

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

func TestGameWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock activities
	env.OnActivity(GetGameScore, mock.Anything).Return(map[string]string{
		"130": "0",
		"264": "0",
	}, nil)
	env.OnActivity(SendNotification, mock.Anything).Return(nil)

	// Create test game
	game := Game{
		ID:        "test-game-1",
		StartTime: time.Now().Add(-time.Hour), // Game started 1 hour ago
		Status:    "in",
		Odds:      "MICH -7.5",
		CurrentScore: map[string]string{
			"130": "0",
			"264": "0",
		},
		HomeTeam: Team{
			ID:          "130",
			Name:        "Wolverines",
			DisplayName: "Michigan Wolverines",
			ConferenceId: "5",
			Favorite:    true,
		},
		AwayTeam: Team{
			ID:          "264",
			Name:        "Huskies",
			DisplayName: "Washington Huskies",
			ConferenceId: "9",
			Underdog:    true,
		},
	}

	// Execute workflow
	env.ExecuteWorkflow(GameWorkflow, game)

	// Verify workflow completed
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify the result
	var result string
	err := env.GetWorkflowResult(&result)
	assert.NoError(t, err)
	assert.Contains(t, result, "Final score:")
}

func TestGameWorkflow_ScoreChange(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock activities with score changes
	callCount := 0
	env.OnActivity(GetGameScore, mock.Anything).Return(func(gameID string) (map[string]string, error) {
		callCount++
		if callCount == 1 {
			return map[string]string{"130": "0", "264": "0"}, nil
		}
		return map[string]string{"130": "7", "264": "0"}, nil
	})

	env.OnActivity(SendNotification, mock.Anything).Return(nil)

	// Create test game that starts in the past
	game := Game{
		ID:        "test-game-1",
		StartTime: time.Now().Add(-30 * time.Minute),
		Status:    "in",
		CurrentScore: map[string]string{
			"130": "0",
			"264": "0",
		},
		HomeTeam: Team{
			ID:          "130",
			DisplayName: "Michigan Wolverines",
		},
		AwayTeam: Team{
			ID:          "264",
			DisplayName: "Washington Huskies",
		},
	}

	// Set up timer to advance time
	env.RegisterDelayedCallback(func() {
		// Timer callback - time will advance automatically
	}, time.Minute)

	// Execute workflow
	env.ExecuteWorkflow(GameWorkflow, game)

	// Verify workflow completed
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify activities were called
	env.AssertExpectations(t)
}

func TestGameWorkflow_FutureGame(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock activities
	env.OnActivity(GetGameScore, mock.Anything).Return(map[string]string{
		"130": "0",
		"264": "0",
	}, nil)

	// Create test game that starts in the future
	futureTime := time.Now().Add(2 * time.Hour)
	game := Game{
		ID:        "test-game-future",
		StartTime: futureTime,
		Status:    "pre",
		CurrentScore: map[string]string{
			"130": "0",
			"264": "0",
		},
		HomeTeam: Team{
			ID:          "130",
			DisplayName: "Michigan Wolverines",
		},
		AwayTeam: Team{
			ID:          "264",
			DisplayName: "Washington Huskies",
		},
	}

	// Set up callback to advance time to game start
	env.RegisterDelayedCallback(func() {
		// Timer callback - time will advance automatically
	}, 2*time.Hour+time.Minute)

	// Execute workflow
	env.ExecuteWorkflow(GameWorkflow, game)

	// Verify workflow completed
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

func TestGameWorkflow_QueryHandler(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock activities
	env.OnActivity(GetGameScore, mock.Anything).Return(map[string]string{
		"130": "0",
		"264": "0",
	}, nil)

	game := Game{
		ID:        "test-game-query",
		StartTime: time.Now().Add(-time.Hour),
		Status:    "in",
		HomeTeam: Team{
			ID:          "130",
			DisplayName: "Michigan Wolverines",
		},
		AwayTeam: Team{
			ID:          "264",
			DisplayName: "Washington Huskies",
		},
	}

	// Start workflow
	env.ExecuteWorkflow(GameWorkflow, game)

	// Test query handler
	var queryResult Game
	encodedValue, err := env.QueryWorkflow("gameInfo")
	assert.NoError(t, err)
	err = encodedValue.Get(&queryResult)
	assert.NoError(t, err)
	assert.Equal(t, game.ID, queryResult.ID)
	assert.Equal(t, game.HomeTeam.DisplayName, queryResult.HomeTeam.DisplayName)
	assert.Equal(t, game.AwayTeam.DisplayName, queryResult.AwayTeam.DisplayName)
}

func TestGameWorkflow_ActivityFailure(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock activity to fail
	env.OnActivity(GetGameScore, mock.Anything).Return(nil, assert.AnError)

	game := Game{
		ID:        "test-game-fail",
		StartTime: time.Now().Add(-time.Hour),
		Status:    "in",
		HomeTeam: Team{
			ID:          "130",
			DisplayName: "Michigan Wolverines",
		},
		AwayTeam: Team{
			ID:          "264",
			DisplayName: "Washington Huskies",
		},
	}

	// Execute workflow
	env.ExecuteWorkflow(GameWorkflow, game)

	// Workflow should still complete despite activity failures
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

func TestGameWorkflow_LongRunning(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock activities
	env.OnActivity(GetGameScore, mock.Anything).Return(map[string]string{
		"130": "14",
		"264": "7",
	}, nil)
	env.OnActivity(SendNotification, mock.Anything).Return(nil)

	game := Game{
		ID:        "test-game-long",
		StartTime: time.Now().Add(-time.Hour),
		Status:    "in",
		CurrentScore: map[string]string{
			"130": "0",
			"264": "0",
		},
		HomeTeam: Team{
			ID:          "130",
			DisplayName: "Michigan Wolverines",
		},
		AwayTeam: Team{
			ID:          "264",
			DisplayName: "Washington Huskies",
		},
	}

	// Set up multiple timer callbacks to simulate long-running workflow
	for i := 1; i <= 5; i++ {
		minutes := time.Duration(i) * time.Minute
		env.RegisterDelayedCallback(func() {
			// Timer callback - time will advance automatically
		}, minutes)
	}

	// Execute workflow
	env.ExecuteWorkflow(GameWorkflow, game)

	// Verify workflow completed
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

func TestGameWorkflow_NoScoreChange(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock activities - always return same score
	env.OnActivity(GetGameScore, mock.Anything).Return(map[string]string{
		"130": "0",
		"264": "0",
	}, nil)

	// SendNotification should not be called since no score changes
	env.OnActivity(SendNotification, mock.Anything).Return(nil).Times(0)

	game := Game{
		ID:        "test-game-no-change",
		StartTime: time.Now().Add(-time.Hour),
		Status:    "in",
		CurrentScore: map[string]string{
			"130": "0",
			"264": "0",
		},
		HomeTeam: Team{
			ID:          "130",
			DisplayName: "Michigan Wolverines",
		},
		AwayTeam: Team{
			ID:          "264",
			DisplayName: "Washington Huskies",
		},
	}

	// Set up timer to advance time
	env.RegisterDelayedCallback(func() {
		// Timer callback - time will advance automatically
	}, 2*time.Minute)

	// Execute workflow
	env.ExecuteWorkflow(GameWorkflow, game)

	// Verify workflow completed
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify SendNotification was not called
	env.AssertExpectations(t)
}

// Benchmark test for workflow execution
func BenchmarkGameWorkflow(b *testing.B) {
	testSuite := &testsuite.WorkflowTestSuite{}

	game := Game{
		ID:        "benchmark-game",
		StartTime: time.Now().Add(-time.Hour),
		Status:    "in",
		HomeTeam: Team{
			ID:          "130",
			DisplayName: "Michigan Wolverines",
		},
		AwayTeam: Team{
			ID:          "264",
			DisplayName: "Washington Huskies",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env := testSuite.NewTestWorkflowEnvironment()
		env.OnActivity(GetGameScore, mock.Anything).Return(map[string]string{
			"130": "0",
			"264": "0",
		}, nil)

		env.ExecuteWorkflow(GameWorkflow, game)
	}
}
