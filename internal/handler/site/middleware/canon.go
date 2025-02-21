package middleware

import (
	"net/http"
	"os"
	"path"
	"strings"

	handler "github.com/oursky/pageship/internal/handler/site"
	"github.com/oursky/pageship/internal/site"
)

func CanonicalizePath(fs site.FS, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlpath, err := canonicalizePath(fs, r.URL.Path)
		if err != nil {
			handler.Error(w, r, err)
			return
		} else if r.URL.Path != urlpath {
			http.Redirect(w, r, urlpath, http.StatusMovedPermanently)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func canonicalizePath(fs site.FS, urlpath string) (string, error) {
	const indexPage = "index.html"

	urlpath = path.Clean(urlpath)

	// redirect .../index.html to .../
	if strings.HasSuffix(urlpath, "/"+indexPage) {
		urlpath = strings.TrimSuffix(urlpath, indexPage)
	}

	var isDir bool
	f, err := fs.Stat(urlpath)
	if os.IsNotExist(err) {
		isDir = false
	} else if err != nil {
		return "", err
	} else {
		isDir = f.IsDir
	}

	if isDir {
		if urlpath[len(urlpath)-1] != '/' {
			urlpath = urlpath + "/"
		}
	} else {
		if urlpath[len(urlpath)-1] == '/' {
			urlpath = urlpath[:len(urlpath)-1]
		}
	}

	return urlpath, nil
}
