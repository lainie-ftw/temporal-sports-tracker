package sports

import "time"

// ESPN API Response Models
type ESPNResponse struct {
	Events  []Event  `json:"events"`
}

type League struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Abbreviation string `json:"abbreviation"`
}


type Event struct {
	ID          string    `json:"id"`
	Date        ESPNTime  `json:"date"`
	Name        string    `json:"name"`
	ShortName   string    `json:"shortName"`
	Week        Week      `json:"week"`
	Competitions []Competition `json:"competitions"`
	Status      Status    `json:"status"`
}

type Week struct {
	Number int `json:"number"`
}

type Competition struct {
	ID         string        `json:"id"`
	Date       ESPNTime      `json:"date"`
	Competitors []Competitor `json:"competitors"`
	Odds       []Odd         `json:"odds"`
	Status     Status        `json:"status"`
}

type Competitor struct {
	ID     string `json:"id"`
	Team   Team   `json:"team"`
	Score  string `json:"score"`
	HomeAway string `json:"homeAway"`
}

type Team struct {
	ID            string `json:"id"`
	Location      string `json:"location"`
	Name          string `json:"name"`
	Abbreviation  string `json:"abbreviation"`
	DisplayName   string `json:"displayName"`
	ConferenceId  string `json:"conferenceId"`
	Favorite      bool
	Underdog      bool
}

type Status struct {
	Clock       float64 `json:"clock"`
	DisplayClock string `json:"displayClock"`
	Period      int    `json:"period"`
	Type        StatusType `json:"type"`
}

type StatusType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	State       string `json:"state"`
	Completed   bool   `json:"completed"`
	Description string `json:"description"`
}

 
// Odd represents betting odds information for a competition
type Odd struct {
	Details       string    `json:"details"` // Abbreviation that indicates the projected winner and how many points they'll win by, i.e. "MICH -7.5" = U of M will win by 7.5 points
	OverUnder     float64   `json:"overUnder"`
	HomeTeamOdds  *TeamOdds `json:"homeTeamOdds,omitempty"`
	AwayTeamOdds  *TeamOdds `json:"awayTeamOdds,omitempty"`
}

// TeamOdds represents odds information for a specific team in a matchup
type TeamOdds struct {
	Favorite  bool    `json:"favorite,omitempty"`
	Underdog  bool    `json:"underdog,omitempty"`
}

// Game represents a simplified game structure for our workflow
type Game struct {
	ID           string
	EventID      string
	HomeTeam     Team
	AwayTeam     Team
	StartTime    time.Time
	CurrentScore map[string]string // team ID -> score
	Status       string
	Odds         string
}

// ScoreUpdate represents a score change notification
type ScoreUpdate struct {
	GameID      string
	HomeTeam    string
	AwayTeam    string
	HomeScore   string
	AwayScore   string
	Timestamp   time.Time
}

// TrackingRequest represents the request to start tracking
type TrackingRequest struct {
	Sport       string   `json:"sport"`
	League      string   `json:"league"`
	Teams       []string `json:"teams"`
	Conferences []string `json:"conferences"`
}