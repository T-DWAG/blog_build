// Package compose 端到端验证 S11：docker compose up 起 db+api，
// 环境变量缺失时进程应退出而不是带着零值跑起来。
// 运行前提：本机已装 Docker，且 5432/8080 端口未被占用（先停掉手动起的 db / blog-server）。
package compose

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const (
	testJWTSecret     = "s11-test-secret-please-ignore"
	testAdminUser     = "admin"
	testAdminPassword = "s11-test-password"
	healthzURL        = "http://localhost:8080/healthz"
)

// serverDir 返回 server/ 目录（docker-compose.yml 所在处）的绝对路径。
func serverDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Join(wd, "..", "..")
}

func envFilePath(t *testing.T) string {
	return filepath.Join(serverDir(t), ".env")
}

// waitHealthy 轮询 /healthz 直到 200 或超时。
func waitHealthy(timeout time.Duration) error {
	client := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := client.Get(healthzURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("healthz status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("healthz not ready within %s: %v", timeout, lastErr)
}

var envBackup []byte
var envExistedBefore bool

func TestMain(m *testing.M) {
	srvDir := ""
	if wd, err := os.Getwd(); err == nil {
		srvDir = filepath.Join(wd, "..", "..")
	}
	envPath := filepath.Join(srvDir, ".env")
	if b, err := os.ReadFile(envPath); err == nil {
		envBackup = b
		envExistedBefore = true
	}

	writeEnvFileDirect(envPath, testJWTSecret)

	upCmd := exec.Command("docker", "compose", "up", "-d", "--build")
	upCmd.Dir = srvDir
	out, err := upCmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		fmt.Println("docker compose up failed:", err)
		cleanupEnv(envPath)
		os.Exit(1)
	}

	if err := waitHealthy(120 * time.Second); err != nil {
		fmt.Println("stack never became healthy:", err)
		logsCmd := exec.Command("docker", "compose", "logs", "api")
		logsCmd.Dir = srvDir
		logsOut, _ := logsCmd.CombinedOutput()
		fmt.Println(string(logsOut))
		downCmd := exec.Command("docker", "compose", "down")
		downCmd.Dir = srvDir
		_ = downCmd.Run()
		cleanupEnv(envPath)
		os.Exit(1)
	}

	code := m.Run()

	downCmd := exec.Command("docker", "compose", "down")
	downCmd.Dir = srvDir
	_ = downCmd.Run()
	cleanupEnv(envPath)

	os.Exit(code)
}

func writeEnvFileDirect(path, jwtSecret string) {
	content := fmt.Sprintf("JWT_SECRET=%s\nADMIN_USERNAME=%s\nADMIN_PASSWORD=%s\n",
		jwtSecret, testAdminUser, testAdminPassword)
	_ = os.WriteFile(path, []byte(content), 0o600)
}

func cleanupEnv(path string) {
	if envExistedBefore {
		_ = os.WriteFile(path, envBackup, 0o600)
	} else {
		_ = os.Remove(path)
	}
}
