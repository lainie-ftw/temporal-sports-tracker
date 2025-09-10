package sports

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
)

// Start a game workflow
func StartGameWorkflow(ctx context.Context, game Game) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting a game workflow with game ID ", "gameID", game.ID)

	var workflowID = "game-" + game.ID

	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: TaskQueueName,
	}
	c, err := client.Dial(client.Options{})
	if err != nil {
		return fmt.Errorf("unable to create Temporal client: %w", err)
	}
	defer c.Close()

	// Start the workflow
	we, err := c.ExecuteWorkflow(context.Background(), options, GameWorkflow, game)
	if err != nil {
		return fmt.Errorf("unable to execute workflow: %w", err)
	}
	logger.Info("Started workflow", "WorkflowID", we.GetID(), "RunID", we.GetRunID())
	return nil
}

// Get games by conference identifier from the ESPN API
func GetGames(ctx context.Context, trackingRequest TrackingRequest) ([]Game, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Fetching games from ESPN API")

	// Use the trackingRequest (sport and league) to build the URL
	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/%s/%s/scoreboard", trackingRequest.Sport, trackingRequest.League)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch games: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var espnResp ESPNResponse
	if err := json.Unmarshal(body, &espnResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ESPN response: %w", err)
	}

	var games []Game
	bigTenConferenceID := "5" // Big Ten conference ID

	for _, event := range espnResp.Events {
		logger.Info("Processing event", "name", event.Name)
	//	for _, event := range league.Events {
			// Filter for Big Ten games
			if len(event.Competitions) > 0 && len(event.Competitions[0].Competitors) >= 2 {
				comp := event.Competitions[0]

				// Check if either team is in the conference
				homeTeam := comp.Competitors[0]
				awayTeam := comp.Competitors[1]
				logger.Info("Home Team name", "name", homeTeam.Team.Name)
				logger.Info("Away Team name", "name", awayTeam.Team.Name)

				if homeTeam.Team.ConferenceId == bigTenConferenceID ||
				   awayTeam.Team.ConferenceId == bigTenConferenceID {

					//odds = ProcessOdds(comp.Odds)
					game := Game{
						ID:        comp.ID,
						EventID:   event.ID,
						StartTime: comp.Date.Time,
						Status:    comp.Status.Type.State,
						CurrentScore: make(map[string]string),
					}

					// Determine home and away teams
					if homeTeam.HomeAway == "home" {
						game.HomeTeam = homeTeam.Team
						game.AwayTeam = awayTeam.Team
						game.CurrentScore[homeTeam.Team.ID] = homeTeam.Score
						game.CurrentScore[awayTeam.Team.ID] = awayTeam.Score
					} else {
						game.HomeTeam = awayTeam.Team
						game.AwayTeam = homeTeam.Team
						game.CurrentScore[awayTeam.Team.ID] = awayTeam.Score
						game.CurrentScore[homeTeam.Team.ID] = homeTeam.Score
					}

					// Set favorite and underdog based on odds
					if len(comp.Odds) > 0 {
						game.Odds = comp.Odds[0].Details
						game.HomeTeam.Favorite = comp.Odds[0].HomeTeamOdds.Favorite
						game.HomeTeam.Underdog = comp.Odds[0].HomeTeamOdds.Underdog
						game.AwayTeam.Favorite = comp.Odds[0].AwayTeamOdds.Favorite
						game.AwayTeam.Underdog = comp.Odds[0].AwayTeamOdds.Underdog
					}
					games = append(games, game)
				}
			}
	//	}
	}

	logger.Info("Fetched games", "count", len(games))
	return games, nil
}

// FetchGameScoreActivity fetches current score for a specific game
func GetGameScore(ctx context.Context, gameID string) (map[string]string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Fetching game score", "gameID", gameID)

	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/football/college-football/scoreboard")

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch game score: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var espnResp ESPNResponse
	if err := json.Unmarshal(body, &espnResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ESPN response: %w", err)
	}

	// Find the specific game
	for _, event := range espnResp.Events {
	//	for _, event := range league.Events {
			if len(event.Competitions) > 0 && event.Competitions[0].ID == gameID {
				comp := event.Competitions[0]
				scores := make(map[string]string)

				for _, competitor := range comp.Competitors {
					scores[competitor.Team.ID] = competitor.Score
				}

				return scores, nil
			}
		//}
	}

	return nil, fmt.Errorf("game not found: %s", gameID)
}

func SendNotification(ctx context.Context, update ScoreUpdate) error {
	return SendSlackNotificationActivity(ctx, update)
}

// SendSlackNotificationActivity sends a notification to Slack (mocked)
func SendSlackNotificationActivity(ctx context.Context, update ScoreUpdate) error {
	logger := activity.GetLogger(ctx)

	// Mock Slack notification - in real implementation, this would send to Slack webhook
	message := fmt.Sprintf("🏈 Score Update!\n%s vs %s\nScore: %s - %s\nTime: %s",
		update.HomeTeam, update.AwayTeam, update.HomeScore, update.AwayScore,
		update.Timestamp.Format("2006-01-02 15:04:05"))

	logger.Info("Slack notification (mocked)", "message", message)

	// Simulate some processing time
	// time.Sleep(100 * time.Millisecond)

	return nil
}
