package handler

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"

	"github.com/yu1ec/go-shorten/internal/auth"
	"github.com/yu1ec/go-shorten/internal/session"
	"github.com/yu1ec/go-shorten/internal/storage"
)

// AdminHTTPHandler 管理界面处理器
type AdminHTTPHandler struct {
	urlStorage   *storage.URLStorage
	userManager  *auth.UserManager
	sessionMgr   *session.Manager
	templates    map[string]*template.Template
	baseTemplate *template.Template
}

// NewAdminHTTPHandler 创建管理界面处理器
func NewAdminHTTPHandler(urlStorage *storage.URLStorage, userManager *auth.UserManager, sessionMgr *session.Manager) *AdminHTTPHandler {
	// 加载模板
	templates := make(map[string]*template.Template)

	// 为每个页面模板创建包含layout的完整模板
	templateFiles := []string{
		"dashboard.html", "urls.html", "url_form.html",
	}

	for _, file := range templateFiles {
		// 每个模板都包含layout.html和对应的内容模板
		tmpl := template.Must(template.ParseFiles(
			"templates/layout.html",
			"templates/"+file,
		))
		templates[file] = tmpl
	}

	// 独立模板（不需要layout）
	standaloneFiles := []string{
		"login.html", "error.html",
	}

	for _, file := range standaloneFiles {
		tmpl := template.Must(template.ParseFiles("templates/" + file))
		templates[file] = tmpl
	}

	return &AdminHTTPHandler{
		urlStorage:   urlStorage,
		userManager:  userManager,
		sessionMgr:   sessionMgr,
		templates:    templates,
		baseTemplate: nil, // 不再需要baseTemplate
	}
}

// ServeHTTP 实现http.Handler接口
func (h *AdminHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 路由处理
	switch {
	// 登录相关路由
	case r.URL.Path == "/login" && r.Method == http.MethodGet:
		h.handleLoginPage(w, r)
	case r.URL.Path == "/login" && r.Method == http.MethodPost:
		h.handleLogin(w, r)
	case r.URL.Path == "/logout":
		h.handleLogout(w, r)

	// 管理面板路由
	case r.URL.Path == "/admin" || r.URL.Path == "/admin/":
		h.withAuth(h.handleDashboard)(w, r)

	// URL管理路由
	case r.URL.Path == "/admin/urls" && r.Method == http.MethodGet:
		h.withAuth(h.handleListURLs)(w, r)
	case r.URL.Path == "/admin/urls/new" && r.Method == http.MethodGet:
		h.withAuth(h.handleNewURLForm)(w, r)
	case r.URL.Path == "/admin/urls" && r.Method == http.MethodPost:
		h.withAuth(h.handleCreateURL)(w, r)
	case regexp.MustCompile(`^/admin/urls/([^/]+)/edit$`).MatchString(r.URL.Path) && r.Method == http.MethodGet:
		h.withAuth(h.handleEditURLForm)(w, r)
	case regexp.MustCompile(`^/admin/urls/([^/]+)$`).MatchString(r.URL.Path) && r.Method == http.MethodPost:
		h.withAuth(h.handleUpdateURL)(w, r)
	case regexp.MustCompile(`^/admin/urls/([^/]+)/delete$`).MatchString(r.URL.Path) && r.Method == http.MethodPost:
		h.withAuth(h.handleDeleteURL)(w, r)

	default:
		// 404页面
		h.renderErrorPage(w, "页面不存在", "请检查URL是否正确", http.StatusNotFound)
	}
}

// withAuth 认证中间件
func (h *AdminHTTPHandler) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 获取会话
		session, err := h.sessionMgr.Get(r)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// 检查用户名
		username, ok := session.Values["username"].(string)
		if !ok || username == "" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// 设置上下文
		r = setContextValue(r, "username", username)
		next(w, r)
	}
}

// 渲染模板
func (h *AdminHTTPHandler) renderTemplate(w http.ResponseWriter, name string, data map[string]interface{}) {
	// 直接使用预编译的模板
	tmpl, exists := h.templates[name]
	if !exists {
		h.renderErrorPage(w, "模板错误", fmt.Sprintf("模板 %s 不存在", name), http.StatusInternalServerError)
		return
	}

	// 渲染模板
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("渲染模板 %s 失败: %v\n", name, err)
		http.Error(w, "渲染页面失败", http.StatusInternalServerError)
	}
}

// 渲染错误页面
func (h *AdminHTTPHandler) renderErrorPage(w http.ResponseWriter, title, message string, status int) {
	w.WriteHeader(status)
	h.renderTemplate(w, "error.html", map[string]interface{}{
		"title":   title,
		"message": message,
	})
}

// 提取URL路径参数
func getPathParam(path, pattern string) string {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(path)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// 处理登录页面
func (h *AdminHTTPHandler) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	// 检查是否已登录
	session, err := h.sessionMgr.Get(r)
	if err == nil {
		if username, ok := session.Values["username"].(string); ok && username != "" {
			// 已登录，重定向到管理面板
			http.Redirect(w, r, "/admin", http.StatusFound)
			return
		}
	}

	h.renderTemplate(w, "login.html", map[string]interface{}{
		"title": "登录",
	})
}

// 处理登录请求
func (h *AdminHTTPHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderErrorPage(w, "表单错误", "无法解析表单", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	// 验证用户名和密码
	authenticated, _ := h.userManager.Authenticate(username, password)
	if !authenticated {
		h.renderTemplate(w, "login.html", map[string]interface{}{
			"title":    "登录",
			"error":    "用户名或密码错误",
			"username": username,
		})
		return
	}

	// 创建会话
	session, err := h.sessionMgr.Start(w, r)
	if err != nil {
		h.renderErrorPage(w, "会话错误", "创建会话失败", http.StatusInternalServerError)
		return
	}

	// 设置会话值
	session.Values["username"] = username

	// 重定向到管理面板
	http.Redirect(w, r, "/admin", http.StatusFound)
}

// 处理登出请求
func (h *AdminHTTPHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	// 销毁会话
	h.sessionMgr.Destroy(w, r)

	// 重定向到登录页面
	http.Redirect(w, r, "/login", http.StatusFound)
}

// 处理管理面板
func (h *AdminHTTPHandler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	username := getContextValue(r, "username").(string)

	// 获取所有短链接
	urls, err := h.urlStorage.GetAllURLs()
	if err != nil {
		h.renderErrorPage(w, "错误", "获取链接列表失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.renderTemplate(w, "dashboard.html", map[string]interface{}{
		"title":    "管理面板",
		"username": username,
		"urls":     urls,
		"urlCount": len(urls),
	})
}

// 处理URL列表
func (h *AdminHTTPHandler) handleListURLs(w http.ResponseWriter, r *http.Request) {
	username := getContextValue(r, "username").(string)

	urls, err := h.urlStorage.GetAllURLs()
	if err != nil {
		h.renderErrorPage(w, "错误", "获取链接列表失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.renderTemplate(w, "urls.html", map[string]interface{}{
		"title":    "短链接管理",
		"username": username,
		"urls":     urls,
	})
}

// 处理新建URL表单
func (h *AdminHTTPHandler) handleNewURLForm(w http.ResponseWriter, r *http.Request) {
	username := getContextValue(r, "username").(string)

	h.renderTemplate(w, "url_form.html", map[string]interface{}{
		"title":    "创建短链接",
		"username": username,
		"isNew":    true,
	})
}

// 处理创建URL
func (h *AdminHTTPHandler) handleCreateURL(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderErrorPage(w, "表单错误", "无法解析表单", http.StatusBadRequest)
		return
	}

	username := getContextValue(r, "username").(string)

	targetURL := r.FormValue("target_url")
	shortCode := r.FormValue("short_code")
	remark := r.FormValue("remark")

	// 验证目标URL
	if targetURL == "" {
		h.renderTemplate(w, "url_form.html", map[string]interface{}{
			"title":     "创建短链接",
			"error":     "目标URL不能为空",
			"username":  username,
			"targetURL": targetURL,
			"shortCode": shortCode,
			"remark":    remark,
			"isNew":     true,
		})
		return
	}

	// 如果短代码为空，生成随机短代码
	if shortCode == "" {
		code, err := GenerateRandomCode(6)
		if err != nil {
			h.renderErrorPage(w, "错误", "生成短代码失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		shortCode = code
	}

	// 创建URL记录
	err := h.urlStorage.CreateURL(storage.URLRecord{
		ShortCode: shortCode,
		TargetURL: targetURL,
		Remark:    remark,
	})

	if err != nil {
		h.renderTemplate(w, "url_form.html", map[string]interface{}{
			"title":     "创建短链接",
			"error":     "创建链接失败: " + err.Error(),
			"username":  username,
			"targetURL": targetURL,
			"shortCode": shortCode,
			"remark":    remark,
			"isNew":     true,
		})
		return
	}

	// 重定向到管理面板
	http.Redirect(w, r, "/admin", http.StatusFound)
}

// 处理编辑URL表单
func (h *AdminHTTPHandler) handleEditURLForm(w http.ResponseWriter, r *http.Request) {
	username := getContextValue(r, "username").(string)

	shortCode := getPathParam(r.URL.Path, `^/admin/urls/([^/]+)/edit$`)
	if shortCode == "" {
		h.renderErrorPage(w, "错误", "短链接代码无效", http.StatusBadRequest)
		return
	}

	url, err := h.urlStorage.GetURLByCode(shortCode)
	if err != nil {
		h.renderErrorPage(w, "错误", "链接不存在: "+err.Error(), http.StatusNotFound)
		return
	}

	h.renderTemplate(w, "url_form.html", map[string]interface{}{
		"title":     "编辑短链接",
		"username":  username,
		"isNew":     false,
		"url":       url,
		"shortCode": url.ShortCode,
		"targetURL": url.TargetURL,
		"remark":    url.Remark,
	})
}

// 处理更新URL
func (h *AdminHTTPHandler) handleUpdateURL(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderErrorPage(w, "表单错误", "无法解析表单", http.StatusBadRequest)
		return
	}

	username := getContextValue(r, "username").(string)

	shortCode := getPathParam(r.URL.Path, `^/admin/urls/([^/]+)$`)
	if shortCode == "" {
		h.renderErrorPage(w, "错误", "短链接代码无效", http.StatusBadRequest)
		return
	}

	targetURL := r.FormValue("target_url")
	remark := r.FormValue("remark")

	// 验证目标URL
	if targetURL == "" {
		h.renderTemplate(w, "url_form.html", map[string]interface{}{
			"title":     "编辑短链接",
			"error":     "目标URL不能为空",
			"username":  username,
			"shortCode": shortCode,
			"targetURL": targetURL,
			"remark":    remark,
			"isNew":     false,
		})
		return
	}

	// 更新URL记录
	err := h.urlStorage.UpdateURL(storage.URLRecord{
		ShortCode: shortCode,
		TargetURL: targetURL,
		Remark:    remark,
	})

	if err != nil {
		h.renderTemplate(w, "url_form.html", map[string]interface{}{
			"title":     "编辑短链接",
			"error":     "更新链接失败: " + err.Error(),
			"username":  username,
			"shortCode": shortCode,
			"targetURL": targetURL,
			"remark":    remark,
			"isNew":     false,
		})
		return
	}

	// 重定向到管理面板
	http.Redirect(w, r, "/admin", http.StatusFound)
}

// 处理删除URL
func (h *AdminHTTPHandler) handleDeleteURL(w http.ResponseWriter, r *http.Request) {
	shortCode := getPathParam(r.URL.Path, `^/admin/urls/([^/]+)/delete$`)
	if shortCode == "" {
		h.renderErrorPage(w, "错误", "短链接代码无效", http.StatusBadRequest)
		return
	}

	err := h.urlStorage.DeleteURL(shortCode)
	if err != nil {
		h.renderErrorPage(w, "错误", "删除链接失败: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 重定向到管理面板
	http.Redirect(w, r, "/admin", http.StatusFound)
}

// 上下文键类型，避免冲突
type contextKey string

// 设置上下文值
func setContextValue(r *http.Request, key string, value interface{}) *http.Request {
	ctx := r.Context()
	return r.WithContext(context.WithValue(ctx, contextKey(key), value))
}

// 获取上下文值
func getContextValue(r *http.Request, key string) interface{} {
	return r.Context().Value(contextKey(key))
}
