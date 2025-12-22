package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	sports "temporal-sports-tracker"
	"time"

	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

type Handlers struct {
	temporalClient client.Client
}

func NewHandlers(temporalClient client.Client) *Handlers {
	return &Handlers{
		temporalClient: temporalClient,
	}
}

// Sport represents a sport available in ESPN API
type Sport struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// League represents a league within a sport
type League struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// Conference represents a conference within a league
type Conference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GameWorkflow represents running workflow information
type GameWorkflow struct {
	WorkflowID string    `json:"workflowId"`
	RunID      string    `json:"runId"`
	WorkflowURL string    `json:"workflowUrl,omitempty"`
	Status     string    `json:"status"`
	HomeTeam  string    `json:"homeTeam"`
	HomeScore string    `json:"homeScore"`
	AwayTeam  string    `json:"awayTeam"`
	AwayScore string    `json:"awayScore"`
	StartTime time.Time `json:"startTime"`
	GameID   string    `json:"gameId"`
}

// GetSports returns available sports from ESPN API
func (h *Handlers) GetSports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Predefined list of supported ESPN sports
	sports := []Sport{
		{ID: "baseball", Name: "Baseball", Path: "baseball"},
		{ID: "basketball", Name: "Basketball", Path: "basketball"},
		{ID: "esports", Name: "eSports", Path: "esports"},
		{ID: "football", Name: "Football", Path: "football"},
		{ID: "hockey", Name: "Hockey", Path: "hockey"},
		{ID: "soccer", Name: "Soccer", Path: "soccer"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sports)
}

// GetLeagues returns available leagues for a sport
func (h *Handlers) GetLeagues(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sportPath := strings.TrimPrefix(r.URL.Path, "/api/leagues/")
	if sportPath == "" {
		http.Error(w, "Sport required", http.StatusBadRequest)
		return
	}

	var leagues []League
	switch sportPath {
	case "football":
		leagues = []League{
			{ID: "nfl", Name: "NFL", Path: "nfl"},
			{ID: "college-football", Name: "College Football", Path: "college-football"},
		}
	case "basketball":
		leagues = []League{
			{ID: "nba", Name: "NBA", Path: "nba"},
			{ID: "mens-college-basketball", Name: "Men's College Basketball", Path: "mens-college-basketball"},
			{ID: "womens-college-basketball", Name: "Women's College Basketball", Path: "womens-college-basketball"},
		}
	case "baseball":
		leagues = []League{
			{ID: "mlb", Name: "MLB", Path: "mlb"},
		}
	case "hockey":
		leagues = []League{
			{ID: "nhl", Name: "NHL", Path: "nhl"},
		}
	case "soccer":
		leagues = []League{
			{ID: "usa.1", Name: "MLS", Path: "usa.1"},
			{ID: "eng.1", Name: "English Premier League", Path: "eng.1"},
			{ID: "uefa.champions", Name: "UEFA Champions League", Path: "uefa.champions"},
		}
	case "esports":
		leagues = []League{
			{ID: "cs2", Name: "CS2", Path: "cs2"},
		}
	default:
		http.Error(w, "Unsupported sport", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(leagues)
}

// GetTeams fetches teams for a specific sport/league from ESPN API or Liquipedia
func (h *Handlers) GetTeams(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/teams/"), "/")
	if len(pathParts) < 2 {
		http.Error(w, "Sport and league required", http.StatusBadRequest)
		return
	}

	sport := pathParts[0]
	league := pathParts[1]

	// Handle eSports CS2 separately - scrape from Liquipedia
	if sport == "esports" && league == "cs2" {
		teams, err := getCS2TeamsFromLiquipedia()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to fetch CS2 teams: %v", err), http.StatusInternalServerError)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(teams)
		return
	}

	// Handle ESPN sports
	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/%s/%s/scoreboard", sport, league)
	
	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, "Failed to fetch teams", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	var espnResp sports.ESPNResponse
	if err := json.Unmarshal(body, &espnResp); err != nil {
		http.Error(w, "Failed to parse ESPN response", http.StatusInternalServerError)
		return
	}

	// Extract unique teams
	teamMap := make(map[string]sports.Team)
	for _, event := range espnResp.Events {
		for _, comp := range event.Competitions {
			for _, competitor := range comp.Competitors {
				team := competitor.Team
				teamMap[team.ID] = sports.Team{
					ID:           team.ID,
					Name:         team.Name,
					DisplayName:  team.DisplayName,
					Abbreviation: team.Abbreviation,
					ConferenceId: team.ConferenceId,
				}
			}
		}
	}

	// Convert map to slice
	var teams []sports.Team
	for _, team := range teamMap {
		teams = append(teams, team)
	}
	
	// Sort teams alphabetically by DisplayName
	sort.Slice(teams, func(i, j int) bool {
		return teams[i].DisplayName < teams[j].DisplayName
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(teams)
}

// scrapeCS2TeamsFromLiquipedia fetches Notable Active CS2 teams
// Uses a curated list to ensure only active teams are returned
func getCS2TeamsFromLiquipedia() ([]sports.Team, error) {
	// Curated list of best CS2 teams
	//TODO: read the main page and add teams that have active games coming up
	notableTeamNames := []string{
		"FaZe Clan", "G2 Esports", "Astralis", "Passion UA", "FURIA", 
		"Team Liquid", "Team Falcons",
	}

	teams := []sports.Team{}
	
	for _, teamName := range notableTeamNames {
		teamID := strings.ReplaceAll(teamName, " ", "_")
		
		// Create abbreviation from first letters of words
		words := strings.Fields(teamName)
		abbreviation := ""
		for _, word := range words {
			if len(word) > 0 {
				abbreviation += strings.ToUpper(string(word[0]))
			}
		}
		if abbreviation == "" || len(abbreviation) > 5 {
			// If no abbreviation or too long, use first 4 chars
			if len(teamID) >= 4 {
				abbreviation = strings.ToUpper(teamID[:4])
			} else {
				abbreviation = strings.ToUpper(teamID)
			}
		}
		
		teams = append(teams, sports.Team{
			ID:           teamID,
			Name:         teamName,
			DisplayName:  teamName,
			Abbreviation: abbreviation,
		})
	}

	// Sort teams alphabetically by DisplayName
	sort.Slice(teams, func(i, j int) bool {
		return teams[i].DisplayName < teams[j].DisplayName
	})

	return teams, nil
}

// GetConferences returns available conferences for a sport/league
func (h *Handlers) GetConferences(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/conferences/"), "/")
	if len(pathParts) < 2 {
		http.Error(w, "Sport and league required", http.StatusBadRequest)
		return
	}

	league := pathParts[1]

	// For now, return predefined conferences for college sports
	var conferences []Conference
	if league == "college-football" {
		conferences = []Conference{
			{ID: "5", Name: "Big Ten"},
			{ID: "8", Name: "SEC"},
			{ID: "1", Name: "ACC"},
			{ID: "4", Name: "Big 12"},
			{ID: "151", Name: "American"}, 
			{ID: "15", Name: "MAC"}, 
			{ID: "17", Name: "Mountain West"},
			{ID: "20", Name: "Sun Belt"},
		}
	}

	if league == "mens-college-basketball" || league == "womens-college-basketball" {
		conferences = []Conference{
			{ID: "7", Name: "Big Ten"},
			{ID: "23", Name: "SEC"},
			{ID: "2", Name: "ACC"},
			{ID: "7", Name: "Big 12"},
			{ID: "62", Name: "American"}, 
			{ID: "14", Name: "MAC"}, 
			{ID: "44", Name: "Mountain West"},
			{ID: "27", Name: "Sun Belt"},
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(conferences)
}

// StartTracking starts tracking workflows for selected teams/conferences
func (h *Handlers) StartTracking(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req sports.TrackingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Default to "once" if not specified
	if req.ScheduleType == "" {
		req.ScheduleType = "once"
	}

	// Check if Temporal client is available
	if h.temporalClient == nil {
		response := map[string]string{
			"workflowId": "demo-workflow-" + time.Now().Format("20060102-150405"),
			"runId":      "demo-run-" + time.Now().Format("150405"),
			"message":    "Demo mode: Tracking request received (Temporal server not connected)",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	TaskQueueName := os.Getenv("TASK_QUEUE")
	if TaskQueueName == "" {
		http.Error(w, "TASK_QUEUE environment variable is not set", http.StatusInternalServerError)
		return
	}

	// Handle based on schedule type
	if req.ScheduleType == "once" {
		// One-time execution (original behavior)
		workflowID := fmt.Sprintf("sports-%s", time.Now().Format("20060102-150405"))

		options := client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: TaskQueueName,
		}
		
		we, err := h.temporalClient.ExecuteWorkflow(context.Background(), options, sports.CollectGamesWorkflow, req)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to start workflow: %v", err), http.StatusInternalServerError)
			return
		}

		response := map[string]string{
			"workflowId": we.GetID(),
			"runId":      we.GetRunID(),
			"message":    "Tracking started successfully",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	} else {
		// Create a schedule (daily or weekly)
		scheduleID := fmt.Sprintf("collect-games-%s-%s-%s-%s", 
			req.ScheduleType, req.Sport, req.League, time.Now().Format("20060102-150405"))

		var cronExpression string
		switch req.ScheduleType {
			case "daily":
				cronExpression = "0 1 * * *" // 1AM every day
			case "weekly":
				cronExpression = "0 1 * * 1" // 1AM every Monday
			default:
				http.Error(w, "Invalid schedule type. Must be 'once', 'daily', or 'weekly'", http.StatusBadRequest)
				return
		}

		scheduleClient := h.temporalClient.ScheduleClient()
		_, err := scheduleClient.Create(context.Background(), client.ScheduleOptions{
			ID: scheduleID,
			Spec: client.ScheduleSpec{
				CronExpressions: []string{cronExpression},
				TimeZoneName:    "America/New_York",
			},
			Action: &client.ScheduleWorkflowAction{
				Workflow:  sports.CollectGamesWorkflow,
				Args:      []interface{}{req},
				TaskQueue: TaskQueueName,
			},
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create schedule: %v", err), http.StatusInternalServerError)
			return
		}

		response := map[string]string{
			"scheduleId": scheduleID,
			"message":    fmt.Sprintf("Schedule created successfully (%s)", req.ScheduleType),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// GetWorkflows returns currently running workflows
func (h *Handlers) GetWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var gameWorkflows []GameWorkflow

	// Check if Temporal client is available
	if h.temporalClient == nil {
		// Return empty list in demo mode
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(gameWorkflows)
		return
	}

	// List workflows using the Temporal Go SDK
	// Query for running workflows with game- prefix (GameWorkflows)
	listRequest := &workflowservice.ListWorkflowExecutionsRequest{
		Query: "WorkflowId STARTS_WITH 'game-' AND ExecutionStatus = 'Running'",
	}

	resp, err := h.temporalClient.ListWorkflow(context.Background(), listRequest)
	if err != nil {
		// Log error but don't fail the request - return empty list
		fmt.Printf("Failed to list workflows: %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(gameWorkflows)
		return
	}

	// Process the workflow executions
	for _, execution := range resp.Executions {
		workflow := GameWorkflow{
			WorkflowID: execution.Execution.WorkflowId,
			RunID:      execution.Execution.RunId,
			Status:     execution.Status.String(),
		}
		
		var tempURL = fmt.Sprintf("/namespaces/%s/workflows/%s/%s", os.Getenv("TEMPORAL_NAMESPACE"), workflow.WorkflowID, workflow.RunID)

		// Add http or https and UI URL, based on TEMPORAL_HOST
		if os.Getenv("TEMPORAL_HOST") != "localhost:7233" {
			workflow.WorkflowURL = fmt.Sprintf("https://cloud.temporal.io%s", tempURL)
		} else {
			workflow.WorkflowURL = fmt.Sprintf("http://localhost:8233%s", tempURL)
		}

		// Get the info about the game from the gameInfo query in GameWorkflow
		var gameInfo sports.Game
		gameInfoResult, err := h.temporalClient.QueryWorkflow(context.Background(), workflow.WorkflowID, workflow.RunID, "gameInfo")
		if err != nil {
			fmt.Printf("Failed to query workflow %s: %v\n", workflow.WorkflowID, err)
		}
		err = gameInfoResult.Get(&gameInfo)
		if err != nil {
			fmt.Printf("Failed to get query result for workflow %s: %v\n", workflow.WorkflowID, err)
		}
		workflow.HomeTeam = gameInfo.HomeTeam.DisplayName
		workflow.HomeScore = gameInfo.CurrentScore[gameInfo.HomeTeam.ID]
		workflow.AwayTeam = gameInfo.AwayTeam.DisplayName
		workflow.AwayScore = gameInfo.CurrentScore[gameInfo.AwayTeam.ID]
		workflow.StartTime = gameInfo.StartTime
		workflow.GameID = gameInfo.ID

		gameWorkflows = append(gameWorkflows, workflow)
	}

	// Sort workflows by StartTime
	sort.Slice(gameWorkflows, func(i, j int) bool {
		return gameWorkflows[i].StartTime.Before(gameWorkflows[j].StartTime)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(gameWorkflows)
}

// ManageWorkflow handles workflow management (cancel, etc.)
func (h *Handlers) ManageWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := strings.TrimPrefix(r.URL.Path, "/api/workflows/")
	if workflowID == "" {
		http.Error(w, "Workflow ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		// Check if Temporal client is available
		if h.temporalClient == nil {
			response := map[string]string{
				"message": "Demo mode: Workflow cancel request received (Temporal server not connected)",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Cancel workflow
		err := h.temporalClient.CancelWorkflow(context.Background(), workflowID, "")
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to cancel workflow: %v", err), http.StatusInternalServerError)
			return
		}
		
		response := map[string]string{
			"message": "Workflow cancelled successfully",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ScheduleInfo represents a schedule's information for display
type ScheduleInfo struct {
	ScheduleID   string    `json:"scheduleId"`
	Sport        string    `json:"sport"`
	League       string    `json:"league"`
	Teams        []string  `json:"teams"`
	Conferences  []string  `json:"conferences"`
	ScheduleType string    `json:"scheduleType"`
	NextRunTime  time.Time `json:"nextRunTime"`
	Paused       bool      `json:"paused"`
}

// GetSchedules returns currently active schedules
func (h *Handlers) GetSchedules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var schedules []ScheduleInfo

	// Check if Temporal client is available
	if h.temporalClient == nil {
		// Return empty list in demo mode
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(schedules)
		return
	}

	scheduleClient := h.temporalClient.ScheduleClient()
	
	// List schedules
	iter, err := scheduleClient.List(context.Background(), client.ScheduleListOptions{})
	if err != nil {
		fmt.Printf("Failed to list schedules: %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(schedules)
		return
	}

	// Process each schedule
	for iter.HasNext() {
		entry, err := iter.Next()
		if err != nil {
			fmt.Printf("Error iterating schedules: %v\n", err)
			continue
		}

		// Only include schedules that match our pattern
		if !strings.HasPrefix(entry.ID, "collect-games-") {
			continue
		}

		// Get schedule handle to retrieve details
		handle := scheduleClient.GetHandle(context.Background(), entry.ID)
		desc, err := handle.Describe(context.Background())
		if err != nil {
			fmt.Printf("Failed to describe schedule %s: %v\n", entry.ID, err)
			continue
		}

		// Extract tracking request from schedule action
		var trackingReq sports.TrackingRequest
		if workflowAction, ok := desc.Schedule.Action.(*client.ScheduleWorkflowAction); ok {
			if len(workflowAction.Args) > 0 {
				// The arg is a Temporal payload with base64 encoded data
				// We need to decode it properly
				jsonBytes, err := json.Marshal(workflowAction.Args[0])
				if err != nil {
					fmt.Printf("Failed to marshal schedule args for %s: %v\n", entry.ID, err)
				} else {
					// Parse the payload structure
					var payload struct {
						Metadata struct {
							Encoding string `json:"encoding"`
						} `json:"metadata"`
						Data string `json:"data"`
					}
					
					err = json.Unmarshal(jsonBytes, &payload)
					if err != nil {
						fmt.Printf("Failed to unmarshal payload for %s: %v\n", entry.ID, err)
					} else {
						// Decode the base64 data
						decodedData, err := base64.StdEncoding.DecodeString(payload.Data)
						if err != nil {
							fmt.Printf("Failed to decode base64 data for %s: %v\n", entry.ID, err)
						} else {
							// Now unmarshal the actual tracking request
							err = json.Unmarshal(decodedData, &trackingReq)
							if err != nil {
								fmt.Printf("Failed to unmarshal tracking request for %s: %v\n", entry.ID, err)
							}
						}
					}
				}
			}
		}

		scheduleInfo := ScheduleInfo{
			ScheduleID:   entry.ID,
			Sport:        trackingReq.Sport,
			League:       trackingReq.League,
			Teams:        trackingReq.Teams,
			Conferences:  trackingReq.Conferences,
			ScheduleType: trackingReq.ScheduleType,
			Paused:       desc.Schedule.State.Paused,
		}

		// Get next run time
		if len(desc.Info.NextActionTimes) > 0 {
			scheduleInfo.NextRunTime = desc.Info.NextActionTimes[0]
		}

		schedules = append(schedules, scheduleInfo)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schedules)
}

// ManageSchedule handles schedule management (delete, pause, etc.)
func (h *Handlers) ManageSchedule(w http.ResponseWriter, r *http.Request) {
	scheduleID := strings.TrimPrefix(r.URL.Path, "/api/schedules/")
	if scheduleID == "" {
		http.Error(w, "Schedule ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		// Check if Temporal client is available
		if h.temporalClient == nil {
			response := map[string]string{
				"message": "Demo mode: Schedule delete request received (Temporal server not connected)",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Delete schedule
		scheduleClient := h.temporalClient.ScheduleClient()
		handle := scheduleClient.GetHandle(context.Background(), scheduleID)
		err := handle.Delete(context.Background())
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to delete schedule: %v", err), http.StatusInternalServerError)
			return
		}
		
		response := map[string]string{
			"message": "Schedule deleted successfully",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
