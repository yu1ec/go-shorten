package handler

import (
	"encoding/json"
	"net/http"

	"github.com/yu1ec/go-shorten/internal/auth"
	"github.com/yu1ec/go-shorten/internal/storage"
)

// APIRequest API请求体
type APIRequest struct {
	TargetURL string `json:"target_url"`
	ShortCode string `json:"short_code,omitempty"`
	Remark    string `json:"remark,omitempty"`
}

// APIResponse API响应体
type APIResponse struct {
	ShortCode  string `json:"short_code"`
	TargetURL  string `json:"target_url"`
	ShortURL   string `json:"short_url,omitempty"`
	Remark     string `json:"remark,omitempty"`
	CreateTime string `json:"create_time,omitempty"`
}

// APIHTTPHandler API处理器
type APIHTTPHandler struct {
	urlStorage  *storage.URLStorage
	userManager *auth.UserManager
}

// NewAPIHTTPHandler 创建API处理器
func NewAPIHTTPHandler(urlStorage *storage.URLStorage, userManager *auth.UserManager) *APIHTTPHandler {
	return &APIHTTPHandler{
		urlStorage:  urlStorage,
		userManager: userManager,
	}
}

// ServeHTTP 实现http.Handler接口
func (h *APIHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 基本认证
	username, password, ok := r.BasicAuth()
	if !ok || !h.userManager.AuthenticateBasic(username, password) {
		w.Header().Set("WWW-Authenticate", "Basic realm=\"Authorization Required\"")
		http.Error(w, "未授权", http.StatusUnauthorized)
		return
	}

	// 只处理POST请求
	if r.Method != http.MethodPost {
		http.Error(w, "方法不被允许", http.StatusMethodNotAllowed)
		return
	}

	// 解析JSON请求体
	var request APIRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "无效的请求格式", http.StatusBadRequest)
		return
	}

	// 验证目标URL
	if request.TargetURL == "" {
		http.Error(w, "目标URL不能为空", http.StatusBadRequest)
		return
	}

	// 如果短代码为空，生成随机短代码
	if request.ShortCode == "" {
		code, err := GenerateRandomCode(6)
		if err != nil {
			http.Error(w, "生成短代码失败", http.StatusInternalServerError)
			return
		}
		request.ShortCode = code
	}

	// 创建URL记录
	err := h.urlStorage.CreateURL(storage.URLRecord{
		ShortCode: request.ShortCode,
		TargetURL: request.TargetURL,
		Remark:    request.Remark,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 获取完整的短链接URL
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	shortURL := scheme + "://" + r.Host + "/" + request.ShortCode

	// 返回结果
	response := APIResponse{
		ShortCode: request.ShortCode,
		TargetURL: request.TargetURL,
		ShortURL:  shortURL,
		Remark:    request.Remark,
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// 写入JSON响应
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "编码响应失败", http.StatusInternalServerError)
		return
	}
}
