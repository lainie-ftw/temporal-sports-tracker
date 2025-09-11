package sports

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
)

// Start a game workflow
func StartGameWorkflow(ctx context.Context, game Game) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting a game workflow with game ID ", "gameID", game.ID)

	// We don't need to worry about duplicate "games" being created because we're using the game ID - if we try to start a second workflow with the same
	// game ID -> workflow ID, the default of the Go SDK is to just return the run ID of the already running workflow. Other SDKs will have different defaults!
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

	// Start the workflow with the Game object sent in
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
	var apiRoot string = fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/%s/%s", trackingRequest.Sport, trackingRequest.League)
	scoreboardUrl := apiRoot + "/scoreboard" //If you don't specify a conference, it will give you the top 25 games across all conferences

	var games []Game

	// if trackingRequest.Conferences is not empty, hit API for each conference and combine results
	if len(trackingRequest.Conferences) > 0 {
		for _, conf := range trackingRequest.Conferences {
			url := fmt.Sprintf("%s/scoreboard?groups=%s", apiRoot, conf)
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

			// Process every game in this conference
			for _, event := range espnResp.Events {
				logger.Info("Processing event", "name", event.Name)
				if len(event.Competitions) > 0 && len(event.Competitions[0].Competitors) >= 2 {
					comp := event.Competitions[0]

					homeTeam := comp.Competitors[0]
					awayTeam := comp.Competitors[1]
					logger.Info("Home Team name", "name", homeTeam.Team.Name)
					logger.Info("Away Team name", "name", awayTeam.Team.Name)

					game := BuildGame(comp, homeTeam, awayTeam, apiRoot)
					games = append(games, game)
				}
			}
		}
	}
	
	// if trackingRequest.Teams is not empty, hit the general scoreboard and filter results for those teams
	if len(trackingRequest.Teams) > 0 {
		resp, err := http.Get(scoreboardUrl)
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

		for _, event := range espnResp.Events {
			logger.Info("Processing event", "name", event.Name)
			if len(event.Competitions) > 0 && len(event.Competitions[0].Competitors) >= 2 {
				comp := event.Competitions[0]

				homeTeam := comp.Competitors[0]
				awayTeam := comp.Competitors[1]
				logger.Info("Home Team name", "name", homeTeam.Team.Name)
				logger.Info("Away Team name", "name", awayTeam.Team.Name)

				// Filter games by teams in the request
				if slices.Contains(trackingRequest.Teams, homeTeam.Team.ID) ||
					slices.Contains(trackingRequest.Teams, awayTeam.Team.ID) {
					game := BuildGame(comp, homeTeam, awayTeam, apiRoot)
					games = append(games, game)
				}
			}
		}
	}

	logger.Info("Fetched games", "count", len(games))
	return games, nil
}

// Helper function to create a Game from a Competition and its Competitors
func BuildGame(comp Competition, homeTeam, awayTeam Competitor, apiRoot string) Game {
	game := Game{
		ID:        comp.ID,
		StartTime: comp.Date.Time,
		Status:    comp.Status.Type.State,
		APIRoot: apiRoot,
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
	return game
}

// FetchGameScoreActivity fetches current score for a specific game
func GetGameScore(ctx context.Context, game Game) (map[string]string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Fetching game score", "gameID", game.ID)

	url := game.APIRoot + "/scoreboard"
//	url := fmt.Sprintf("%s/summary?event=%s", game.APIRoot, game.ID) //Example: https://site.api.espn.com/apis/site/v2/sports/football/college-football/summary?event=:gameId
	
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
		if len(event.Competitions) > 0 && event.Competitions[0].ID == game.ID {
			comp := event.Competitions[0]
			scores := make(map[string]string)

			for _, competitor := range comp.Competitors {
				scores[competitor.Team.ID] = competitor.Score
			}

			return scores, nil
		}
	}

	return nil, fmt.Errorf("game not found: %s", game.ID)
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
