package sports

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestESPNTime_UnmarshalJSON_Detailed(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    time.Time
		expectError bool
	}{
		{
			name:        "RFC3339 format with Z timezone",
			input:       `"2023-09-10T15:30:00Z"`,
			expected:    time.Date(2023, 9, 10, 15, 30, 0, 0, time.UTC),
			expectError: false,
		},
		{
			name:        "RFC3339 format with positive offset",
			input:       `"2023-09-10T15:30:00+05:00"`,
			expected:    time.Date(2023, 9, 10, 15, 30, 0, 0, time.FixedZone("", 5*3600)),
			expectError: false,
		},
		{
			name:        "RFC3339 format with negative offset",
			input:       `"2023-09-10T15:30:00-04:00"`,
			expected:    time.Date(2023, 9, 10, 15, 30, 0, 0, time.FixedZone("", -4*3600)),
			expectError: false,
		},
		{
			name:        "Short format without seconds with Z",
			input:       `"2023-09-10T15:30Z"`,
			expected:    time.Date(2023, 9, 10, 15, 30, 0, 0, time.UTC),
			expectError: false,
		},
		{
			name:        "Short format without seconds with offset",
			input:       `"2023-09-10T15:30-04:00"`,
			expected:    time.Date(2023, 9, 10, 15, 30, 0, 0, time.FixedZone("", -4*3600)),
			expectError: false,
		},
		{
			name:        "Empty string",
			input:       `""`,
			expected:    time.Time{},
			expectError: false,
		},
		{
			name:        "Null value",
			input:       `null`,
			expected:    time.Time{},
			expectError: false,
		},
		{
			name:        "Invalid date format",
			input:       `"invalid-date"`,
			expected:    time.Time{},
			expectError: true,
		},
		{
			name:        "Partial date",
			input:       `"2023-09-10"`,
			expected:    time.Time{},
			expectError: true,
		},
		{
			name:        "Time only",
			input:       `"15:30:00"`,
			expected:    time.Time{},
			expectError: true,
		},
		{
			name:        "Invalid JSON",
			input:       `"2023-09-10T15:30:00Z`,
			expected:    time.Time{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var espnTime ESPNTime
			err := json.Unmarshal([]byte(tt.input), &espnTime)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			
			// For empty string and null, we expect zero time
			if tt.input == `""` || tt.input == `null` {
				assert.True(t, espnTime.Time.IsZero())
				return
			}

			// For valid dates, check if times are equal
			assert.True(t, tt.expected.Equal(espnTime.Time), 
				"Expected %v, got %v", tt.expected, espnTime.Time)
		})
	}
}

func TestESPNTime_UnmarshalJSON_InStruct(t *testing.T) {
	// Test ESPNTime when used within a struct (like in ESPN API responses)
	type TestEvent struct {
		ID   string   `json:"id"`
		Date ESPNTime `json:"date"`
		Name string   `json:"name"`
	}

	tests := []struct {
		name        string
		jsonInput   string
		expectedID  string
		expectedName string
		expectedTime time.Time
		expectError bool
	}{
		{
			name: "valid event with RFC3339 date",
			jsonInput: `{
				"id": "401520281",
				"date": "2023-09-10T15:30:00Z",
				"name": "Michigan vs Washington"
			}`,
			expectedID:   "401520281",
			expectedName: "Michigan vs Washington",
			expectedTime: time.Date(2023, 9, 10, 15, 30, 0, 0, time.UTC),
			expectError:  false,
		},
		{
			name: "valid event with short format date",
			jsonInput: `{
				"id": "401520282",
				"date": "2023-09-10T15:30Z",
				"name": "Ohio State vs Penn State"
			}`,
			expectedID:   "401520282",
			expectedName: "Ohio State vs Penn State",
			expectedTime: time.Date(2023, 9, 10, 15, 30, 0, 0, time.UTC),
			expectError:  false,
		},
		{
			name: "event with null date",
			jsonInput: `{
				"id": "401520283",
				"date": null,
				"name": "TBD vs TBD"
			}`,
			expectedID:   "401520283",
			expectedName: "TBD vs TBD",
			expectedTime: time.Time{},
			expectError:  false,
		},
		{
			name: "event with empty date",
			jsonInput: `{
				"id": "401520284",
				"date": "",
				"name": "Future Game"
			}`,
			expectedID:   "401520284",
			expectedName: "Future Game",
			expectedTime: time.Time{},
			expectError:  false,
		},
		{
			name: "event with invalid date",
			jsonInput: `{
				"id": "401520285",
				"date": "invalid-date",
				"name": "Error Game"
			}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event TestEvent
			err := json.Unmarshal([]byte(tt.jsonInput), &event)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedID, event.ID)
			assert.Equal(t, tt.expectedName, event.Name)
			
			if tt.expectedTime.IsZero() {
				assert.True(t, event.Date.Time.IsZero())
			} else {
				assert.True(t, tt.expectedTime.Equal(event.Date.Time))
			}
		})
	}
}

func TestESPNTime_MarshalJSON(t *testing.T) {
	// Test that ESPNTime can be marshaled back to JSON
	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "UTC time",
			time:     time.Date(2023, 9, 10, 15, 30, 0, 0, time.UTC),
			expected: `"2023-09-10T15:30:00Z"`,
		},
		{
			name:     "time with offset",
			time:     time.Date(2023, 9, 10, 15, 30, 0, 0, time.FixedZone("EST", -5*3600)),
			expected: `"2023-09-10T15:30:00-05:00"`,
		},
		{
			name:     "zero time",
			time:     time.Time{},
			expected: `"0001-01-01T00:00:00Z"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			espnTime := ESPNTime{Time: tt.time}
			jsonBytes, err := json.Marshal(espnTime)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(jsonBytes))
		})
	}
}

func TestESPNTime_RoundTrip(t *testing.T) {
	// Test that unmarshaling and then marshaling produces consistent results
	originalTimes := []time.Time{
		time.Date(2023, 9, 10, 15, 30, 0, 0, time.UTC),
		time.Date(2023, 9, 10, 15, 30, 0, 0, time.FixedZone("EST", -5*3600)),
		time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC),
	}

	for i, originalTime := range originalTimes {
		t.Run(fmt.Sprintf("round_trip_%d", i), func(t *testing.T) {
			// Marshal original time
			espnTime := ESPNTime{Time: originalTime}
			jsonBytes, err := json.Marshal(espnTime)
			require.NoError(t, err)

			// Unmarshal back
			var unmarshaledTime ESPNTime
			err = json.Unmarshal(jsonBytes, &unmarshaledTime)
			require.NoError(t, err)

			// Should be equal
			assert.True(t, originalTime.Equal(unmarshaledTime.Time))
		})
	}
}

func TestESPNTime_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expectError bool
	}{
		{
			name:        "malformed JSON",
			input:       []byte(`"2023-09-10T15:30:00Z`),
			expectError: true,
		},
		{
			name:        "number instead of string",
			input:       []byte(`1694358600`),
			expectError: true,
		},
		{
			name:        "boolean instead of string",
			input:       []byte(`true`),
			expectError: true,
		},
		{
			name:        "object instead of string",
			input:       []byte(`{"year": 2023}`),
			expectError: true,
		},
		{
			name:        "array instead of string",
			input:       []byte(`["2023", "09", "10"]`),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var espnTime ESPNTime
			err := espnTime.UnmarshalJSON(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestESPNTime_TimeZoneHandling(t *testing.T) {
	// Test various timezone formats that ESPN might use
	tests := []struct {
		name     string
		input    string
		expected time.Time
	}{
		{
			name:     "UTC with Z",
			input:    `"2023-09-10T15:30:00Z"`,
			expected: time.Date(2023, 9, 10, 15, 30, 0, 0, time.UTC),
		},
		{
			name:     "UTC with +00:00",
			input:    `"2023-09-10T15:30:00+00:00"`,
			expected: time.Date(2023, 9, 10, 15, 30, 0, 0, time.UTC),
		},
		{
			name:     "Eastern Time",
			input:    `"2023-09-10T15:30:00-04:00"`,
			expected: time.Date(2023, 9, 10, 15, 30, 0, 0, time.FixedZone("", -4*3600)),
		},
		{
			name:     "Pacific Time",
			input:    `"2023-09-10T15:30:00-07:00"`,
			expected: time.Date(2023, 9, 10, 15, 30, 0, 0, time.FixedZone("", -7*3600)),
		},
		{
			name:     "Central European Time",
			input:    `"2023-09-10T15:30:00+01:00"`,
			expected: time.Date(2023, 9, 10, 15, 30, 0, 0, time.FixedZone("", 1*3600)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var espnTime ESPNTime
			err := json.Unmarshal([]byte(tt.input), &espnTime)
			require.NoError(t, err)
			assert.True(t, tt.expected.Equal(espnTime.Time))
		})
	}
}

// Benchmark tests
func BenchmarkESPNTime_UnmarshalJSON(b *testing.B) {
	input := []byte(`"2023-09-10T15:30:00Z"`)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var espnTime ESPNTime
		espnTime.UnmarshalJSON(input)
	}
}

func BenchmarkESPNTime_UnmarshalJSON_ShortFormat(b *testing.B) {
	input := []byte(`"2023-09-10T15:30Z"`)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var espnTime ESPNTime
		espnTime.UnmarshalJSON(input)
	}
}

func BenchmarkESPNTime_MarshalJSON(b *testing.B) {
	espnTime := ESPNTime{Time: time.Date(2023, 9, 10, 15, 30, 0, 0, time.UTC)}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(espnTime)
	}
}
