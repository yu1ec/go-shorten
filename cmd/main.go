package main

import (
	"log"
	"net/http"

	"github.com/yu1ec/go-shorten/internal/handler"
)

func main() {
	// POST 创建短链接记录
	http.HandleFunc("/shorten", handler.ShortenHandler)
	// GET 获取短链接记录进行跳转
	http.HandleFunc("/", handler.RedirectHandler)

	log.Println("Starting server on :5768")
	if err := http.ListenAndServe(":5768", nil); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
