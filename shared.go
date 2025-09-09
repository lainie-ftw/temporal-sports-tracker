package sports

const TaskQueueName = "sports-tracker-task-queue"

//All ESPN APIs use a URL in the following format: https://site.api.espn.com/apis/site/v2/sports/{SPORT_PATH}/{LEAGUE_PATH}/scoreboard 
// where {SPORT_PATH} is the sport and {LEAGUE_PATH} is the league that the team plays in.
//For example, for the NFL, the URL is https://site.api.espn.com/apis/site/v2/sports/football/nfl/scoreboard