package sports

import (
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

	//WorkflowRunTimeout: 6 * time.Hour

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

	// Initialize score tracking
	lastScores := make(map[string]string)
	for teamID, score := range game.CurrentScore {
		lastScores[teamID] = score
	}

	// Monitor game every minute using a loop with timers
	for workflow.Now(ctx).Before(game.StartTime.Add(4 * time.Hour)) {
		// Wait 1 minute before next poll
		timer := workflow.NewTimer(ctx, 1*time.Minute)
		selector := workflow.NewSelector(ctx)
		selector.AddFuture(timer, func(f workflow.Future) {
			// Timer fired, time to poll again
		})
		selector.Select(ctx)

		// Fetch current scores
		var currentScores map[string]string
		err := workflow.ExecuteActivity(ctx, GetGameScore, game.ID).Get(ctx, &currentScores)
		if err != nil {
			logger.Error("Failed to fetch game score", "gameID", game.ID, "error", err)
			continue
		}

		// Check for score changes
		scoreChanged := false
		for teamID, currentScore := range currentScores {
			if lastScore, exists := lastScores[teamID]; !exists || lastScore != currentScore {
				scoreChanged = true
				break
			}
		}

		// Send notification if score changed
		if scoreChanged {
			logger.Info("Score change detected", "gameID", game.ID)

			update := ScoreUpdate{
				GameID:    game.ID,
				HomeTeam:  game.HomeTeam.DisplayName,
				AwayTeam:  game.AwayTeam.DisplayName,
				HomeScore: currentScores[game.HomeTeam.ID],
				AwayScore: currentScores[game.AwayTeam.ID],
				Timestamp: workflow.Now(ctx),
			}

			err = workflow.ExecuteActivity(ctx, SendNotification, update).Get(ctx, nil)
			if err != nil {
				logger.Error("Failed to send Slack notification", "gameID", game.ID, "error", err)
			}

			// Update last scores
			for teamID, score := range currentScores {
				lastScores[teamID] = score
			}
		}
	}

	logger.Info("Game workflow completed", "gameID", game.ID)
	var finalScore string = "Final score: "
	return finalScore, nil
}
