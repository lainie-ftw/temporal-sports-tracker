# Temporal Sports Tracker

The Temporal Sports Tracker is a Temporal application that uses two different workflows to monitor sports (anything available via the ESPN Scoreboard API) and send notifications when certain things happen. 

Currently supported notification channels (can do any combination, default is logger):
- Home Assistant, via a webhook that triggers an automation (`hass`)
- Slack, via a Slack bot app that posts to a specific channel (`slack`)
- Workflow/Activity logger (`logger`)

Currently supported notification types (can do any combination, default is score_change):
- Score change (`score_change`)
- Game is in overtime (`overtime`)
- The underdog has started winning (`underdog`)

## Architecture

### Workflows
1. **WeeklyPollerWorkflow**: Runs weekly to fetch games and schedule individual game workflows
2. **GameWorkflow**: Monitors a single game, polls every minute, detects score changes

### Activities
1. **FetchGamesActivity**: Fetches games from ESPN API and filters for Big Ten games
2. **FetchGameScoreActivity**: Gets current score for a specific game
3. **SendSlackNotificationActivity**: Sends notifications (currently mocked)

## Setup (to run locally)

1. **Set up .env file**
```bash
cp .env.example .env
```
Modify values as necessary.

2. **Install Dependencies**
   ```bash
   go mod tidy
   ```

3. **Start Temporal Server** (if running locally)
   ```bash
   # Using Temporal CLI
   temporal server start-dev
   ```

4. **Start the Worker and the UI**
   ```bash
   go run worker/main.go
   go run cmd/web/main.go
   ```

## Setup to run Dockerized or deploy to the K8s of your choice

See [DEPLOYMENT.md](the Deployment README) for instructions!

## ESPN API

The system uses the ESPN Scoreboard API:
- Endpoint (for college football): `https://site.api.espn.com/apis/site/v2/sports/football/college-football/scoreboard`
- Parses game data including teams, scores, and start times
- Huge thanks to [Public ESPN API](https://github.com/pseudo-r/Public-ESPN-API) and the [Home Assistant Team Tracker Integration](https://github.com/vasqued2/ha-teamtracker) for info on how to use this API.

## Future Enhancements
- Add ability to have a recurring CollectGamesWorkflow that runs weekly for the duration of a season
- View notification types (score_change, overtime, underdog) and notification channels (hass, slack, logger) in the UI
- Set up notification types and notification channels in the UI
- Have different notification type(s) and channel(s) per team or conference
- Show completed games in the UI
- Modify GetGameScoreActivity to use the game endpoint instead of the scoreboard endpoint
