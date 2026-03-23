package web

import "net/http"

type RequestDoer interface {
	Do(*http.Request) (*http.Response, error)
}
