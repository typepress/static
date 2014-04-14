// static file Handler, support gzip precompression.
// 已经设定预压缩类型: .css, .htm, .html, .js
package static

import (
	"fmt"
	"html"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var gzTypes = map[string]string{
	".css":  "text/css; charset=utf-8",
	".htm":  "text/html; charset=utf-8",
	".html": "text/html; charset=utf-8",
	".js":   "application/x-javascript; charset=utf-8",
}

const (
	FIgnoreEmptyExt = 1 << iota
	FDirList
	FDirRedirect
)

const (
	lGzip = iota + 1
)

func init() {
	mime.AddExtensionType(".gz", "application/gzip")
}

// GzipPrecompressionExt not safe for concurrent use.

/**
GzipPrecompressionExt 设定为扩展名为 ext 的文件提供预压缩输出支持.
非并发安全, 适合初始化阶段调用.
举例:
	mime.AddExtensionType(".pdf", "application/pdf")
	mime.AddExtensionType(".xml", "text/xml; charset=utf-8")
	mime.AddExtensionType(".atom", "application/atom+xml; charset=utf-8")
	mime.AddExtensionType(".rss", "application/rss+xml; charset=utf-8")
	GzipPrecompressionExt(".pdf", ".xml", ".atom", ".rss")
*/
func GzipPrecompressionExt(exts ...string) error {
	for _, ext := range exts {
		if ext == ".gz" {
			continue
		}
		mimeType := mime.TypeByExtension(ext)
		if mimeType == "" {
			return fmt.Errorf(`static: mime.TypeByExtension(%#v) returns empty.`, ext)
		}
		gzTypes[ext] = mimeType
	}
	return nil
}

/**
Handler for static file, support gzip precompression.
Deny dot file, prefix "_" and invalid URL.Path.
Ignore dir of arguments is empty, req.Method not equal "GET" or "HEAD".
Redirect ".../index.html" to ".../".
Use contents of index.html for directory, if present.
Try to send gzip precompression file.
Try to send other static files.
If send fail, do nothing.

flags order:
	FIgnoreEmptyExt   do nothing for ".../foobar".
	FDirList          automatic directory listings.
	FDirRedirect      ".../directory" to ".../directory/".
*/

/**
Handler 对静态文件进行发送, 支持 Gzip 预压缩文件.
拒绝点文件, "_" 开头文件和非法 URL.Path.
忽略 dir =="" , req.Method 不是 "GET" 或 "HEAD".
如果 URL.Path 以 "/index.html" 结尾, 301 重定向到 "./".
如果 URL.Path 以 "/" 结尾, 当作 "./index.html" 处理.
如果 URL.Path 已设定预压缩类型, 尝试发送压缩文件或原文件.
如果 URL.Path 是文件尝试发送.
尝试失败不做处理.
flag: 取值和优先级:
	FIgnoreEmptyExt
		如果 URL.Path 没有扩展名, 忽略.
	FDirList
		允许目录列表
	FDirRedirect
		如果 URL.Path 是目录并且非"/"结尾, 301 重定向到 "./".
*/
func Handler(flag int) func(res http.ResponseWriter, req *http.Request, dir http.Dir) {
	return func(res http.ResponseWriter, req *http.Request, dir http.Dir) {
		root := string(dir)
		if root == "" || req.Method != "GET" && req.Method != "HEAD" {
			return
		}

		filename := req.URL.Path
		if len(filename) >= 11 && filename[len(filename)-11:] == "/index.html" {
			localRedirect(res, req, "./")
			return
		}

		var (
			ext, ctype, basename string
			f                    http.File
			fi                   os.FileInfo
			err                  error
			indexed              bool
		)
		// check filename
		path := filename

		if path[len(path)-1] == '/' {
			// index
			ext = ".html"
			ctype = gzTypes[ext]
			indexed = true
			basename = "index.html"
			filename += "index.html"

		} else {
			basename = filepath.Base(filename)
			ext = filepath.Ext(basename)

			// ignore dot file, prefix "_"
			if basename[0] == '.' || basename[0] == '_' {
				res.WriteHeader(http.StatusForbidden)
				return
			}

			// ignore empty file extension
			if ext == "" {
				if flag&FIgnoreEmptyExt != 0 {
					return
				}
			}

			if ext == ".gz" {
				/*
					filename = filename[:len(filename)-3]
					basename = basename[:len(basename)-3]
					ctype = gzTypes[filepath.Ext(basename)]
				*/
			} else if ext != "" {
				ctype = gzTypes[ext]
			}
		}

		filename = pathToDir(filename)
		if filename == "" {
			res.WriteHeader(http.StatusForbidden)
			return
		}

		var encoding int

		if ext == "" {
			f, err = os.Open(filepath.Join(root, filename))

		} else {

			if ext == ".gz" {
				/*
					filename += ".gz"
					basename += ".gz"
					ctype = ""
				*/
			} else if strings.Index(req.Header.Get("Accept-Encoding"), "gzip") >= 0 {
				// Accept-Encoding:gzip,deflate,sdch
				// try Gzip Precompression
				f, err = os.Open(filepath.Join(root, filename+".gz"))
				encoding = lGzip
			}

			if encoding == 0 || err != nil {
				encoding = 0
				f, err = os.Open(filepath.Join(root, filename))
			}
		}

		// for directory listings
		if err != nil && indexed && flag&FDirList != 0 {
			f, err = os.Open(filepath.Join(root, pathToDir(path)))
		}

		if err != nil {
			return
		}

		defer f.Close()

		fi, err = f.Stat()
		if err != nil {
			res.WriteHeader(http.StatusNotAcceptable)
			return
		}

		if fi.IsDir() {
			if flag&FDirList != 0 {
				dirList(res, f)
				return
			}
			if flag&FDirRedirect != 0 && path[len(path)-1] != '/' {
				localRedirect(res, req, path+"/")
			}
			return
		}

		if ctype != "" {
			if encoding == lGzip /*|| ext == ".gz" */ {
				res.Header().Set("Content-Encoding", "gzip")
			}
			res.Header().Set("Content-Type", ctype)
		}

		http.ServeContent(res, req, basename, fi.ModTime(), f)
	}
}

func localRedirect(w http.ResponseWriter, r *http.Request, newPath string) {
	if q := r.URL.RawQuery; q != "" {
		newPath += "?" + q
	}
	w.Header().Set("Location", newPath)
	w.WriteHeader(http.StatusMovedPermanently)
}

func dirList(w http.ResponseWriter, f http.File) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<pre>\n")
	fmt.Fprint(w, "<a href=\"../\">..</a>\n")
	for {
		dirs, err := f.Readdir(100)
		if err != nil || len(dirs) == 0 {
			break
		}
		for _, d := range dirs {
			if !d.IsDir() {
				continue
			}
			name := html.EscapeString(d.Name())
			fmt.Fprintf(w, "<a href=\"%s/\">%s/</a>\n", name, name)
		}
		for _, d := range dirs {
			if d.IsDir() {
				continue
			}
			name := html.EscapeString(d.Name())
			fmt.Fprintf(w, "<a href=\"%s\">%s</a>\n", name, name)
		}
	}
	fmt.Fprintf(w, "</pre>\n")
}

func pathToDir(name string) string {
	if filepath.Separator != '/' && strings.IndexRune(name, filepath.Separator) >= 0 ||
		strings.Contains(name, "\x00") {
		return ""
	}
	return filepath.FromSlash(path.Clean("/" + name))
}
