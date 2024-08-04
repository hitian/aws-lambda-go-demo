package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndex(t *testing.T) {
	router := routerEngine()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "Hello World!", w.Body.String())
}

func TestAPI(t *testing.T) {
	router := routerEngine()

	apis := []string{
		"/version",
		"/ping",
		"/ip",
		"/ua",
		"/headers",
		"/date",
		"/timestamp",
		"/check_status",
		"/dns/github.com",
		"/generate_204",
		"/proto",
	}

	for _, uri := range apis {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", uri, nil)
		router.ServeHTTP(w, req)

		if uri == "/generate_204" {
			assert.Equal(t, 204, w.Code)
		} else {
			assert.Equal(t, 200, w.Code)
		}
		fmt.Println("test ", uri, " ok.")
	}

}

func TestPrintMemSize(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{100, "100.00 B"},
		{1024 + 500, "1.49 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024*1024 + 1, "1.00 MB"},
		{(1024 + 100) * 1024 * 1024, "1.10 GB"},
		{(1024+2)*1024*1024 + 100, "1.00 GB"},
		{1024*1024*1024*1024 + 1, "1.00 TB"},
		{1025 * 1024 * 1024 * 1024, "1.00 TB"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("printMemSize(%d)", tt.input), func(t *testing.T) {
			assert.Equal(t, tt.expected, printMemSize(tt.input))
		})
	}
}
