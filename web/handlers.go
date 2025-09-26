package web

import (
	"context"
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
			{ID: "mls", Name: "MLS", Path: "mls"},
		}
	default:
		http.Error(w, "Unsupported sport", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(leagues)
}

// GetTeams fetches teams for a specific sport/league from ESPN API
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

	// Create scheduling workflow ID with timestamp
	workflowID := fmt.Sprintf("sports-%s", time.Now().Format("20060102-150405"))

	TaskQueueName := os.Getenv("TASK_QUEUE")
	if TaskQueueName == "" {
		http.Error(w, "TASK_QUEUE environment variable is not set", http.StatusInternalServerError)
		return
	}

	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: TaskQueueName,
	}
	// Start the CollectGamesWorkflow

	//TODO collapse TrackingRequest.Teams and TrackingRequest.Conferences into a single []string of TeamsToTrack
	//TODO change the CollectGamesWorkflow to accept TeamsToTrack as Teams[] only
	
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
