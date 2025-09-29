package utils

import (
	"testing"
	"time"
)

func TestParseTime(t *testing.T) {
	tests := []struct {
		input    string
		expected bool // true if should parse successfully
	}{
		{"2023-12-25T15:30:45Z", true},
		{"2023-12-25T15:30:45.123Z", true},
		{"2023-12-25T15:30:45+08:00", true},
		{"2023-12-25T15:30:45.123+08:00", true},
		{"2023-12-25 15:30:45", true},
		{"2023-12-25", true},
		{"invalid-time", false},
		{"", false},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			_, err := ParseTime(test.input)
			if test.expected && err != nil {
				t.Errorf("Expected to parse '%s' but got error: %v", test.input, err)
			}
			if !test.expected && err == nil {
				t.Errorf("Expected to fail parsing '%s' but it succeeded", test.input)
			}
		})
	}
}

func TestParseTimeRange(t *testing.T) {
	tests := []struct {
		start    string
		end      string
		expected bool
	}{
		{"2023-12-25T00:00:00Z", "2023-12-25T23:59:59Z", true},
		{"2023-12-25", "2023-12-26", true},
		{"2023-12-26T00:00:00Z", "2023-12-25T00:00:00Z", false}, // start > end
		{"invalid", "2023-12-25T00:00:00Z", false},
		{"", "", false},
	}

	for _, test := range tests {
		t.Run(test.start+"_to_"+test.end, func(t *testing.T) {
			timeRange, err := ParseTimeRange(test.start, test.end)
			if test.expected && err != nil {
				t.Errorf("Expected to parse range '%s' to '%s' but got error: %v", test.start, test.end, err)
			}
			if !test.expected && err == nil {
				t.Errorf("Expected to fail parsing range '%s' to '%s' but it succeeded", test.start, test.end)
			}
			if test.expected && err == nil && !timeRange.IsValid() {
				t.Errorf("Expected valid time range but got invalid one")
			}
		})
	}
}

func TestCreateTimeRangeFromHours(t *testing.T) {
	timeRange := CreateTimeRangeFromHours(24)
	
	if !timeRange.IsValid() {
		t.Error("Expected valid time range")
	}

	duration := timeRange.Duration()
	expectedDuration := 24 * time.Hour
	
	if duration != expectedDuration {
		t.Errorf("Expected duration %v but got %v", expectedDuration, duration)
	}
}

func TestCreateYesterdayRange(t *testing.T) {
	timeRange := CreateYesterdayRange()
	
	if !timeRange.IsValid() {
		t.Error("Expected valid time range")
	}

	duration := timeRange.Duration()
	expectedDuration := 24 * time.Hour
	
	if duration != expectedDuration {
		t.Errorf("Expected duration %v but got %v", expectedDuration, duration)
	}
}

func TestIsInRange(t *testing.T) {
	start := time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC)
	end := time.Date(2023, 12, 25, 23, 59, 59, 0, time.UTC)
	
	tests := []struct {
		t        time.Time
		expected bool
	}{
		{time.Date(2023, 12, 25, 12, 0, 0, 0, time.UTC), true},
		{time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC), true}, // boundary
		{time.Date(2023, 12, 25, 23, 59, 59, 0, time.UTC), true}, // boundary
		{time.Date(2023, 12, 24, 23, 59, 59, 0, time.UTC), false}, // before
		{time.Date(2023, 12, 26, 0, 0, 1, 0, time.UTC), false}, // after
	}

	for _, test := range tests {
		result := IsInRange(test.t, start, end)
		if result != test.expected {
			t.Errorf("IsInRange(%v, %v, %v) = %v, expected %v", 
				test.t, start, end, result, test.expected)
		}
	}
}

func TestParseTimeOrZero(t *testing.T) {
	// Valid time
	validResult := ParseTimeOrZero("2023-12-25T15:30:45Z")
	if validResult.IsZero() {
		t.Error("Expected non-zero time for valid input")
	}

	// Invalid time
	invalidResult := ParseTimeOrZero("invalid-time")
	if !invalidResult.IsZero() {
		t.Error("Expected zero time for invalid input")
	}
}

func TestParseTimeOrNow(t *testing.T) {
	before := time.Now()
	
	// Valid time - should not be close to now
	validResult := ParseTimeOrNow("2023-12-25T15:30:45Z")
	if validResult.After(before.Add(-time.Second)) && validResult.Before(time.Now().Add(time.Second)) {
		t.Error("Valid time should not be close to current time")
	}

	// Invalid time - should be close to now
	invalidResult := ParseTimeOrNow("invalid-time")
	if invalidResult.Before(before.Add(-time.Second)) || invalidResult.After(time.Now().Add(time.Second)) {
		t.Error("Invalid time should return current time")
	}
}