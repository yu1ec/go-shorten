package main

import (
	"encoding/base64"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/yu1ec/go-shorten/internal/handler"
)

// BasicAuthMiddleware 中间件，使用基本认证
func basicAuth(next http.HandlerFunc, username, password string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
			return
		}
		payload, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 || pair[0] != username || pair[1] != password {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
			return
		}
		next(w, r)
	}
}

func main() {
	username := os.Getenv("SHORTEN_AUTH_USER")
	password := os.Getenv("SHORTEN_AUTH_PASS")

	if username == "" || password == "" {
		slog.Warn("For system security, it is recommended to set environment variables. SHORTEN_AUTH_USER and SHORTEN_AUTH_PASS")
	}

	// POST 创建短链接记录
	http.HandleFunc("/shorten", basicAuth(handler.ShortenHandler, username, password))
	// GET 获取短链接记录进行跳转
	http.HandleFunc("/", handler.RedirectHandler)

	log.Println("Starting server on :5768")
	if err := http.ListenAndServe(":5768", nil); err != nil {
		slog.Error("Could not start server", slog.Any("error", err))
		os.Exit(1)
	}
}
