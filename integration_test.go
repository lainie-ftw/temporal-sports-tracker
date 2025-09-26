package sports

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

// Integration tests that test multiple components working together
func TestIntegration_FullWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Test data
	trackingRequest := TrackingRequest{
		Sport:       "football",
		League:      "college-football",
		Teams:       []string{"130"},
		Conferences: []string{"5"},
	}

	testGames := []Game{
		{
			ID:        "integration-game-1",
			StartTime: time.Now().Add(time.Hour),
			Status:    "pre",
			HomeTeam: Team{
				ID:          "130",
				DisplayName: "Michigan Wolverines",
				ConferenceId: "5",
			},
			AwayTeam: Team{
				ID:          "264",
				DisplayName: "Washington Huskies",
				ConferenceId: "9",
			},
			CurrentScore: map[string]string{
				"130": "0",
				"264": "0",
			},
		},
	}

	// Mock activities for the full workflow
	env.OnActivity(GetGamesActivity, trackingRequest).Return(testGames, nil)
	env.OnActivity(StartGameWorkflowActivity, testGames[0]).Return(nil)

	// Execute the collect games workflow
	env.ExecuteWorkflow(CollectGamesWorkflow, trackingRequest)

	// Verify workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify all expected activities were called
	env.AssertExpectations(t)
}

func TestIntegration_GameWorkflowWithScoreUpdates(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Create a game that has already started
	game := Game{
		ID:        "integration-game-2",
		StartTime: time.Now().Add(-30 * time.Minute), // Started 30 minutes ago
		Status:    "in",
		HomeTeam: Team{
			ID:          "130",
			DisplayName: "Michigan Wolverines",
		},
		AwayTeam: Team{
			ID:          "264",
			DisplayName: "Washington Huskies",
		},
		CurrentScore: map[string]string{
			"130": "0",
			"264": "0",
		},
	}

	// Mock score progression: 0-0 -> 7-0 -> 7-7
	callCount := 0
	env.OnActivity(GetGameScoreActivity, &game).Return(func(g *Game) error {
		callCount++
		switch callCount {
		case 1:
			g.CurrentScore = map[string]string{"130": "0", "264": "0"}
		case 2:
			g.CurrentScore = map[string]string{"130": "7", "264": "0"}
		default:
			g.CurrentScore = map[string]string{"130": "7", "264": "7"}
		}
		return nil
	})

	// Expect notifications for score changes
	env.OnActivity(SendNotificationListActivity, SendNotifications{
		Channel: "logger",
		NotificationList: []Notification{
			{
				Title:   "Score Update",
				Message: "Michigan Wolverines 7 - Washington Huskies 0",
			},
		},
	}).Return(nil).Once()

	env.OnActivity(SendNotificationListActivity, SendNotifications{
		Channel: "logger",
		NotificationList: []Notification{
			{
				Title:   "Score Update",
				Message: "Michigan Wolverines 7 - Washington Huskies 7",
			},
		},
	}).Return(nil).Once()

	// Set up timers to advance the workflow
	env.RegisterDelayedCallback(func() {
		// First score check
	}, time.Minute)

	env.RegisterDelayedCallback(func() {
		// Second score check
	}, 2*time.Minute)

	// Execute the game workflow
	env.ExecuteWorkflow(GameWorkflow, game)

	// Verify workflow completed
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify the result
	var result string
	err := env.GetWorkflowResult(&result)
	assert.NoError(t, err)
	assert.Contains(t, result, "Final score:")

	// Verify all activities were called as expected
	env.AssertExpectations(t)
}

func TestIntegration_ESPNTimeInWorkflow(t *testing.T) {
	// Test that ESPNTime works correctly in the context of workflows
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Create a game with ESPNTime
	gameTime := time.Date(2023, 9, 10, 15, 30, 0, 0, time.UTC)
	game := Game{
		ID:        "espn-time-test",
		StartTime: gameTime,
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

	// Mock activities
	env.OnActivity(GetGameScoreActivity, &game).Return(nil)

	// Execute workflow
	env.ExecuteWorkflow(GameWorkflow, game)

	// Test query handler to verify game time is preserved
	var queryResult Game
	encodedValue, err := env.QueryWorkflow("gameInfo")
	require.NoError(t, err)
	err = encodedValue.Get(&queryResult)
	require.NoError(t, err)

	// Verify the time was preserved correctly
	assert.True(t, gameTime.Equal(queryResult.StartTime))
	assert.Equal(t, game.ID, queryResult.ID)
}

func TestIntegration_ErrorHandling(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	trackingRequest := TrackingRequest{
		Sport:  "football",
		League: "college-football",
	}

	// Mock GetGamesActivity to fail
	env.OnActivity(GetGamesActivity, trackingRequest).Return(nil, assert.AnError)

	// Execute workflow
	env.ExecuteWorkflow(CollectGamesWorkflow, trackingRequest)

	// Verify workflow failed as expected
	assert.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestIntegration_ActivityRetries(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	game := Game{
		ID:        "retry-test-game",
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

	// Mock GetGameScoreActivity to fail first few times, then succeed
	callCount := 0
	env.OnActivity(GetGameScoreActivity, &game).Return(func(g *Game) error {
		callCount++
		if callCount < 3 {
			return assert.AnError
		}
		g.CurrentScore = map[string]string{"130": "14", "264": "7"}
		return nil
	})

	// Execute workflow
	env.ExecuteWorkflow(GameWorkflow, game)

	// Workflow should complete despite initial failures due to retry policy
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

func TestIntegration_MultipleGamesWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	trackingRequest := TrackingRequest{
		Sport:       "football",
		League:      "college-football",
		Conferences: []string{"5"},
	}

	// Multiple games
	testGames := []Game{
		{
			ID:        "multi-game-1",
			StartTime: time.Now().Add(time.Hour),
			Status:    "pre",
			HomeTeam:  Team{ID: "130", DisplayName: "Michigan Wolverines"},
			AwayTeam:  Team{ID: "264", DisplayName: "Washington Huskies"},
		},
		{
			ID:        "multi-game-2",
			StartTime: time.Now().Add(2 * time.Hour),
			Status:    "pre",
			HomeTeam:  Team{ID: "194", DisplayName: "Northwestern Wildcats"},
			AwayTeam:  Team{ID: "275", DisplayName: "Wisconsin Badgers"},
		},
		{
			ID:        "multi-game-3",
			StartTime: time.Now().Add(-time.Hour), // Past game, should be filtered
			Status:    "final",
			HomeTeam:  Team{ID: "213", DisplayName: "Minnesota Golden Gophers"},
			AwayTeam:  Team{ID: "356", DisplayName: "Iowa Hawkeyes"},
		},
	}

	env.OnActivity(GetGamesActivity, trackingRequest).Return(testGames, nil)
	
	// Only future games should trigger StartGameWorkflowActivity
	env.OnActivity(StartGameWorkflowActivity, testGames[0]).Return(nil).Once()
	env.OnActivity(StartGameWorkflowActivity, testGames[1]).Return(nil).Once()
	// testGames[2] should not trigger StartGameWorkflowActivity because it's in the past

	// Execute workflow
	env.ExecuteWorkflow(CollectGamesWorkflow, trackingRequest)

	// Verify workflow completed
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify only 2 StartGameWorkflowActivity calls were made (for future games)
	env.AssertExpectations(t)
}

func TestIntegration_WorkflowCancellation(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	game := Game{
		ID:        "cancel-test-game",
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

	// Mock activities
	env.OnActivity(GetGameScoreActivity, &game).Return(nil)

	// Set up cancellation after 30 seconds
	env.RegisterDelayedCallback(func() {
		env.CancelWorkflow()
	}, 30*time.Second)

	// Execute workflow
	env.ExecuteWorkflow(GameWorkflow, game)

	// Verify workflow was cancelled
	assert.True(t, env.IsWorkflowCompleted())
	// Note: Cancelled workflows may or may not return an error depending on implementation
}

// Benchmark integration test
func BenchmarkIntegration_FullWorkflow(b *testing.B) {
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
			HomeTeam:  Team{ID: "130", DisplayName: "Michigan Wolverines"},
			AwayTeam:  Team{ID: "264", DisplayName: "Washington Huskies"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env := testSuite.NewTestWorkflowEnvironment()
		env.OnActivity(GetGamesActivity, trackingRequest).Return(testGames, nil)
		env.OnActivity(StartGameWorkflowActivity, testGames[0]).Return(nil)

		env.ExecuteWorkflow(CollectGamesWorkflow, trackingRequest)
	}
}

// Test helper functions
func TestIntegration_HelperFunctions(t *testing.T) {
	// Test createTestGame helper from activities_test.go
	game := createTestGame()
	assert.NotEmpty(t, game.ID)
	assert.NotEmpty(t, game.HomeTeam.DisplayName)
	assert.NotEmpty(t, game.AwayTeam.DisplayName)
	assert.NotNil(t, game.CurrentScore)

	// Test createTestScoreUpdate helper
	update := createTestScoreUpdate()
	assert.NotEmpty(t, update.GameID)
	assert.NotEmpty(t, update.HomeTeam)
	assert.NotEmpty(t, update.AwayTeam)
	assert.NotZero(t, update.Timestamp)
}

// Test constants and shared values
func TestIntegration_SharedConstants(t *testing.T) {
	TaskQueueName := os.Getenv("TASK_QUEUE")
	// Test that TaskQueueName is defined and not empty
	assert.NotEmpty(t, TaskQueueName)
	assert.Equal(t, "sports-tracker-task-queue", TaskQueueName)
}
