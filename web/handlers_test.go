package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	sports "temporal-sports-tracker"
)

func TestGetSports(t *testing.T) {
	handlers := NewHandlers(nil)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "successful GET request",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedCount:  5, // baseball, basketball, football, hockey, soccer
		},
		{
			name:           "invalid POST request",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/sports", nil)
			w := httptest.NewRecorder()

			handlers.GetSports(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var sports []Sport
				err := json.Unmarshal(w.Body.Bytes(), &sports)
				assert.NoError(t, err)
				assert.Len(t, sports, tt.expectedCount)
				
				// Verify specific sports are included
				sportNames := make(map[string]bool)
				for _, sport := range sports {
					sportNames[sport.Name] = true
				}
				assert.True(t, sportNames["Football"])
				assert.True(t, sportNames["Basketball"])
			}
		})
	}
}

func TestGetLeagues(t *testing.T) {
	handlers := NewHandlers(nil)

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "football leagues",
			method:         http.MethodGet,
			path:           "/api/leagues/football",
			expectedStatus: http.StatusOK,
			expectedCount:  2, // NFL, College Football
		},
		{
			name:           "basketball leagues",
			method:         http.MethodGet,
			path:           "/api/leagues/basketball",
			expectedStatus: http.StatusOK,
			expectedCount:  3, // NBA, Men's College, Women's College
		},
		{
			name:           "unsupported sport",
			method:         http.MethodGet,
			path:           "/api/leagues/tennis",
			expectedStatus: http.StatusBadRequest,
			expectedCount:  0,
		},
		{
			name:           "missing sport",
			method:         http.MethodGet,
			path:           "/api/leagues/",
			expectedStatus: http.StatusBadRequest,
			expectedCount:  0,
		},
		{
			name:           "invalid method",
			method:         http.MethodPost,
			path:           "/api/leagues/football",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handlers.GetLeagues(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var leagues []League
				err := json.Unmarshal(w.Body.Bytes(), &leagues)
				assert.NoError(t, err)
				assert.Len(t, leagues, tt.expectedCount)
			}
		})
	}
}

func TestGetConferences(t *testing.T) {
	handlers := NewHandlers(nil)

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		minCount       int
	}{
		{
			name:           "college football conferences",
			method:         http.MethodGet,
			path:           "/api/conferences/football/college-football",
			expectedStatus: http.StatusOK,
			minCount:       5, // At least Big Ten, SEC, etc.
		},
		{
			name:           "college basketball conferences",
			method:         http.MethodGet,
			path:           "/api/conferences/basketball/mens-college-basketball",
			expectedStatus: http.StatusOK,
			minCount:       5,
		},
		{
			name:           "NFL conferences (should be empty)",
			method:         http.MethodGet,
			path:           "/api/conferences/football/nfl",
			expectedStatus: http.StatusOK,
			minCount:       0,
		},
		{
			name:           "missing parameters",
			method:         http.MethodGet,
			path:           "/api/conferences/football",
			expectedStatus: http.StatusBadRequest,
			minCount:       0,
		},
		{
			name:           "invalid method",
			method:         http.MethodPost,
			path:           "/api/conferences/football/college-football",
			expectedStatus: http.StatusMethodNotAllowed,
			minCount:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handlers.GetConferences(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var conferences []Conference
				err := json.Unmarshal(w.Body.Bytes(), &conferences)
				assert.NoError(t, err)
				assert.GreaterOrEqual(t, len(conferences), tt.minCount)
				
				if len(conferences) > 0 {
					// Verify conference structure
					conf := conferences[0]
					assert.NotEmpty(t, conf.ID)
					assert.NotEmpty(t, conf.Name)
				}
			}
		})
	}
}

func TestStartTracking_DemoMode(t *testing.T) {
	handlers := NewHandlers(nil) // Demo mode (no Temporal client)

	tests := []struct {
		name           string
		method         string
		body           interface{}
		expectedStatus int
	}{
		{
			name:   "successful tracking start in demo mode",
			method: http.MethodPost,
			body: sports.TrackingRequest{
				Sport:       "football",
				League:      "college-football",
				Teams:       []string{"130", "264"},
				Conferences: []string{"5"},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid JSON body",
			method:         http.MethodPost,
			body:           "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid method",
			method:         http.MethodGet,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.body != nil {
				if str, ok := tt.body.(string); ok {
					body = []byte(str)
				} else {
					body, _ = json.Marshal(tt.body)
				}
			}

			req := httptest.NewRequest(tt.method, "/api/start-tracking", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			handlers.StartTracking(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response, "workflowId")
				assert.Contains(t, response, "runId")
				assert.Contains(t, response, "message")
				assert.Contains(t, response["message"], "Demo mode")
			}
		})
	}
}

func TestGetWorkflows_DemoMode(t *testing.T) {
	handlers := NewHandlers(nil) // Demo mode

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "demo mode returns empty list",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedCount:  0,
		},
		{
			name:           "invalid method",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/workflows", nil)
			w := httptest.NewRecorder()

			handlers.GetWorkflows(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var workflows []GameWorkflow
				err := json.Unmarshal(w.Body.Bytes(), &workflows)
				assert.NoError(t, err)
				assert.Len(t, workflows, tt.expectedCount)
			}
		})
	}
}

func TestManageWorkflow_DemoMode(t *testing.T) {
	handlers := NewHandlers(nil) // Demo mode

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "demo mode cancel",
			method:         http.MethodDelete,
			path:           "/api/workflows/test-workflow-123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing workflow ID",
			method:         http.MethodDelete,
			path:           "/api/workflows/",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid method",
			method:         http.MethodGet,
			path:           "/api/workflows/test-workflow-123",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handlers.ManageWorkflow(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response, "message")
				assert.Contains(t, response["message"], "Demo mode")
			}
		})
	}
}

func TestGetTeams(t *testing.T) {
	handlers := NewHandlers(nil)

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "valid sport and league",
			method:         http.MethodGet,
			path:           "/api/teams/football/college-football",
			expectedStatus: http.StatusOK, // Will make actual HTTP call to ESPN
		},
		{
			name:           "missing parameters",
			method:         http.MethodGet,
			path:           "/api/teams/football",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid method",
			method:         http.MethodPost,
			path:           "/api/teams/football/college-football",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handlers.GetTeams(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				// Note: This will make an actual HTTP call to ESPN API
				// In a real test environment, you might want to mock this
				var teams []sports.Team
				err := json.Unmarshal(w.Body.Bytes(), &teams)
				if err == nil {
					// If successful, verify team structure
					if len(teams) > 0 {
						team := teams[0]
						assert.NotEmpty(t, team.ID)
						assert.NotEmpty(t, team.DisplayName)
					}
				}
				// Don't assert on the actual response since it depends on external API
			}
		})
	}
}

// Integration test for handlers
func TestHandlersIntegration(t *testing.T) {
	handlers := NewHandlers(nil) // Demo mode

	// Test the full flow: sports -> leagues -> conferences -> start tracking
	
	// 1. Get sports
	req := httptest.NewRequest(http.MethodGet, "/api/sports", nil)
	w := httptest.NewRecorder()
	handlers.GetSports(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 2. Get leagues for football
	req = httptest.NewRequest(http.MethodGet, "/api/leagues/football", nil)
	w = httptest.NewRecorder()
	handlers.GetLeagues(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 3. Get conferences for college football
	req = httptest.NewRequest(http.MethodGet, "/api/conferences/football/college-football", nil)
	w = httptest.NewRecorder()
	handlers.GetConferences(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 4. Start tracking
	trackingReq := sports.TrackingRequest{
		Sport:       "football",
		League:      "college-football",
		Conferences: []string{"5"},
	}
	body, _ := json.Marshal(trackingReq)
	req = httptest.NewRequest(http.MethodPost, "/api/start-tracking", bytes.NewBuffer(body))
	w = httptest.NewRecorder()
	handlers.StartTracking(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 5. Get workflows (should be empty in demo mode)
	req = httptest.NewRequest(http.MethodGet, "/api/workflows", nil)
	w = httptest.NewRecorder()
	handlers.GetWorkflows(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// Benchmark tests
func BenchmarkGetSports(b *testing.B) {
	handlers := NewHandlers(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/sports", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handlers.GetSports(w, req)
	}
}

func BenchmarkStartTracking(b *testing.B) {
	handlers := NewHandlers(nil)
	trackingReq := sports.TrackingRequest{
		Sport:       "football",
		League:      "college-football",
		Conferences: []string{"5"},
	}
	body, _ := json.Marshal(trackingReq)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/start-tracking", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		handlers.StartTracking(w, req)
	}
}

func BenchmarkGetLeagues(b *testing.B) {
	handlers := NewHandlers(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/leagues/football", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handlers.GetLeagues(w, req)
	}
}
