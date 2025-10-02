# Temporal Sports Tracker

A Temporal workflow system that monitors Big Ten college football games and sends Slack notifications when scores change.

## To Build and Run Locally (Docker)
docker build -f Dockerfile.web .
docker build -f Dockerfile.worker .

docker run --env-file .env temporal-sports-tracker-worker
docker run --env-file .env temporal-sports-tracker-web

## To Build and Run Locally (No Docker)
go run worker/main.go
go run web/main.go

## To Do
- Add real notifications 
- (?) Add updates pushed to UI with scores
- Add SDK metrics
- Add SDK metrics to a dashboard
-- https://github.com/temporalio/dashboards/tree/master/sdk
-- https://community.temporal.io/t/sdk-metrics-in-go/8564/3 
-- https://github.com/temporalio/samples-go/tree/main/metrics
-- https://docs.temporal.io/cloud/metrics/prometheus-grafana#grafana-data-sources-configuration
- Add cloud metrics v2 to a dashboard
- Update this README to be accurate after all the changes...

## Features

- **Weekly Game Polling**: Automatically fetches upcoming Big Ten games from ESPN API every week
- **Real-time Score Monitoring**: Monitors games every minute during active play
- **Score Change Notifications**: Sends Slack notifications (currently mocked) when scores change
- **Temporal Workflow Reliability**: Uses Temporal's built-in retry policies and workflow management
- **Big Ten Conference Focus**: Filters games to only monitor Big Ten conference teams

## Architecture

### Workflows
1. **WeeklyPollerWorkflow**: Runs weekly to fetch games and schedule individual game workflows
2. **GameWorkflow**: Monitors a single game, polls every minute, detects score changes

### Activities
1. **FetchGamesActivity**: Fetches games from ESPN API and filters for Big Ten games
2. **FetchGameScoreActivity**: Gets current score for a specific game
3. **SendSlackNotificationActivity**: Sends notifications (currently mocked)

## Prerequisites

- Go 1.25.0 or later
- Temporal server running locally (or remote Temporal cluster)
- Internet connection for ESPN API access

## Setup

1. **Install Dependencies**:
   ```bash
   go mod tidy
   ```

2. **Start Temporal Server** (if running locally):
   ```bash
   # Using Temporal CLI
   temporal server start-dev
   ```

3. **Run the Worker**:
   ```bash
   go run .
   ```

## Configuration

The system is configured to:
- Monitor games as selected in the UI (by conference or team name)
- Poll games every minute during active play
- Use Temporal's built-in retry policies for resilience
- Send mock Slack notifications (update `SendSlackNotificationActivity` for real Slack integration)

## Usage

1. **Start the System**:
   ```bash
   go run .
   ```

2. **Trigger Weekly Poller** (in another terminal):
   ```bash
   go run test.go
   ```
   This will start the weekly poller workflow and test the activities.

## Testing

- TODO: add this info

## ESPN API

The system uses the ESPN College Football API:
- Endpoint: `https://site.api.espn.com/apis/site/v2/sports/football/college-football/scoreboard`
- Parses game data including teams, scores, and start times

## Slack Integration

Currently mocked in `SendSlackNotificationActivity`. To enable real Slack notifications:

1. Get a Slack webhook URL from your Slack app
2. Update the `SendSlackNotificationActivity` function to use HTTP POST to the webhook
3. Add proper error handling and authentication

## Monitoring

The system logs:
- Game fetching activities
- Score changes detected
- Notification sending (mocked)
- Workflow progress and errors

## Future Enhancements

- Real Slack webhook integration
- Additional conference support
- Game completion detection from API
- Score prediction notifications
- Multiple notification channels (email, SMS, etc.)
- Dashboard for game monitoring
- Historical score tracking
- Support for all teams using the /team API (instead of /scoreboard, which only has the top 25)

## Dependencies

- `go.temporal.io/sdk`: Temporal Go SDK for workflow management
- `github.com/sirupsen/logrus`: Structured logging
- Standard Go libraries for HTTP and JSON handling

## Notes
- Why not child workflows? Largely because of the timing - if I use the scheduling workflow to create children ahead of time, in a loop, it will only create one unless I set up the children as abandoned workflows. At that point, might as well just have different workflows entirely and use...signal with start?? Additionally, because I don't need to maintain the relationship between the scheduler and the games.
