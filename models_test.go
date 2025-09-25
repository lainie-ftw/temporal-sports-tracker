package sports

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestESPNTime_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Time
		wantErr  bool
	}{
		{
			name:     "RFC3339 format with timezone",
			input:    `"2023-09-10T15:30:00Z"`,
			expected: time.Date(2023, 9, 10, 15, 30, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "RFC3339 format with offset",
			input:    `"2023-09-10T15:30:00-04:00"`,
			expected: time.Date(2023, 9, 10, 15, 30, 0, 0, time.FixedZone("", -4*3600)),
			wantErr:  false,
		},
		{
			name:     "Short format without seconds",
			input:    `"2023-09-10T15:30Z"`,
			expected: time.Date(2023, 9, 10, 15, 30, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:    "Empty string",
			input:   `""`,
			wantErr: false,
		},
		{
			name:    "Null value",
			input:   `null`,
			wantErr: false,
		},
		{
			name:    "Invalid format",
			input:   `"invalid-date"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var espnTime ESPNTime
			err := json.Unmarshal([]byte(tt.input), &espnTime)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.input != `""` && tt.input != `null` {
				assert.True(t, tt.expected.Equal(espnTime.Time), 
					"Expected %v, got %v", tt.expected, espnTime.Time)
			}
		})
	}
}

func TestESPNResponse_UnmarshalJSON(t *testing.T) {
	jsonData := `{
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
								"score": "21",
								"homeAway": "home"
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
							"clock": 0.0,
							"displayClock": "0:00",
							"period": 1,
							"type": {
								"id": "2",
								"name": "In Progress",
								"state": "in",
								"completed": false,
								"description": "In Progress"
							}
						}
					}
				],
				"status": {
					"clock": 0.0,
					"displayClock": "0:00",
					"period": 1,
					"type": {
						"id": "2",
						"name": "In Progress",
						"state": "in",
						"completed": false,
						"description": "In Progress"
					}
				}
			}
		]
	}`

	var response ESPNResponse
	err := json.Unmarshal([]byte(jsonData), &response)
	require.NoError(t, err)

	assert.Len(t, response.Events, 1)
	
	event := response.Events[0]
	assert.Equal(t, "401520281", event.ID)
	assert.Equal(t, "Michigan vs Washington", event.Name)
	assert.Equal(t, "MICH vs WASH", event.ShortName)
	assert.Equal(t, 2, event.Week.Number)
	
	assert.Len(t, event.Competitions, 1)
	competition := event.Competitions[0]
	assert.Equal(t, "401520281", competition.ID)
	
	assert.Len(t, competition.Competitors, 1)
	competitor := competition.Competitors[0]
	assert.Equal(t, "130", competitor.ID)
	assert.Equal(t, "21", competitor.Score)
	assert.Equal(t, "home", competitor.HomeAway)
	
	team := competitor.Team
	assert.Equal(t, "130", team.ID)
	assert.Equal(t, "Michigan", team.Location)
	assert.Equal(t, "Wolverines", team.Name)
	assert.Equal(t, "MICH", team.Abbreviation)
	assert.Equal(t, "Michigan Wolverines", team.DisplayName)
	assert.Equal(t, "5", team.ConferenceId)
	
	assert.Len(t, competition.Odds, 1)
	odds := competition.Odds[0]
	assert.Equal(t, "MICH -7.5", odds.Details)
	assert.Equal(t, 45.5, odds.OverUnder)
	assert.True(t, odds.HomeTeamOdds.Favorite)
	assert.False(t, odds.HomeTeamOdds.Underdog)
	assert.False(t, odds.AwayTeamOdds.Favorite)
	assert.True(t, odds.AwayTeamOdds.Underdog)
}

func TestGame_Creation(t *testing.T) {
	startTime := time.Now()
	game := Game{
		ID:        "test-game-1",
		StartTime: startTime,
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
			Favorite:    true,
		},
		AwayTeam: Team{
			ID:          "264",
			Name:        "Huskies",
			DisplayName: "Washington Huskies",
			Underdog:    true,
		},
	}

	assert.Equal(t, "test-game-1", game.ID)
	assert.Equal(t, startTime, game.StartTime)
	assert.Equal(t, "pre", game.Status)
	assert.Equal(t, "MICH -7.5", game.Odds)
	assert.Equal(t, "0", game.CurrentScore["130"])
	assert.Equal(t, "0", game.CurrentScore["264"])
	assert.True(t, game.HomeTeam.Favorite)
	assert.True(t, game.AwayTeam.Underdog)
}

func TestScoreUpdate_Creation(t *testing.T) {
	timestamp := time.Now()
	update := ScoreUpdate{
		GameID:    "game-123",
		HomeTeam:  "Michigan Wolverines",
		AwayTeam:  "Washington Huskies",
		HomeScore: "14",
		AwayScore: "7",
		Timestamp: timestamp,
	}

	assert.Equal(t, "game-123", update.GameID)
	assert.Equal(t, "Michigan Wolverines", update.HomeTeam)
	assert.Equal(t, "Washington Huskies", update.AwayTeam)
	assert.Equal(t, "14", update.HomeScore)
	assert.Equal(t, "7", update.AwayScore)
	assert.Equal(t, timestamp, update.Timestamp)
}

func TestTrackingRequest_Creation(t *testing.T) {
	request := TrackingRequest{
		Sport:       "football",
		League:      "college-football",
		Teams:       []string{"130", "264"},
		Conferences: []string{"5"},
	}

	assert.Equal(t, "football", request.Sport)
	assert.Equal(t, "college-football", request.League)
	assert.Contains(t, request.Teams, "130")
	assert.Contains(t, request.Teams, "264")
	assert.Contains(t, request.Conferences, "5")
}
