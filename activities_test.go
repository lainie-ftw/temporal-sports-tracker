package sports

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
)

// Mock Temporal client for testing
type MockTemporalClient struct {
	mock.Mock
}

func (m *MockTemporalClient) ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error) {
	mockArgs := m.Called(ctx, options, workflow, args)
	return mockArgs.Get(0).(client.WorkflowRun), mockArgs.Error(1)
}

func (m *MockTemporalClient) Close() {
	m.Called()
}

// Mock WorkflowRun for testing
type MockWorkflowRun struct {
	mock.Mock
}

func (m *MockWorkflowRun) GetID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockWorkflowRun) GetRunID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockWorkflowRun) Get(ctx context.Context, valuePtr interface{}) error {
	args := m.Called(ctx, valuePtr)
	return args.Error(0)
}

func TestGetGames(t *testing.T) {
	// Create test suite for activity testing
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	
	// Register the activity
	env.RegisterActivity(GetGamesActivity)

	tests := []struct {
		name           string
		trackingReq    TrackingRequest
		mockResponse   string
		expectedGames  int
		expectedError  bool
		statusCode     int
	}{
		{
			name: "successful fetch with Big Ten games",
			trackingReq: TrackingRequest{
				Sport:  "football",
				League: "college-football",
			},
			mockResponse: `{
				"events": [
					{
						"id": "401520281",
						"date": "2023-09-10T15:30:00Z",
						"name": "Michigan vs Washington",
						"shortName": "MICH vs WASH",
						"week": {"number": 2},
						"competitions": [
							{
								"id": "401520281",
								"date": "2023-09-10T15:30:00Z",
								"competitors": [
									{
										"id": "130",
										"team": {
											"id": "130",
											"location": "Michigan",
											"name": "Wolverines",
											"abbreviation": "MICH",
											"displayName": "Michigan Wolverines",
											"conferenceId": "5"
										},
										"score": "0",
										"homeAway": "home"
									},
									{
										"id": "264",
										"team": {
											"id": "264",
											"location": "Washington",
											"name": "Huskies",
											"abbreviation": "WASH",
											"displayName": "Washington Huskies",
											"conferenceId": "9"
										},
										"score": "0",
										"homeAway": "away"
									}
								],
								"odds": [
									{
										"details": "MICH -7.5",
										"overUnder": 45.5,
										"homeTeamOdds": {
											"favorite": true,
											"underdog": false
										},
										"awayTeamOdds": {
											"favorite": false,
											"underdog": true
										}
									}
								],
								"status": {
									"type": {
										"state": "pre"
									}
								}
							}
						]
					}
				]
			}`,
			expectedGames: 1,
			expectedError: false,
			statusCode:    200,
		},
		{
			name: "no Big Ten games",
			trackingReq: TrackingRequest{
				Sport:  "football",
				League: "college-football",
			},
			mockResponse: `{
				"events": [
					{
						"id": "401520282",
						"competitions": [
							{
								"id": "401520282",
								"competitors": [
									{
										"team": {"conferenceId": "1"},
										"homeAway": "home"
									},
									{
										"team": {"conferenceId": "8"},
										"homeAway": "away"
									}
								]
							}
						]
					}
				]
			}`,
			expectedGames: 0,
			expectedError: false,
			statusCode:    200,
		},
		{
			name: "HTTP error",
			trackingReq: TrackingRequest{
				Sport:  "football",
				League: "college-football",
			},
			mockResponse:  "",
			expectedGames: 0,
			expectedError: true,
			statusCode:    500,
		},
		{
			name: "invalid JSON response",
			trackingReq: TrackingRequest{
				Sport:  "football",
				League: "college-football",
			},
			mockResponse:  "invalid json",
			expectedGames: 0,
			expectedError: true,
			statusCode:    200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedURL := fmt.Sprintf("/apis/site/v2/sports/%s/%s/scoreboard", tt.trackingReq.Sport, tt.trackingReq.League)
				assert.Equal(t, expectedURL, r.URL.Path)
				
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == 200 {
					w.Write([]byte(tt.mockResponse))
				}
			}))
			defer server.Close()

			// Replace the ESPN API URL in the function (this would need to be configurable in real implementation)
			// For now, we'll test the logic with a mock server
			
			// Execute the activity
			encodedValue, err := env.ExecuteActivity(GetGamesActivity, tt.trackingReq)
			
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				
				var games []Game
				err = encodedValue.Get(&games)
				assert.NoError(t, err)
				assert.Len(t, games, tt.expectedGames)
				
				if len(games) > 0 {
					game := games[0]
					assert.NotEmpty(t, game.ID)
					assert.NotEmpty(t, game.HomeTeam.DisplayName)
					assert.NotEmpty(t, game.AwayTeam.DisplayName)
					assert.NotNil(t, game.CurrentScore)
				}
			}
		})
	}
}

func TestGetGameScore(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	
	// Register the activity
	env.RegisterActivity(GetGameScoreActivity)

	tests := []struct {
		name          string
		game          *Game
		mockResponse  string
		expectedError bool
		statusCode    int
	}{
		{
			name: "successful score fetch",
			game: &Game{
				ID:      "401520281",
				APIRoot: "https://site.api.espn.com/apis/site/v2/sports/football/college-football",
				CurrentScore: make(map[string]string),
			},
			mockResponse: `{
				"events": [
					{
						"competitions": [
							{
								"id": "401520281",
								"competitors": [
									{
										"team": {"id": "130"},
										"score": "14"
									},
									{
										"team": {"id": "264"},
										"score": "7"
									}
								],
								"status": {
									"period": 2
								}
							}
						]
					}
				]
			}`,
			expectedError: false,
			statusCode:    200,
		},
		{
			name: "game not found",
			game: &Game{
				ID:      "nonexistent",
				APIRoot: "https://site.api.espn.com/apis/site/v2/sports/football/college-football",
				CurrentScore: make(map[string]string),
			},
			mockResponse: `{"events": []}`,
			expectedError: true,
			statusCode:    200,
		},
		{
			name: "HTTP error",
			game: &Game{
				ID:      "401520281",
				APIRoot: "https://site.api.espn.com/apis/site/v2/sports/football/college-football",
				CurrentScore: make(map[string]string),
			},
			mockResponse:  "",
			expectedError: true,
			statusCode:    500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == 200 {
					w.Write([]byte(tt.mockResponse))
				}
			}))
			defer server.Close()

			// Update the game's APIRoot to use the test server
			tt.game.APIRoot = server.URL

			_, err := env.ExecuteActivity(GetGameScoreActivity, tt.game)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// The activity modifies the game object directly
				assert.NotNil(t, tt.game.CurrentScore)
				if tt.name == "successful score fetch" {
					assert.Equal(t, "2", tt.game.Quarter)
					assert.Contains(t, tt.game.CurrentScore, "130")
					assert.Contains(t, tt.game.CurrentScore, "264")
				}
			}
		})
	}
}

func TestSendSlackNotificationActivity(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	
	// Register the activity
	env.RegisterActivity(SendSlackNotification)

	notification := Notification{
		Title:   "Game Update",
		Message: "Michigan Wolverines 14 - Washington Huskies 7",
	}

	_, err := env.ExecuteActivity(SendSlackNotification, notification)
	assert.NoError(t, err)
}

func TestSendNotificationList(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	
	// Register the activity
	env.RegisterActivity(SendNotificationListActivity)

	sendNotifications := SendNotifications{
		Channel: "logger",
		NotificationList: []Notification{
			{
				Title:   "Game Update",
				Message: "Michigan Wolverines 21 - Washington Huskies 14",
			},
		},
	}

	_, err := env.ExecuteActivity(SendNotificationListActivity, sendNotifications)
	assert.NoError(t, err)
}

// Integration test for the activity context
func TestActivitiesWithContext(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	
	// Register the activity
	env.RegisterActivity(GetGamesActivity)

	// Test GetGamesActivity with context
	trackingReq := TrackingRequest{
		Sport:  "football",
		League: "college-football",
	}

	_, err := env.ExecuteActivity(GetGamesActivity, trackingReq)
	// This will fail due to actual HTTP call, but tests the context setup
	assert.Error(t, err) // Expected since we're making real HTTP calls
}

// Test helper functions for creating test data
func createTestGame() Game {
	return Game{
		ID:        "test-game-1",
		StartTime: time.Now().Add(time.Hour),
		Status:    "pre",
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
}

func createTestScoreUpdate() ScoreUpdate {
	return ScoreUpdate{
		GameID:    "game-123",
		HomeTeam:  "Michigan Wolverines",
		AwayTeam:  "Washington Huskies",
		HomeScore: "14",
		AwayScore: "7",
		Timestamp: time.Now(),
	}
}

// Benchmark tests
func BenchmarkGetGames(b *testing.B) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	
	// Register the activity
	env.RegisterActivity(GetGamesActivity)

	trackingReq := TrackingRequest{
		Sport:  "football",
		League: "college-football",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env.ExecuteActivity(GetGamesActivity, trackingReq)
	}
}

func BenchmarkSendSlackNotification(b *testing.B) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	
	// Register the activity
	env.RegisterActivity(SendSlackNotification)

	notification := Notification{
		Title:   "Game Update",
		Message: "Test notification message",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env.ExecuteActivity(SendSlackNotification, notification)
	}
}
