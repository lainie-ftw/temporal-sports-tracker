package sports

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// CollectGamesWorkflow collects all games based on input and schedules each game as a GameWorkflow
func CollectGamesWorkflow(ctx workflow.Context, trackingRequest TrackingRequest) (int, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting Collect Games Workflow.")

	// Set up activity options with retry policy
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	// Fetch games from ESPN API
	var games []Game
	err := workflow.ExecuteActivity(ctx, GetGamesActivity, trackingRequest).Get(ctx, &games)
	if err != nil {
		logger.Error("Failed to fetch games", "error", err)
		return 0, err
	}

	logger.Info("Fetched games", "count", len(games))

	// Schedule game workflows for upcoming games
	for _, game := range games {
		// Only schedule games that haven't started yet
		if game.Status == "pre" && game.StartTime.After(workflow.Now(ctx)) {
			err := workflow.ExecuteActivity(ctx, StartGameWorkflowActivity, game).Get(ctx, nil)
			if err != nil {
				logger.Error("Failed to start game workflow", "gameID", game.ID, "error", err)
				return 0, err
			}
		}
	}

	var totalGames = len(games)
	logger.Info("Collect Games Workflow completed.")
	return totalGames, nil
}