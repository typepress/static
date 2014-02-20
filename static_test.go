/*
  mk _test
  mk _test/dir
  mk _test/gz
  cd _test
  creat index.html
  index.html 6 bytes with static
  gzip index.html index.html.gz
  mv index.html dir/index.html
  cp index.html.gz gz/index.html.gz
*/

package static

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestStatic(t *testing.T) {
	tests := []struct {
		path     string
		redirect string
		code     int
		length   int
	}{
		{"/", "", 200, 0},
		{"/_test?code=301", "/_test/?code=301", 301, 40},
		{"/_test/dir", "/_test/dir/", 301, 6},
		{"/_test/dir/index.html", "/_test/dir/", 301, 6},
		{"/_test/gz/index.html", "/_test/gz/", 301, 40},
		{"/_test/gz/", "", 200, 40},
	}

	dir, _ := os.Getwd()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Handler(w, r, http.Dir(dir))
	}))
	defer ts.Close()

	var checkErr error
	var lastVia []*http.Request
	var c *http.Client
	var lastreq *http.Request
	c = &http.Client{CheckRedirect: func(r *http.Request, via []*http.Request) error {
		r.Header.Set("Accept-Encoding", "gzip")
		lastVia = via
		lastreq = r
		return checkErr
	}}
	for _, tt := range tests {
		lastVia = []*http.Request{}
		greq, _ := http.NewRequest("GET", ts.URL+tt.path, nil)
		greq.Header.Set("Accept-Encoding", "gzip")
		res, err := c.Do(greq)
		if err != nil || res.StatusCode != 200 {
			t.Fatal(err, res.Status, tt.path)
		}
		switch tt.code {
		case 200:
			if len(lastVia) != 0 {
				t.Fatal(tt, lastVia[0].URL.RequestURI())
			}
		case 301:
			if len(lastVia) != 1 || lastreq.URL.RequestURI() != tt.redirect {
				t.Fatal(tt, lastreq.URL.RequestURI())
			}
		default:
			t.Fatal(res.Status, tt)
		}

		n, err := io.Copy(ioutil.Discard, res.Body)
		if err != nil || int(n) != tt.length ||
			tt.length == 40 && res.Header.Get("Content-Encoding") != "gzip" {
			show(res)
			t.Fatal(err, n, tt)
		}
	}
}

func show(res *http.Response) {
	fmt.Println(res.Request.Header.Get("Accept-Encoding"))
	fmt.Println(res.Status)
	for k, v := range res.Header {
		fmt.Println(k, v)
	}
	if res.Status != "" {
		return
	}
	for k, v := range res.Request.Header {
		fmt.Println(k, v)
	}
}
