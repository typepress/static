// static file Handler, support gzip precompression
package static

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var gzTypes = map[string]string{
	".css":  "text/css; charset=utf-8",
	".html": "text/html; charset=utf-8",
	".js":   "application/x-javascript; charset=utf-8",
}

var lock sync.RWMutex

func init() {
	mime.AddExtensionType(".gz", "application/gzip")
}

// 为扩展名为 ext 的文件提供预压缩输出支持
func PreGzip(ext string) error {
	if ext == "" {
		return errors.New(`static: extension is empty`)
	}
	if ext[0] != '.' {
		ext = "." + ext
	}
	mimeType := mime.TypeByExtension(ext)
	if len(mimeType) == 0 {
		mimeType = "application/octet-stream"
	}
	lock.Lock()
	gzTypes[ext] = mimeType
	lock.Unlock()
	return nil
}

func getGzipType(ext string) string {
	lock.RLock()
	ext = gzTypes[ext]
	lock.RUnlock()
	return ext
}

// Static file Handler, support Gzip
func Handler(res http.ResponseWriter, req *http.Request, dir http.Dir) {
	const indexPage = string(filepath.Separator) + "index.html"
	if string(dir) == "" || req.Method != "GET" && req.Method != "HEAD" {
		return
	}
	file := filepath.Join(string(dir), req.URL.Path)
	if strings.HasSuffix(file, indexPage) {
		localRedirect(res, req, "./")
		return
	}
	var hasSlash bool
	var ext, ctype string

	if strings.HasSuffix(req.URL.Path, "/") {
		hasSlash = true
		file = filepath.Join(file, "index.html")
		ext = ".html"
	} else {
		ext = filepath.Ext(file)
	}

	// /path/to/dir redirect /path/to/dir/
	if !hasSlash && len(ext) == 0 {
		fi, _ := os.Stat(file)
		if fi != nil && fi.IsDir() {
			localRedirect(res, req, req.URL.Path+"/")
			return
		}
	}

	if ext == ".gz" {
		trySendGzipFile(res, req, file, ctype)
		return
	}

	ctype = getGzipType(ext)
	if len(ctype) != 0 {
		trySendGzipFile(res, req, file+".gz", ctype)
		if res.Header().Get("Content-Encoding") != "" {
			return
		}
	}

	f, _ := os.Open(file)
	if f == nil {
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return
	}
	// /path/to/dir.ext/ ??
	if stat.IsDir() {
		return
	}

	http.ServeContent(res, req, file, stat.ModTime(), f)
}

// 尝试发送一个 gzip 压缩的文件.
// 如果文件存在返回 true, 发送并添加 Header "Content-Encoding: gzip".
// filename 是文件完整路径, 扩展名必须是 ".gz"
// ctype 是 Content-Type, 为空 自动判断
func trySendGzipFile(res http.ResponseWriter, req *http.Request, filename, ctype string) {
	if strings.Contains(filename, "\x00") ||
		res.Header().Get("Content-Encoding") != "" ||
		!strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") {
		return
	}

	ext := filepath.Ext(filename)
	if ext != ".gz" {
		return
	}

	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil || fi.IsDir() {
		return
	}

	if ctype == "" {
		ext = filepath.Ext(filename[:len(filename)-3])
		ctype = mime.TypeByExtension(ext)
	}

	res.Header().Set("Content-Encoding", "gzip")
	if ctype != "" {
		res.Header().Set("Content-Type", ctype)
	}
	http.ServeContent(res, req, filename, fi.ModTime(), f)
}

func localRedirect(w http.ResponseWriter, r *http.Request, newPath string) {
	if q := r.URL.RawQuery; q != "" {
		newPath += "?" + q
	}
	w.Header().Set("Location", newPath)
	w.WriteHeader(http.StatusMovedPermanently)
}
