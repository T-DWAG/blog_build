package compose

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestCompose_Healthz 验证 api 容器起来后 /healthz 返回 code==0。
func TestCompose_Healthz(t *testing.T) {
	resp, err := http.Get(healthzURL)
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, b)
	}
	if !strings.Contains(string(b), `"code":0`) {
		t.Fatalf("body 应含 code:0, got %s", b)
	}
}

// TestCompose_LoginPage 验证管理台登录页可访问（embed 静态资源生效）。
func TestCompose_LoginPage(t *testing.T) {
	resp, err := http.Get("http://localhost:8080/admin/login")
	if err != nil {
		t.Fatalf("GET /admin/login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

// TestCompose_AdminFont 验证 embed 进二进制的字体资源在容器内可访问，
// 不依赖 FRONTEND_DIR / 挂载 frontend/。
func TestCompose_AdminFont(t *testing.T) {
	resp, err := http.Get("http://localhost:8080/admin-assets/fonts/fusion-pixel-12px-monospaced-zh_hans.ttf.woff2")
	if err != nil {
		t.Fatalf("GET font: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}
