package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/yu1ec/go-shorten/internal/auth"
	"github.com/yu1ec/go-shorten/internal/handler"
	"github.com/yu1ec/go-shorten/internal/session"
	"github.com/yu1ec/go-shorten/internal/storage"
)

func main() {
	// 初始化存储层
	urlStorage, err := storage.NewURLStorage()
	if err != nil {
		slog.Error("初始化URL存储失败", slog.Any("error", err))
		os.Exit(1)
	}

	// 初始化用户管理器
	userManager, err := auth.NewUserManager()
	if err != nil {
		slog.Error("初始化用户管理器失败", slog.Any("error", err))
		os.Exit(1)
	}

	// 初始化会话管理器
	sessionMgr := session.NewManager("go-shorten-session", 24*time.Hour)
	sessionMgr.StartGCTimer()

	// 创建HTTP处理器
	mux := http.NewServeMux()

	// 创建API处理器
	apiHandler := handler.NewAPIHTTPHandler(urlStorage, userManager)
	mux.Handle("/api/shorten", apiHandler)

	// 创建管理界面处理器
	adminHandler := handler.NewAdminHTTPHandler(urlStorage, userManager, sessionMgr)

	// 登录相关路由
	mux.Handle("/login", adminHandler)
	mux.Handle("/logout", adminHandler)

	// 管理面板路由
	mux.Handle("/admin", adminHandler)
	mux.Handle("/admin/", adminHandler)

	// 重定向处理器（必须放在最后注册，因为它处理所有根路径下的请求）
	redirectHandler := handler.NewRedirectHTTPHandler(urlStorage)
	mux.Handle("/", redirectHandler)

	// 启动服务器
	port := os.Getenv("PORT")
	if port == "" {
		port = "5768"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Println("Starting server on :" + port)
	if err := server.ListenAndServe(); err != nil {
		slog.Error("启动服务器失败", slog.Any("error", err))
		os.Exit(1)
	}
}
