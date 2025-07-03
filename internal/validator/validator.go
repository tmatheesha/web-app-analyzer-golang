package validator

import (
	"net/url"
	"regexp"
	"strings"
)

type URLValidator struct {
	urlRegex *regexp.Regexp
}

func NewURLValidator() *URLValidator {
	pattern := `^(https?:\/\/)?([\da-z\.-]+)\.([a-z\.]{2,6})([\/\w \.-]*)*\/?$`
	return &URLValidator{
		urlRegex: regexp.MustCompile(pattern),
	}
}

// ValidateURL validates if the given string is a valid URL
func (v *URLValidator) ValidateURL(input string) (string, error) {
	input = strings.TrimSpace(input)

	if input == "" {
		return "", ErrEmptyURL
	}

	if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
		input = "https://" + input
	}

	if !v.urlRegex.MatchString(input) {
		return "", ErrInvalidURL
	}
	parsedURL, err := url.Parse(input)
	if err != nil {
		return "", ErrInvalidURL
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", ErrInvalidScheme
	}
	if parsedURL.Host == "" {
		return "", ErrMissingHost
	}
	if !v.isValidHost(parsedURL.Host) {
		return "", ErrInvalidHost
	}

	return input, nil
}

// isValidHost checks if the host part of the URL is valid
func (v *URLValidator) isValidHost(host string) bool {
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	hostRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
	return hostRegex.MatchString(host)
}

func (v *URLValidator) IsInternalLink(linkURL, baseURL string) bool {
	linkParsed, err := url.Parse(linkURL)
	if err != nil {
		return false
	}

	baseParsed, err := url.Parse(baseURL)
	if err != nil {
		return false
	}

	return linkParsed.Host == baseParsed.Host
}

var (
	ErrEmptyURL      = &ValidationError{Message: "URL cannot be empty"}
	ErrInvalidURL    = &ValidationError{Message: "Invalid URL format"}
	ErrInvalidScheme = &ValidationError{Message: "URL must start with http:// or https://"}
	ErrMissingHost   = &ValidationError{Message: "URL must contain a host"}
	ErrInvalidHost   = &ValidationError{Message: "Invalid host format"}
)

type ValidationError struct {
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return e.Message
}
