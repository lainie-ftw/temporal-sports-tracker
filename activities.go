package sports

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
)

// Start a game workflow
func StartGameWorkflowActivity(ctx context.Context, game Game) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting a game workflow with game ID ", "gameID", game.ID)

	// We don't need to worry about duplicate "games" being created because we're using the game ID - if we try to start a second workflow with the same
	// game ID -> workflow ID, the default of the Go SDK is to just return the run ID of the already running workflow. Other SDKs will have different defaults!
	var workflowID = "game-" + game.ID

	TaskQueueName := os.Getenv("TASK_QUEUE")
	if TaskQueueName == "" {
		return fmt.Errorf("TASK_QUEUE environment variable is not set")
	}

	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: TaskQueueName,
	}
	c, err := client.Dial(GetClientOptions())
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

// Get games based on user input from the ESPN API
func GetGamesActivity(ctx context.Context, trackingRequest TrackingRequest) ([]Game, error) {
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

	// Add TV network
	if len(comp.Broadcasts) > 0 {
		game.TVNetwork = comp.Broadcasts[0].Name
	}
	return game
}

// FetchGameScoreActivity fetches current score for a specific game
func GetGameScoreActivity(ctx context.Context, game *Game) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Fetching game score", "gameID", game.ID)

	url := game.APIRoot + "/scoreboard"
//	url := fmt.Sprintf("%s/summary?event=%s", game.APIRoot, game.ID) //Example: https://site.api.espn.com/apis/site/v2/sports/football/college-football/summary?event=:gameId
	
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch game score: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var espnResp ESPNResponse
	if err := json.Unmarshal(body, &espnResp); err != nil {
		return fmt.Errorf("failed to unmarshal ESPN response: %w", err)
	}

	// Find the specific game
	for _, event := range espnResp.Events {
		if len(event.Competitions) > 0 && event.Competitions[0].ID == game.ID {
			comp := event.Competitions[0]
			scores := make(map[string]string)

			for _, competitor := range comp.Competitors {
				scores[competitor.Team.ID] = competitor.Score
			}
			
			// Update the current quarter
			if comp.Status.Period > 0 {
				game.Quarter = fmt.Sprintf("%d", comp.Status.Period)
			} else {
				game.Quarter = "0"
			}
			
			game.CurrentScore = scores
			logger.Info("Fetched game score", "gameID", game.ID, "scores", scores)
			return nil
		}
	}

	return fmt.Errorf("game not found: %s", game.ID)
}

func SendNotificationListActivity(ctx context.Context, sendNotifications SendNotifications) error {
	// For each notification message in the input list, send it to the specified channel in sendNotifications.Channel
	// NOTE: This means that if one notification in the list fails, the whole activity fails and none of the notifications are sent.
	// You could also do this with an activity per notification.
	for _, notification := range sendNotifications.NotificationList {
		logger := activity.GetLogger(ctx)
		logger.Info("Sending notification", "channel", sendNotifications.Channel, "title", notification.Title, "message", notification.Message)

		// Call the appropriate activity based on the channel
		switch sendNotifications.Channel {
		case "slack":
			err := SendSlackNotification(ctx, notification)
			if err != nil {
				return fmt.Errorf("failed to send Slack notification: %w", err)
			}
		case "hass":
			err := SendHomeAssistantNotification(ctx, notification)
			if err != nil {
				return fmt.Errorf("failed to send Home Assistant notification: %w", err)
			}
		case "logger":
			logger := activity.GetLogger(ctx)
			logger.Info("Logger notification", "title", notification.Title, "message", notification.Message)
		default:
			return fmt.Errorf("unknown notification channel: %s", sendNotifications.Channel)
		}
	}
	return nil
}

func SendHomeAssistantNotification(ctx context.Context, notification Notification) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending Home Assistant notification", "title", notification.Title, "message", notification.Message)

	hassWebhook := os.Getenv("HASS_WEBHOOK_URL")
	if hassWebhook == "" {
		return fmt.Errorf("HASS_WEBHOOK_URL environment variable is not set")
	}
	// Build the payload for Home Assistant
	jsonScoreUpdate := map[string]string{
		"title":   notification.Title,
		"message": notification.Message,
	}	
	jsonData, err := json.Marshal(jsonScoreUpdate)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	// Send the POST request to Home Assistant webhook with jsonData payload
	req, err := http.NewRequest("POST", hassWebhook, io.NopCloser(io.Reader(bytes.NewReader(jsonData))))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("received non-OK response from Home Assistant: %s", resp.Status)
	}
	return nil
}

// SendSlackNotificationActivity sends a notification to Slack
// TODO: test this
func SendSlackNotification(ctx context.Context, notification Notification) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending Slack notification", "title", notification.Title, "message", notification.Message)

	slackWebhook := os.Getenv("SLACK_WEBHOOK_URL")
	if slackWebhook == "" {
		return fmt.Errorf("SLACK_WEBHOOK_URL environment variable is not set")
	}

	// Build the payload for Slack
	payload := map[string]string{
		"text": fmt.Sprintf("*%s*\n%s", notification.Title, notification.Message),
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Send the POST request to Slack webhook with jsonData payload
	req, err := http.NewRequest("POST", slackWebhook, io.NopCloser(io.Reader(bytes.NewReader(jsonData))))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("received non-OK response from Slack: %s", resp.Status)
	}

	return nil
}
