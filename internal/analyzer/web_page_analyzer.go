package analyzer

import "net/http"

type WebPageAnalyzer struct {
	client *http.Client
}
