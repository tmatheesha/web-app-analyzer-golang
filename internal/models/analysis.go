package models

import "time"

// AnalysisResult represents the result of analyzing a web page
type AnalysisResult struct {
	URL               string         `json:"url" example:"https://example.com"`
	HTMLVersion       string         `json:"html_version" example:"HTML5"`
	PageTitle         string         `json:"page_title" example:"Example Page"`
	Headings          map[string]int `json:"headings"`
	InternalLinks     int            `json:"internal_links" example:"5"`
	ExternalLinks     int            `json:"external_links" example:"2"`
	InaccessibleLinks int            `json:"inaccessible_links" example:"0"`
	HasLoginForm      bool           `json:"has_login_form" example:"true"`
	AnalysisTime      string         `json:"analysis_time" example:"1.234s"` // Changed from time.Duration to string
	Timestamp         time.Time      `json:"timestamp" example:"2023-01-01T12:00:00Z"`
	Error             string         `json:"error,omitempty" example:"Failed to fetch page"`
	HTTPStatusCode    int            `json:"http_status_code,omitempty" example:"200"`

	// New analysis fields
	Images        []ImageInfo       `json:"images"`
	MetaTags      []MetaTag         `json:"meta_tags"`
	Scripts       []ScriptInfo      `json:"scripts"`
	Stylesheets   []StylesheetInfo  `json:"stylesheets"`
	Forms         []FormInfo        `json:"forms"`
	Tables        int               `json:"tables" example:"2"`
	Lists         int               `json:"lists" example:"5"`
	Buttons       int               `json:"buttons" example:"3"`
	Inputs        int               `json:"inputs" example:"8"`
	TextContent   TextContentInfo   `json:"text_content"`
	Accessibility AccessibilityInfo `json:"accessibility"`
}

type ImageInfo struct {
	Src        string `json:"src"`
	Alt        string `json:"alt"`
	Width      string `json:"width"`
	Height     string `json:"height"`
	IsExternal bool   `json:"is_external"`
}

type MetaTag struct {
	Name     string `json:"name"`
	Content  string `json:"content"`
	Property string `json:"property"`
}

type ScriptInfo struct {
	Src        string `json:"src"`
	Type       string `json:"type"`
	IsExternal bool   `json:"is_external"`
}

type StylesheetInfo struct {
	Href       string `json:"href"`
	Media      string `json:"media"`
	IsExternal bool   `json:"is_external"`
}

type FormInfo struct {
	Action     string `json:"action"`
	Method     string `json:"method"`
	HasLogin   bool   `json:"has_login"`
	InputCount int    `json:"input_count"`
}

type TextContentInfo struct {
	WordCount      int  `json:"word_count"`
	CharCount      int  `json:"char_count"`
	Paragraphs     int  `json:"paragraphs"`
	HasMainContent bool `json:"has_main_content"`
}

type AccessibilityInfo struct {
	HasAltText      bool `json:"has_alt_text"`
	HasARIALabels   bool `json:"has_aria_labels"`
	HasSemanticHTML bool `json:"has_semantic_html"`
	HasSkipLinks    bool `json:"has_skip_links"`
}

// NewAnalysisResult creates a new AnalysisResult with default values
func NewAnalysisResult(url string) *AnalysisResult {
	return &AnalysisResult{
		URL:            url,
		Headings:       make(map[string]int),
		Timestamp:      time.Now(),
		HTMLVersion:    "Unknown",
		HTTPStatusCode: 200, // Default to OK
		Images:         make([]ImageInfo, 0),
		MetaTags:       make([]MetaTag, 0),
		Scripts:        make([]ScriptInfo, 0),
		Stylesheets:    make([]StylesheetInfo, 0),
		Forms:          make([]FormInfo, 0),
		TextContent:    TextContentInfo{},
		Accessibility:  AccessibilityInfo{},
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
