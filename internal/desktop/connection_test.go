package desktop

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pancake-lee/pflow/internal/api"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	h, err := NewHandler(filepath.Join(t.TempDir(), "desktop.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	return h
}

func request(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	return w
}

func selectService(t *testing.T, h *Handler, target string) {
	t.Helper()
	body, err := json.Marshal(connection{ServerURL: target})
	if err != nil {
		t.Fatal(err)
	}
	w := request(h, "PUT", "/desktop/connection", string(body))
	if w.Code != http.StatusOK {
		t.Fatalf("save connection: %d %s", w.Code, w.Body.String())
	}
}

func TestConnectionProxyAndSwitch(t *testing.T) {
	h := newTestHandler(t)
	var expectedHost string
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/prefix/api/v1/schedules" ||
			r.URL.RawQuery != "date=2026-10-13" || r.Method != "PUT" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing content type")
		}
		if r.Host != expectedHost {
			t.Error("proxy did not replace desktop host")
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"items":[]}` {
			t.Errorf("unexpected body: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"remote conflict"}`))
	}))
	defer remote.Close()
	expectedHost = strings.TrimPrefix(remote.URL, "http://")
	selectService(t, h, remote.URL+"/prefix/")
	r := httptest.NewRequest("PUT", "/api/v1/schedules?date=2026-10-13",
		strings.NewReader(`{"items":[]}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusConflict || w.Body.String() != `{"error":"remote conflict"}` {
		t.Fatalf("remote response not preserved: %d %s", w.Code, w.Body)
	}

	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"source":"second"}`))
	}))
	defer other.Close()
	selectService(t, h, other.URL)
	loaded, err := NewHandler(h.path)
	if err != nil {
		t.Fatal(err)
	}
	defer loaded.Close()
	for _, handler := range []*Handler{h, loaded} {
		w = request(handler, "GET", "/api/v1/dashboard", "")
		if w.Code != http.StatusOK || w.Body.String() != `{"source":"second"}` {
			t.Fatalf("service switch/reload failed: %d %s", w.Code, w.Body)
		}
	}
}

func TestConnectionErrors(t *testing.T) {
	h := newTestHandler(t)
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/desktop/connection", "", 200},
		{"GET", "/api/v1/dashboard", "", 503},
		{"GET", "/api/v1/terminal/lookup", "", 501},
		{"POST", "/api/v1/terminal/start", "{}", 501},
		{"GET", "/not-an-api", "", 404},
		{"POST", "/desktop/connection", "{}", 405},
		{"PUT", "/desktop/connection", "{", 400},
		{"PUT", "/desktop/connection", `{"server_url":"file:///etc/passwd"}`, 400},
		{"PUT", "/desktop/connection", `{"server_url":"http://user:pass@host"}`, 400},
		{"PUT", "/desktop/connection", `{"server_url":"http://host?token=secret"}`, 400},
		{"PUT", "/desktop/connection", `{"server_url":"http://host#part"}`, 400},
		{"PUT", "/desktop/connection", `{"server_url":""}`, 400},
	} {
		t.Run(tc.method+tc.path+tc.body, func(t *testing.T) {
			w := request(h, tc.method, tc.path, tc.body)
			if w.Code != tc.status {
				t.Fatalf("want %d, got %d: %s", tc.status, w.Code, w.Body)
			}
		})
	}
	remote := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	selectService(t, h, remote.URL)
	remote.Close()
	w := request(h, "GET", "/api/v1/dashboard", "")
	if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), "无法连接") {
		t.Fatalf("missing connection error: %d %s", w.Code, w.Body)
	}
}

func TestFailedSaveKeepsSelectedService(t *testing.T) {
	h := newTestHandler(t)
	selectService(t, h, "http://localhost:10002")
	// A file in place of the parent directory forces persistence to fail.
	blocker := filepath.Join(t.TempDir(), "file")
	err := os.WriteFile(blocker, []byte("blocked"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	h.path = filepath.Join(blocker, "desktop.json")
	w := request(h, "PUT", "/desktop/connection", `{"server_url":"http://localhost:10003"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected save status: %d", w.Code)
	}
	w = request(h, "GET", "/desktop/connection", "")
	if !strings.Contains(w.Body.String(), "10002") {
		t.Fatal("failed save changed active connection")
	}
}

func TestInvalidStoredConnection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop.json")
	for _, body := range []string{"{", `{"server_url":"file:///tmp"}`} {
		err := os.WriteFile(path, []byte(body), 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, err = NewHandler(path)
		if err == nil {
			t.Fatal("invalid config accepted")
		}
	}
}

func TestRemoteProjectLifecycle(t *testing.T) {
	// Keep remote service data isolated from the user's real ~/.pflow.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	remote := httptest.NewServer(api.NewServer(nil, nil))
	defer remote.Close()
	h := newTestHandler(t)
	selectService(t, h, remote.URL)
	for _, tc := range []struct {
		method, path, body string
	}{
		{"PUT", "/api/v1/project-roots", `{"path":"/remote/project","priority":"normal"}`},
		{"GET", "/api/v1/project-roots", ""},
		{"DELETE", "/api/v1/project-roots?path=%2Fremote%2Fproject", ""},
	} {
		w := request(h, tc.method, tc.path, tc.body)
		if w.Code != http.StatusOK {
			t.Fatalf("remote %s: %d %s", tc.method, w.Code, w.Body)
		}
		if tc.method == "GET" && !strings.Contains(w.Body.String(), "/remote/project") {
			t.Fatal("remote project was not saved")
		}
	}
	w := request(h, "GET", "/api/v1/project-roots", "")
	if strings.Contains(w.Body.String(), "/remote/project") {
		t.Fatal("remote project was not deleted")
	}
}
