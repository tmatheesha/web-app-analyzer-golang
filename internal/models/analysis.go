package models

import "time"

type AnalysisResult struct {
	URL               string         `json:"url"`
	HTMLVersion       string         `json:"html_version"`
	PageTitle         string         `json:"page_title"`
	Headings          map[string]int `json:"headings"`
	InternalLinks     int            `json:"internal_links"`
	ExternalLinks     int            `json:"external_links"`
	InaccessibleLinks int            `json:"inaccessible_links"`
	HasLoginForm      bool           `json:"has_login_form"`
	AnalysisTime      time.Duration  `json:"analysis_time"`
	Timestamp         time.Time      `json:"timestamp"`
	Error             string         `json:"error,omitempty"`
	HTTPStatusCode    int            `json:"http_status_code,omitempty"`
}

type FormInfo struct {
	Action     string `json:"action"`
	Method     string `json:"method"`
	HasLogin   bool   `json:"has_login"`
	InputCount int    `json:"input_count"`
}

// NewAnalysisResult creates a new AnalysisResult with default values
func NewAnalysisResult(url string) *AnalysisResult {
	return &AnalysisResult{
		URL:            url,
		Headings:       make(map[string]int),
		Timestamp:      time.Now(),
		HTMLVersion:    "Unknown",
		HTTPStatusCode: 200, // Default to OK
	}
}

// SetError sets the error message and HTTP status code for the analysis result
func (ar *AnalysisResult) SetError(message string, statusCode int) {
	ar.Error = message
	ar.HTTPStatusCode = statusCode
}

// AddHeading increments the count for a specific heading type
func (ar *AnalysisResult) AddHeading(level string) {
	ar.Headings[level]++
}

// IsSuccessful checks if the analysis was successful (no errors)
func (ar *AnalysisResult) IsSuccessful() bool {
	return ar.Error == ""
}
