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
	assert.Equal(t, printMemSize(100), "100.00 b")
	assert.Equal(t, printMemSize(1024), "1.00 kb")
	assert.Equal(t, printMemSize(1024*1024), "1.00 mb")
	assert.Equal(t, printMemSize(1024*1024+1), "1.00 mb")
	assert.Equal(t, printMemSize(1024*1024*1024), "1.00 gb")
	assert.Equal(t, printMemSize(1024*1024*1024+100), "1.00 gb")
	assert.Equal(t, printMemSize(1024*1024*1024*1024+1), "1024.00 gb")
	assert.Equal(t, printMemSize(1025*1024*1024*1024), "1025.00 gb")
}
