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

type testStruct struct {
	flag     int    // 0 | FDirList | FDirRedirect | FIgnoreEmptyExt
	gzip     bool   // Accept-Encoding: gzip,deflate,sdch
	length   int    // size of read Response.Body
	path     string // URL.Path
	redirect string // redirect URL.Path
}

/**
root
│   index.html
│   index.html.gz
├───empty
├───gz
│       index.html.gz
└───src
        index.html
*/
var testData = []testStruct{
	// not exists
	{0, true, 0, "/no", ""},
	{0, true, 0, "/no/", ""},
	{0, true, 0, "/no.htm", ""},
	{0, false, 0, "/no", ""},
	{0, false, 0, "/no/", ""},
	{0, false, 0, "/no.html", ""},

	{FIgnoreEmptyExt | FDirList | FDirRedirect,
		true, 0, "/no", ""},
	{FIgnoreEmptyExt | FDirList | FDirRedirect,
		true, 0, "/no/", ""},
	{FIgnoreEmptyExt | FDirList | FDirRedirect,
		true, 0, "/no.js", ""},
	{FIgnoreEmptyExt | FDirList | FDirRedirect,
		false, 0, "/no", ""},
	{FIgnoreEmptyExt | FDirList | FDirRedirect,
		false, 0, "/no/", ""},
	{FIgnoreEmptyExt | FDirList | FDirRedirect,
		false, 0, "/no.css", ""},

	// empty directory
	{0, true, 0, "/empty", ""},
	{0, true, 0, "/empty/", ""},
	{0, false, 0, "/empty", ""},
	{0, false, 0, "/empty/", ""},
	// directory list, 34 bytes
	// <pre> <a href="../">..</a> </pre>
	{FDirList, true, 34, "/empty", ""},
	{FDirList, true, 34, "/empty/", ""},
	{FDirRedirect, true, 0, "/empty?foo=bar&bar=baz", "/empty/?foo=bar&bar=baz"},
	{FIgnoreEmptyExt, true, 0, "/empty", ""},
	{FDirList | FDirRedirect, true, 34, "/empty", ""},
	{FDirList | FDirRedirect, true, 34, "/empty/", ""},
	{FIgnoreEmptyExt | FDirList | FDirRedirect, true, 0, "/empty", ""},

	// root ...
	// source and gzip precompression files
	{0, true, 6, "/", ""},
	{0, true, 6, "/index.html", "/"},
	{0, true, 40, "/index.html.gz", ""},
	{0, false, 6, "/", ""},
	{0, false, 6, "/index.html", "/"},
	{0, false, 40, "/index.html.gz", ""},

	{FIgnoreEmptyExt | FDirList | FDirRedirect,
		true, 6, "/", ""},
	{FIgnoreEmptyExt | FDirList | FDirRedirect,
		true, 6, "/index.html", "/"},
	{FIgnoreEmptyExt | FDirList | FDirRedirect,
		true, 40, "/index.html.gz", ""},
	{FIgnoreEmptyExt | FDirList | FDirRedirect,
		false, 6, "/", ""},
	{FIgnoreEmptyExt | FDirList | FDirRedirect,
		false, 6, "/index.html", "/"},
	{FIgnoreEmptyExt | FDirList | FDirRedirect,
		false, 40, "/index.html.gz", ""},

	//	gzip precompression file only
	{0, true, 0, "/gz", ""},
	{0, true, 6, "/gz/", ""},
	{0, false, 0, "/gz", ""},
	{0, false, 0, "/gz/", ""},

	// directory list, 76 bytes
	// <pre> <a href="../">..</a> <a href="index.html.gz">index.html.gz</a> </pre>
	{FDirList,
		true, 76, "/gz", ""},
	{FDirRedirect,
		true, 6, "/gz", "/gz/"},
	{FDirList | FDirRedirect,
		true, 76, "/gz", ""},
	{FIgnoreEmptyExt,
		true, 0, "/gz", ""},
	{FIgnoreEmptyExt | FDirList | FDirRedirect,
		true, 0, "/gz", ""},
	{FIgnoreEmptyExt,
		true, 6, "/gz/", ""},
	{FIgnoreEmptyExt | FDirList | FDirRedirect,
		true, 6, "/gz/", ""},

	{FDirList,
		false, 76, "/gz", ""},
	{FDirRedirect,
		false, 0, "/gz", "/gz/"},
	{FDirList | FDirRedirect,
		false, 76, "/gz", ""},
	{FIgnoreEmptyExt,
		false, 0, "/gz", ""},
	{FIgnoreEmptyExt | FDirList,
		false, 0, "/gz", ""},
	{FIgnoreEmptyExt | FDirList,
		false, 76, "/gz/", ""},

	//	source file only
	{0, true, 0, "/src", ""},
	{0, true, 6, "/src/", ""},
	{0, false, 0, "/src", ""},
	{0, false, 6, "/src/", ""},

	// directory list, 70 bytes
	// <pre> <a href="../">..</a> <a href="index.html">index.html</a> </pre>
	{FDirList,
		true, 70, "/src", ""},
	{FDirRedirect,
		true, 6, "/src", "/src/"},
	{FDirList | FDirRedirect,
		true, 70, "/src", ""},
	{FIgnoreEmptyExt,
		true, 0, "/src", ""},
	{FIgnoreEmptyExt | FDirList | FDirRedirect,
		true, 0, "/src", ""},
	{FIgnoreEmptyExt,
		true, 6, "/src/", ""},
	{FIgnoreEmptyExt | FDirList | FDirRedirect,
		true, 6, "/src/", ""},

	{FDirList,
		false, 70, "/src", ""},
	{FDirRedirect,
		false, 6, "/src", "/src/"},
	{FDirList | FDirRedirect,
		false, 70, "/src", ""},
	{FIgnoreEmptyExt,
		false, 0, "/src", ""},
	{FIgnoreEmptyExt | FDirList,
		false, 0, "/src", ""},
	{FIgnoreEmptyExt | FDirList,
		false, 6, "/src/", ""},
}

func TestStatic(t *testing.T) {
	dir, _ := os.Getwd()
	dir = dir + "/testdata"
	var handler func(res http.ResponseWriter, req *http.Request, dir http.Dir)
	var gzip bool
	var c *http.Client
	var lastreq, referer *http.Request
	var via []*http.Request

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if gzip {
			req.Header.Set("Accept-Encoding", "gzip,deflate,sdch")
		} else {
			req.Header.Del("Accept-Encoding")
		}
		lastreq = req
		handler(w, req, http.Dir(dir))
	}))
	defer ts.Close()

	c = &http.Client{CheckRedirect: func(req *http.Request, vias []*http.Request) error {
		via = vias
		referer = req
		return nil
	}}

	for _, tt := range testData {
		gzip = tt.gzip
		via = []*http.Request{}
		referer = nil
		handler = Handler(tt.flag)

		req, _ := http.NewRequest("GET", ts.URL+tt.path, nil)
		res, err := c.Do(req)

		if err != nil {
			t.Fatal(err, tt)
		}
		res.Request = lastreq
		if tt.redirect == "" {
			// 200
			if len(via) != 0 || referer != nil {
				show(res)
				t.Fatal(len(via), referer == nil, tt)
			}
		} else {
			// 301
			if len(via) != 1 || referer == nil {
				show(res)
				t.Fatal(len(via), referer == nil, tt)
			}
			if lastreq.RequestURI != tt.redirect {
				show(res)
				t.Fatal(tt)
			}
		}

		n, err := io.Copy(ioutil.Discard, res.Body)
		if err != nil || int(n) != tt.length ||
			tt.length == 40 && res.Header.Get("Content-Encoding") != "" {
			show(res)
			t.Fatal(err, n, tt)
		}
	}
}

func show(res *http.Response) {
	fmt.Println("----------Request.Header----------")
	fmt.Println("URI", res.Request.RequestURI)
	for k, v := range res.Request.Header {
		fmt.Println(k, v)
	}
	fmt.Println("\n--------Response.Header---------")
	fmt.Println(res.Status)
	fmt.Println("ContentLength", res.ContentLength)
	for k, v := range res.Header {
		fmt.Println(k, v)
	}
}
