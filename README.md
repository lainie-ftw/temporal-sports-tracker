# Temporal Sports Tracker

A Temporal workflow system that monitors Big Ten college football games and sends Slack notifications when scores change.

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
- Monitor Big Ten conference games (conference ID: 5)
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

The `test.go` file includes functions to:
- Test fetching games from ESPN API
- Test the mock Slack notification system
- Start the weekly poller workflow

## ESPN API

The system uses the ESPN College Football API:
- Endpoint: `https://site.api.espn.com/apis/site/v2/sports/football/college-football/scoreboard`
- Filters for Big Ten conference games (conference ID: 5)
- Parses game data including teams, scores, and start times

## Slack Integration

Currently mocked in `SendSlackNotificationActivity`. To enable real Slack notifications:

1. Get a Slack webhook URL from your Slack app
2. Update the `SendSlackNotificationActivity` function to use HTTP POST to the webhook
3. Add proper error handling and authentication

## Error Handling

- **Activity Retries**: Uses Temporal's built-in retry policies with exponential backoff
- **Network Failures**: Automatic retries for API calls
- **Workflow Resilience**: Workflows continue running even if individual activities fail
- **Timeout Handling**: Activities have configurable timeouts

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
