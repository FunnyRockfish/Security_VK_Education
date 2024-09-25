package domain

import "net/http"

type Request struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	GetParams  map[string][]string
	Headers    map[string][]string
	Cookies    []*http.Cookie
	PostParams map[string][]string
}

type KeyValueItem struct {
	Key   string
	Value string
}
