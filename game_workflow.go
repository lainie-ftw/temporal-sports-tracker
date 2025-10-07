package sports

import (
	"fmt"
	"os"
	"slices"
	"strconv"
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

	// Initialize score tracking
	lastScores := make(map[string]string)
	for teamID, score := range game.CurrentScore {
		lastScores[teamID] = score
	}

	// Initialize underdog tracking
	underdogWinning := false

	// Initialize overtime tracking to the number of regulation periods in the game
	lastOvertimePeriod := game.NumberOfPeriods

	// Monitor the game for 5 hours after start time - could be modified to check for the game status instead
	for workflow.Now(ctx).Before(game.StartTime.Add(5 * time.Hour)) {
		// Wait 5 minutes before next poll
		timer := workflow.NewTimer(ctx, 5*time.Minute)
		selector := workflow.NewSelector(ctx)
		selector.AddFuture(timer, func(f workflow.Future) {
			// Timer fired, time to poll again
		})
		selector.Select(ctx)

		var gameUpdate Game
		err := workflow.ExecuteActivity(ctx, GetGameScoreActivity, game).Get(ctx, &gameUpdate)
		if err != nil {
			logger.Error("Failed to fetch game score", "gameID", game.ID, "error", err)
			continue
		}

		game.CurrentScore = gameUpdate.CurrentScore
		game.CurrentPeriod = gameUpdate.CurrentPeriod
		game.DisplayClock = gameUpdate.DisplayClock

		// Check for score changes
		scoreChanged := false
		for teamID, currentScore := range game.CurrentScore {
			if lastScore, exists := lastScores[teamID]; !exists || lastScore != currentScore {
				scoreChanged = true
				break
			}
		}

		// Check for a new overtime
		newOvertime := false
		if game.CurrentPeriod != "" {
			currentPeriod, err := strconv.Atoi(game.CurrentPeriod)
			if err == nil && currentPeriod > lastOvertimePeriod {
				newOvertime = true
			}
		}

		notificationList := []Notification{}

		// Send notifications related to score changes if the score changed
		if scoreChanged  {

			if slices.Contains(notificationTypes, "score_change") {
				scoreUpdateNotification := buildScoreUpdateNotification(game)
				notificationList = append(notificationList, scoreUpdateNotification)
				logger.Info("Added score update notification", "gameID", game.ID)
			}

			if slices.Contains(notificationTypes, "underdog") {
				logger.Info("NotificationTypes contains underdog. Checking for underdog status", "gameID", game.ID)
				// We only want to send a notification when the underdog pulls ahead
				underdogTeam := determineUnderdog(game)
				if underdogTeam != "No underdog." {
					if game.CurrentScore[game.HomeTeam.ID] > game.CurrentScore[game.AwayTeam.ID] && game.HomeTeam.Underdog {
						underdogWinning = true
					} else if game.CurrentScore[game.AwayTeam.ID] > game.CurrentScore[game.HomeTeam.ID] && game.AwayTeam.Underdog {
						underdogWinning = true
					} else {
						underdogWinning = false
					}
				}

				if underdogWinning {
					underdogNotification := buildUnderdogNotification(game, underdogTeam)
					notificationList = append(notificationList, underdogNotification)
					logger.Info("Added underdog notification", "gameID", game.ID)
				}
			}

			logger.Info("Score change detected", "gameID", game.ID)

			// Update last scores - maybe move this so it only updates if the notifications are sent successfully?
			for teamID, score := range game.CurrentScore {
				lastScores[teamID] = score
			}
		}

		// Send overtime notification if the game has gone into a new overtime period
		if newOvertime && slices.Contains(notificationTypes, "overtime") {
			overtimeNotification := buildOvertimeNotification(game)
			notificationList = append(notificationList, overtimeNotification)
			logger.Info("Added overtime notification", "gameID", game.ID)
			
			// Update last overtime period
			currentPeriod, err := strconv.Atoi(game.CurrentPeriod)
			if err == nil {
				lastOvertimePeriod = currentPeriod
			}
		}

		// If there are notifications to send, send them
		if len(notificationList) > 0 {
			logger.Info("Notifications to send", "count", len(notificationList), "notifications", notificationList)
			
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
		}
	}

	logger.Info("Game workflow completed", "gameID", game.ID)
	var finalScore string = fmt.Sprintf("Final score: %s %s - %s %s", game.HomeTeam.Abbreviation, game.CurrentScore[game.HomeTeam.ID], game.AwayTeam.Abbreviation, game.CurrentScore[game.AwayTeam.ID])
	return finalScore, nil
}

func buildScoreUpdateNotification(game Game) Notification {
	notification := Notification{}
	periodString := getPeriodStr(game.NumberOfPeriods, game.Sport)

	// Score update notification looks like this:
		// Score Update!
		// Michigan Wolverines vs. Ohio State Buckeyes
		// Score: MICH 100 - OSU 0
		// Q3, 12:34 left on ESPN
	notification.Title = "Score Update!"
	notification.Message = fmt.Sprintf("\n%s vs %s\nScore: %s %s - %s %s\n%s, %s left on %s", 
		game.HomeTeam.DisplayName, game.AwayTeam.DisplayName, game.HomeTeam.Abbreviation, game.CurrentScore[game.HomeTeam.ID], game.AwayTeam.Abbreviation, game.CurrentScore[game.AwayTeam.ID], periodString, game.DisplayClock, game.TVNetwork)

	return notification
}

func buildUnderdogNotification(game Game, underdogTeam string) Notification {
	periodString := getPeriodStr(game.NumberOfPeriods, game.Sport)
	notification := Notification{}
	
	// Underdog notification looks like this:
		// Team Chaos!
		// UCF Knights are winning in the UCF Knights vs. South Florida Bulls game on ESPN! It's currently Q2 with 10:15 left.
		// Score: UCF 14 - USF 7
	notification.Title = "Team Chaos!"

	notification.Message = fmt.Sprintf("%s are winning in the %s vs. %s game on %s! It's currently %s with %s left. \nScore: %s %s - %s %s", 
		underdogTeam, game.HomeTeam.DisplayName, game.AwayTeam.DisplayName, game.TVNetwork, periodString, game.DisplayClock, game.HomeTeam.Abbreviation, game.CurrentScore[game.HomeTeam.ID], game.AwayTeam.Abbreviation, game.CurrentScore[game.AwayTeam.ID])

	return notification
}

func buildOvertimeNotification(game Game) Notification {
	notification := Notification{}

	currentPeriod, err := strconv.Atoi(game.CurrentPeriod)

	if err != nil {
		// If we can't parse the current period, just return a generic notification
		notification.Title = "Overtime!"
		notification.Message = fmt.Sprintf("The game between the %s and the %s is in overtime on %s!\nScore: %s %s - %s %s", 
			game.HomeTeam.DisplayName, game.AwayTeam.DisplayName, game.TVNetwork, game.HomeTeam.Abbreviation, game.CurrentScore[game.HomeTeam.ID], game.AwayTeam.Abbreviation, game.CurrentScore[game.AwayTeam.ID])
		return notification
	}

	//Calculate which overtime we're in - current period minus number of periods for this game.
	overtimeNumber := currentPeriod - game.NumberOfPeriods
	overtimeStr := ""
	switch overtimeNumber {
		case 1:
			overtimeStr = "OT"
		case 2:
			overtimeStr = "Double OT"
		case 3:
			overtimeStr = "TRIPLE OT"
		default:
			overtimeStr = fmt.Sprintf("%dth OT", overtimeNumber)
	}

	// Overtime notification looks like this:
		// Double OT!
		// The game between the Michigan Wolverines and the Ohio State Buckeyes is in Double OT on NBC!
		// Score: MICH 27 - OSU 27
	notification.Title = fmt.Sprintf("%s!", overtimeStr)

	notification.Message = fmt.Sprintf("The game between the %s and the %s is in %s on %s!\nScore: %s %s - %s %s", 
		game.HomeTeam.DisplayName, game.AwayTeam.DisplayName, overtimeStr, game.TVNetwork, game.HomeTeam.Abbreviation, game.CurrentScore[game.HomeTeam.ID], game.AwayTeam.Abbreviation, game.CurrentScore[game.AwayTeam.ID])

	return notification
}

func getPeriodStr(period int, sport string) string {
	switch sport {
	case "baseball":
		return fmt.Sprintf("Inning %d", period)
	case "hockey":
		switch period {
		case 1:
			return "1st Period"
		case 2:
			return "2nd Period"
		case 3:
			return "3rd Period"
		}
	case "soccer":
		return fmt.Sprintf("Half %d", period)	
	}
	return fmt.Sprintf("Q%d", period) // default to quarters for other sports
}

func determineUnderdog(game Game) (string) {
	if game.HomeTeam.Underdog {
		return game.HomeTeam.DisplayName
	} else if game.AwayTeam.Underdog {
		return game.AwayTeam.DisplayName
	}
	return "No underdog."
}
