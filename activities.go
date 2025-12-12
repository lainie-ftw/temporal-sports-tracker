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
	"strings"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"

	"github.com/sashabaranov/go-openai"
	"github.com/slack-go/slack"
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

// fetchCS2MatchesFromLiquipedia uses an LLM to scrape upcoming CS2 matches from Liquipedia
func fetchCS2MatchesFromLiquipedia(ctx context.Context, teamName string, trackingRequest TrackingRequest) ([]Game, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Fetching CS2 matches from Liquipedia via LLM", "team", teamName)

	openRouterAPIKey := os.Getenv("OPENROUTER_API_KEY")
	if openRouterAPIKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY environment variable is not set")
	}

	// Create OpenAI client configured for OpenRouter
	config := openai.DefaultConfig(openRouterAPIKey)
	config.BaseURL = "https://openrouter.ai/api/v1"
	client := openai.NewClientWithConfig(config)

	// Construct the Liquipedia URL
	//liquipediaURL := fmt.Sprintf("https://liquipedia.net/counterstrike/%s", strings.ReplaceAll(teamName, " ", "_"))
	liquipediaURL := "https://liquipedia.net/counterstrike/Liquipedia:Matches"

	// Fetch the HTML content from Liquipedia
	httpResp, err := http.Get(liquipediaURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Liquipedia page: %w", err)
	}
	defer httpResp.Body.Close()

	htmlContent, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Liquipedia page content: %w", err)
	}

	htmlString := string(htmlContent)
	println(htmlString)

	// Create the prompt for the LLM with the actual HTML content
	prompt := fmt.Sprintf(`You are a data extraction assistant. Extract information from the provided HTML content.

TASK: Find the next scheduled match for the team "%s" in the upcoming matches section.

INSTRUCTIONS:
1. Search for "%s" in team names in the Matches section
2. Find the EARLIEST upcoming OR active match (not concluded matches)
3. Extract these exact fields:
   - team1: First team name
   - team2: Second team name (opponent)
   - date: The match date in format like "December 15, 2025"
   - time: The match time (include timezone)
   - tournament: The tournament name

OUTPUT FORMAT:
Respond ONLY with valid JSON array. No explanations, no markdown formatting.
[
  {
    "team1": "Team A",
    "team2": "Team B",
    "date": "December 15, 2025",
    "time": "18:00 CET",
    "tournament": "ESL Pro League Season 19"
  }
]

If you do not find upcoming matches, respond with an empty array:
[]

HTML CONTENT:
%s`, teamName, teamName, htmlString)

	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: "meta-llama/llama-3.3-70b-instruct:free",
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			MaxTokens: 2000,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	responseText := resp.Choices[0].Message.Content
	logger.Info("LLM response", "response", responseText)

	// Parse the JSON response
	var matchesData []map[string]string
	// Clean the response - sometimes LLMs add markdown code blocks
	responseText = strings.TrimSpace(responseText)
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	if err := json.Unmarshal([]byte(responseText), &matchesData); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response as JSON: %w (response: %s)", err, responseText)
	}

	// Convert the parsed data to Game structs
	var games []Game
	for _, matchData := range matchesData {
		team1 := matchData["team1"]
		team2 := matchData["team2"]
		dateStr := matchData["date"]
		timeStr := matchData["time"]
		tournament := matchData["tournament"]

		// Parse the date and time
		startTime := parseMatchDateTime(dateStr, timeStr)

		// Create a unique game ID
		gameID := fmt.Sprintf("cs2-%s-vs-%s-%s", 
			strings.ToLower(strings.ReplaceAll(team1, " ", "-")),
			strings.ToLower(strings.ReplaceAll(team2, " ", "-")),
			startTime.Format("20060102"))

		// Create the Game struct
		game := Game{
			ID:        gameID,
			Sport:     "esports",
			League:    "cs2",
			HomeTeam:  Team{ID: team1, DisplayName: team1, Abbreviation: getAbbreviation(team1)},
			AwayTeam:  Team{ID: team2, DisplayName: team2, Abbreviation: getAbbreviation(team2)},
			StartTime: startTime,
			Status:    "pre",
			CurrentScore: map[string]string{team1: "0", team2: "0"},
			TVNetwork: tournament,
			NotificationTypes:    trackingRequest.NotificationTypes,
			NotificationChannels: trackingRequest.NotificationChannels,
		}

		games = append(games, game)
	}

	logger.Info("Parsed matches from Liquipedia", "count", len(games), "team", teamName)
	return games, nil
}

// parseMatchDateTime converts Liquipedia date/time strings to time.Time
func parseMatchDateTime(dateStr, timeStr string) time.Time {
	// Try to parse the date string
	// Common formats: "December 15, 2025", "2025-12-15", "15 Dec 2025"
	var t time.Time
	var err error

	// Try various date formats
	formats := []string{
		"January 2, 2006",
		"2006-01-02",
		"2 Jan 2006",
		"Jan 2, 2006",
	}

	for _, format := range formats {
		t, err = time.Parse(format, dateStr)
		if err == nil {
			break
		}
	}

	if err != nil {
		// If we can't parse the date, default to 24 hours from now
		return time.Now().Add(24 * time.Hour)
	}

	// If time string is provided, try to parse it (e.g., "18:00 CET")
	// For simplicity, we'll ignore timezone and just parse the time
	if timeStr != "" {
		timeParts := strings.Fields(timeStr)
		if len(timeParts) > 0 {
			timeOnly := timeParts[0] // Get "18:00" from "18:00 CET"
			if timeT, err := time.Parse("15:04", timeOnly); err == nil {
				t = time.Date(t.Year(), t.Month(), t.Day(), timeT.Hour(), timeT.Minute(), 0, 0, t.Location())
			}
		}
	}

	return t
}

// getAbbreviation returns a short abbreviation for a team name (max 4 characters)
func getAbbreviation(teamName string) string {
	if len(teamName) <= 4 {
		return teamName
	}
	return teamName[:4]
}

// Get games based on user input from the ESPN API
func GetGamesActivity(ctx context.Context, trackingRequest TrackingRequest) ([]Game, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Fetching games")

	var games []Game

	// Handle eSports CS2 separately - fetch matches from Liquipedia using LLM
	if trackingRequest.Sport == "esports" && trackingRequest.League == "cs2" {
		logger.Info("Fetching CS2 matches from Liquipedia via LLM")
		
		// Track unique game IDs to avoid duplicates
		seenGames := make(map[string]bool)
		
		// For each team in trackingRequest.Teams, call LLM to fetch matches from Liquipedia
		for _, teamName := range trackingRequest.Teams {
			teamGames, err := fetchCS2MatchesFromLiquipedia(ctx, teamName, trackingRequest)
			if err != nil {
				logger.Warn("Failed to fetch CS2 matches for team", "team", teamName, "error", err)
				continue
			}
			
			// Add games, avoiding duplicates
			for _, game := range teamGames {
				if !seenGames[game.ID] {
					games = append(games, game)
					seenGames[game.ID] = true
				}
			}
		}
		
		logger.Info("Fetched CS2 matches", "count", len(games))
		return games, nil
	}

	// Use the trackingRequest (sport and league) to build the URL
	logger.Info("Fetching games from ESPN API")
	var apiRoot string = fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/%s/%s", trackingRequest.Sport, trackingRequest.League)
	scoreboardUrl := apiRoot + "/scoreboard" //If you don't specify a conference, it will give you the top 25 games across all conferences

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

					game := BuildGame(comp, homeTeam, awayTeam, apiRoot, trackingRequest)
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
					game := BuildGame(comp, homeTeam, awayTeam, apiRoot, trackingRequest)
					games = append(games, game)
				}
			}
		}
	}

	logger.Info("Fetched games", "count", len(games))
	return games, nil
}

// Helper function to create a Game from a Competition and its Competitors
func BuildGame(comp Competition, homeTeam Competitor, awayTeam Competitor, apiRoot string, request TrackingRequest) Game {
	game := Game{
		ID:                   comp.ID,
		Sport:                request.Sport,
		League:               request.League,
		StartTime:            comp.Date.Time,
		Status:               comp.Status.Type.State,
		APIRoot:              apiRoot,
		CurrentScore:         make(map[string]string),
		TVNetwork:            comp.Broadcast,
		DisplayClock:         comp.Status.DisplayClock,
		NumberOfPeriods:      comp.Format.Regulation.NumberOfPeriods,
		UnderdogWinning:      false,
		NotificationTypes:    request.NotificationTypes,
		NotificationChannels: request.NotificationChannels,
	}

	game.CurrentPeriod = fmt.Sprintf("%d", int(comp.Status.Period))

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

// GetGameScoreActivity fetches current score for a specific game
func GetGameScoreActivity(ctx context.Context, game Game) (Game, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Fetching game score", "gameID", game.ID)

	var gameUpdate Game

	// Handle eSports CS2 separately - no score polling for now
	if game.Sport == "esports" && game.League == "cs2" {
		logger.Info("CS2 game - skipping score polling (placeholder)")
		// TODO: Implement LLM-based score polling for CS2 matches
		// This will call an LLM to read the match page on Liquipedia and extract current score
		// For now, return the game with unchanged scores
		gameUpdate.CurrentScore = game.CurrentScore
		gameUpdate.CurrentPeriod = game.CurrentPeriod
		gameUpdate.DisplayClock = game.DisplayClock
		return gameUpdate, nil
	}

	url := game.APIRoot + "/scoreboard"
	//	url := fmt.Sprintf("%s/summary?event=%s", game.APIRoot, game.ID) //Example: https://site.api.espn.com/apis/site/v2/sports/football/college-football/summary?event=:gameId

	resp, err := http.Get(url)
	if err != nil {
		return gameUpdate, fmt.Errorf("failed to fetch game score: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return gameUpdate, fmt.Errorf("failed to read response body: %w", err)
	}

	var espnResp ESPNResponse
	if err := json.Unmarshal(body, &espnResp); err != nil {
		return gameUpdate, fmt.Errorf("failed to unmarshal ESPN response: %w", err)
	}

	// Find the specific game
	for _, event := range espnResp.Events {
		if len(event.Competitions) > 0 && event.Competitions[0].ID == game.ID {
			comp := event.Competitions[0]
			scores := make(map[string]string)

			for _, competitor := range comp.Competitors {
				scores[competitor.Team.ID] = competitor.Score
			}

			// Update the current quarter, display clock, and scores in the game object
			gameUpdate.CurrentPeriod = fmt.Sprintf("%d", int(comp.Status.Period))
			if comp.Status.DisplayClock != "" {
				gameUpdate.DisplayClock = comp.Status.DisplayClock
			}
			gameUpdate.CurrentScore = scores
			logger.Info("Fetched game score", "gameID", game.ID, "period", gameUpdate.CurrentPeriod, "displayClock", gameUpdate.DisplayClock, "scores", gameUpdate.CurrentScore)
			return gameUpdate, nil
		}
	}

	return gameUpdate, fmt.Errorf("game not found: %s", game.ID)
}

func SendNotificationListActivity(ctx context.Context, sendNotifications SendNotifications) error {
	// For each notification message in the input list, send it to the specified channel in sendNotifications.Channel
	// NOTE: This means that if one notification in the list fails, the whole activity fails and none of the notifications are sent.
	// You could also do this with an activity per notification.
	logger := activity.GetLogger(ctx)
	logger.Info("Sending notifications to channel", "channel", sendNotifications.Channel)
	for _, notification := range sendNotifications.NotificationList {
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
func SendSlackNotification(ctx context.Context, notification Notification) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending Slack notification", "title", notification.Title, "message", notification.Message)

	slackBotToken := os.Getenv("SLACK_BOT_TOKEN")
	if slackBotToken == "" {
		return fmt.Errorf("SLACK_BOT_TOKEN environment variable is not set")
	}

	slackChannelID := os.Getenv("SLACK_CHANNEL_ID")
	if slackChannelID == "" {
		return fmt.Errorf("SLACK_CHANNEL_ID environment variable is not set")
	}

	api := slack.New(slackBotToken)
	attachment := slack.Attachment{
		Title: notification.Title,
		Text:  notification.Message,
		Color: "#444CE7", // Temporal UV
	}

	_, _, err := api.PostMessage(
		slackChannelID,
		slack.MsgOptionAttachments(attachment),
	)
	if err != nil {
		return fmt.Errorf("failed to send Slack message: %w", err)
	}
	return nil
}
