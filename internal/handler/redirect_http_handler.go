package handler

import (
	"net/http"
	"strings"

	"github.com/yu1ec/go-shorten/internal/storage"
)

// RedirectHTTPHandler 处理重定向
type RedirectHTTPHandler struct {
	urlStorage *storage.URLStorage
}

// NewRedirectHTTPHandler 创建重定向处理器
func NewRedirectHTTPHandler(urlStorage *storage.URLStorage) *RedirectHTTPHandler {
	return &RedirectHTTPHandler{
		urlStorage: urlStorage,
	}
}

// ServeHTTP 实现http.Handler接口
func (h *RedirectHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 获取短代码
	shortCode := strings.TrimPrefix(r.URL.Path, "/")
	if shortCode == "" {
		http.NotFound(w, r)
		return
	}

	// 查找URL
	url, err := h.urlStorage.GetURLByCode(shortCode)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// 执行重定向
	http.Redirect(w, r, url.TargetURL, http.StatusFound)
}
