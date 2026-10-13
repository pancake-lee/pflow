// Package desktop connects the desktop dashboard to a Linux pflow service.
package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type connection struct {
	ServerURL string `json:"server_url"`
}

// Handler stores the selected service and proxies desktop API requests.
type Handler struct {
	mu        sync.RWMutex
	path      string
	value     connection
	proxy     *httputil.ReverseProxy
	transport *http.Transport
}

// NewHandler loads the desktop connection file, which is separate from
// preferences stored by the remote service.
func NewHandler(path string) (*Handler, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 15 * time.Second
	h := &Handler{path: path, transport: transport}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return h, nil
	}
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &h.value)
	if err != nil {
		return nil, fmt.Errorf("read desktop connection: %w", err)
	}
	if h.value.ServerURL != "" {
		target, err := parseServerURL(h.value.ServerURL)
		if err != nil {
			return nil, err
		}
		h.proxy = h.newProxy(target)
	}
	return h, nil
}

func parseServerURL(value string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("服务地址无效")
	}
	if (u.Scheme != "http" && u.Scheme != "https") ||
		u.Hostname() == "" || u.User != nil || u.RawQuery != "" ||
		u.Fragment != "" || u.Opaque != "" {
		return nil, fmt.Errorf("请输入 HTTP 或 HTTPS 服务地址，不包含账号、查询参数或片段")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = strings.TrimRight(u.RawPath, "/")
	return u, nil
}

func (h *Handler) newProxy(target *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		r.Host = target.Host
	}
	proxy.Transport = h.transport
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("desktop proxy %s %s: %v", r.Method, r.URL.Path, err)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "无法连接 pflow 服务，请检查服务地址和网络连接",
		})
	}
	return proxy
}

// Close releases idle connections when the desktop application exits.
func (h *Handler) Close() {
	h.transport.CloseIdleConnections()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/desktop/connection" {
		h.handleConnection(w, r)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/terminal/") {
		writeJSON(w, http.StatusNotImplemented, map[string]string{
			"error": "桌面版暂不支持 Web 终端，请使用已有终端连接 Agent",
		})
		return
	}
	h.mu.RLock()
	proxy := h.proxy
	h.mu.RUnlock()
	if proxy == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "请先配置 pflow 服务地址",
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	proxy.ServeHTTP(w, r.WithContext(ctx))
}

func (h *Handler) handleConnection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.mu.RLock()
		value := h.value
		h.mu.RUnlock()
		writeJSON(w, http.StatusOK, value)
	case http.MethodPut:
		var value connection
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		err := decoder.Decode(&value)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "连接配置格式无效",
			})
			return
		}
		target, err := parseServerURL(value.ServerURL)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}
		value.ServerURL = target.String()
		h.mu.Lock()
		defer h.mu.Unlock()
		err = saveConnection(h.path, value)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "无法保存连接配置，请检查用户目录权限",
			})
			return
		}
		h.value = value
		h.proxy = h.newProxy(target)
		h.transport.CloseIdleConnections()
		writeJSON(w, http.StatusOK, value)
	default:
		w.Header().Set("Allow", "GET, PUT")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "不支持的请求方法",
		})
	}
}

func saveConnection(path string, value connection) error {
	err := os.MkdirAll(filepath.Dir(path), 0700)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "desktop-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = json.NewEncoder(f).Encode(value)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	err = errors.Join(err, closeErr)
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
