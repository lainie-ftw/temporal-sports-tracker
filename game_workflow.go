package sports

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// GameWorkflow monitors a single game and sends notifications on score changes
func GameWorkflow(ctx workflow.Context, game Game) (string, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting Game Workflow", "gameID", game.ID, "homeTeam", game.HomeTeam.DisplayName, "awayTeam", game.AwayTeam.DisplayName)

	// Query handler for UI - return the game info
	err := workflow.SetQueryHandler(ctx, "gameInfo", func() (Game, error) {
		return game, nil
	})
	if err != nil {
		logger.Error("Failed to set query handler", "error", err)
		return "", err
	}

	// Set up activity options with retry policy
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	// Wait until game starts
	gameStartTime := game.StartTime
	if gameStartTime.After(workflow.Now(ctx)) {
		logger.Info("Waiting for game to start", "gameID", game.ID, "startTime", gameStartTime)
		timerCtx, cancelTimer := workflow.WithCancel(ctx)
		timer := workflow.NewTimer(timerCtx, gameStartTime.Sub(workflow.Now(ctx)))
		selector := workflow.NewSelector(ctx)
		selector.AddFuture(timer, func(f workflow.Future) {
			// Timer fired, game should be starting
		})
		selector.Select(ctx)
		cancelTimer()
	}

	logger.Info("Game monitoring started", "gameID", game.ID)

	// Grab notification types and channels requested
	notificationTypesStr := os.Getenv("NOTIFICATION_TYPES")
	var notificationTypes []string
	if notificationTypesStr == "" {
		notificationTypes = []string{"score_change"} // if not set, default to notifying if the score changes
	} else {
		notificationTypes = strings.Split(notificationTypesStr, ",")
	}

	notificationChannelsStr := os.Getenv("NOTIFICATION_CHANNELS")
	var notificationChannels []string
	if notificationChannelsStr == "" {
		notificationChannels = []string{"logger"} // if not set, default to just logging the message
	} else {
		notificationChannels = strings.Split(notificationChannelsStr, ",")
	}

	underdogWinning := false

	// Initialize score tracking
	lastScores := make(map[string]string)
	for teamID, score := range game.CurrentScore {
		lastScores[teamID] = score
	}

	// Monitor the game for 5 hours after start time - could be modified to check for the game status instead
	for workflow.Now(ctx).Before(game.StartTime.Add(5 * time.Hour)) {
		// Wait 5 minutes before next poll
		timer := workflow.NewTimer(ctx, 5*time.Minute)
		selector := workflow.NewSelector(ctx)
		selector.AddFuture(timer, func(f workflow.Future) {
			// Timer fired, time to poll again
		})
		selector.Select(ctx)

		// Fetch current score - this will update game.CurrentScore
		err := workflow.ExecuteActivity(ctx, GetGameScoreActivity, game).Get(ctx, nil)
		if err != nil {
			logger.Error("Failed to fetch game score", "gameID", game.ID, "error", err)
			continue
		}

		// Check for score changes
		scoreChanged := false
		for teamID, currentScore := range game.CurrentScore {
			if lastScore, exists := lastScores[teamID]; !exists || lastScore != currentScore {
				scoreChanged = true
				break
			}
		}

		// Send notification if score changed
		if scoreChanged {

			notificationList := []Notification{}

			if slices.Contains(notificationTypes, "score_update") {
				scoreUpdateNotification := buildScoreUpdateNotification(game)
				notificationList = append(notificationList, scoreUpdateNotification)
			}

			if slices.Contains(notificationTypes, "underdog") {
				// We only want to send a notification when the underdog pulls ahead
				underdogTeam := determineUnderdog(game)
				if !underdogWinning {
					if underdogTeam != "No underdog." {
						if game.CurrentScore[game.HomeTeam.ID] > game.CurrentScore[game.AwayTeam.ID] && game.HomeTeam.Underdog {
							underdogWinning = true
						} else if game.CurrentScore[game.AwayTeam.ID] > game.CurrentScore[game.HomeTeam.ID] && game.AwayTeam.Underdog {
							underdogWinning = true
						}
					}
				}

				if underdogWinning {
					underdogNotification := buildUnderdogNotification(game, underdogTeam)
					notificationList = append(notificationList, underdogNotification)
				}
			}

			logger.Info("Score change detected", "gameID", game.ID)

			// For each notification channel, send the collected list of notifications:
			for channel := range notificationChannels {
				sendNotifications := SendNotifications{
					Channel: notificationChannels[channel],
					NotificationList: notificationList,
				}
		
				err = workflow.ExecuteActivity(ctx, SendNotificationListActivity, sendNotifications).Get(ctx, nil)
		
				if err != nil {
					logger.Error("Failed to send notification", "gameID", game.ID, "error", err)
				}
			}

			// Update last scores
			for teamID, score := range game.CurrentScore {
				lastScores[teamID] = score
			}
		}
	}

	logger.Info("Game workflow completed", "gameID", game.ID)
	var finalScore string = fmt.Sprintf("Final score: %s %s - %s %s", game.HomeTeam.Abbreviation, game.CurrentScore[game.HomeTeam.ID], game.AwayTeam.Abbreviation, game.CurrentScore[game.AwayTeam.ID])
	return finalScore, nil
}

func buildScoreUpdateNotification(game Game) Notification {
	notification := Notification{}
	notification.Title = "Score Update!"

	notification.Message = fmt.Sprintf("🏈 Score Update!\n%s vs %s\nScore: %s %s - %s %s", 
		game.HomeTeam.Name, game.AwayTeam.Name, game.HomeTeam.Abbreviation, game.CurrentScore[game.HomeTeam.ID], game.AwayTeam.Abbreviation, game.CurrentScore[game.AwayTeam.ID])

	return notification
}

func buildUnderdogNotification(game Game, underdogTeam string) Notification {
	notification := Notification{}
	
	//TODO: add conference, sport, RemainingTime
	//title := "[update.conference] [update.sport]: Team Chaos!"
	notification.Title = "Team Chaos!"

	notification.Message = fmt.Sprintf("%s is winning in the %s vs. %s game on %s! It's currently Q%s with [some time] left. \nScore: %s %s - %s %s", 
		underdogTeam, game.HomeTeam.Name, game.AwayTeam.Name, game.TVNetwork, game.Quarter, game.HomeTeam.Abbreviation, game.CurrentScore[game.HomeTeam.ID], game.AwayTeam.Abbreviation, game.CurrentScore[game.AwayTeam.ID])

	return notification
}

func determineUnderdog(game Game) (string) {
	if game.HomeTeam.Underdog {
		return game.HomeTeam.DisplayName
	} else if game.AwayTeam.Underdog {
		return game.AwayTeam.DisplayName
	}
	return "No underdog."
}
