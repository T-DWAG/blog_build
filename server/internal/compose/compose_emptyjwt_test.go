package compose

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestCompose_EmptyJWT 验证缺 JWT_SECRET 时 api 不会带着零值跑起来：
// 应退出/重启，日志里能看到 "missing JWT_SECRET"。
// 测试结束后会把 .env 和 api 服务恢复为正常状态，不影响同一次 test-s11 里其它测试。
func TestCompose_EmptyJWT(t *testing.T) {
	dir := serverDir(t)
	envPath := envFilePath(t)

	// 写一份缺 JWT_SECRET 的 .env，重启 api 单个服务
	badEnv := "JWT_SECRET=\nADMIN_USERNAME=" + testAdminUser + "\nADMIN_PASSWORD=" + testAdminPassword + "\n"
	if err := os.WriteFile(envPath, []byte(badEnv), 0o600); err != nil {
		t.Fatalf("write bad .env: %v", err)
	}

	defer func() {
		// 还原正常 .env 并重启 api，保证测试结束后 stack 恢复健康，不影响其它测试。
		writeEnvFileDirect(envPath, testJWTSecret)
		up := exec.Command("docker", "compose", "up", "-d", "api")
		up.Dir = dir
		_ = up.Run()
		_ = waitHealthy(60 * time.Second)
	}()

	up := exec.Command("docker", "compose", "up", "-d", "api")
	up.Dir = dir
	_, _ = up.CombinedOutput() // up 本身可能不报错，容器起来后立刻退出

	// 给容器一点时间跑到 config.Load 失败退出
	time.Sleep(5 * time.Second)

	logsCmd := exec.Command("docker", "compose", "logs", "api")
	logsCmd.Dir = dir
	logsOut, err := logsCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker compose logs api: %v", err)
	}
	if !strings.Contains(string(logsOut), "missing JWT_SECRET") {
		t.Fatalf("日志应包含 missing JWT_SECRET, got:\n%s", logsOut)
	}

	psCmd := exec.Command("docker", "compose", "ps", "--format", "{{.Service}} {{.State}}", "api")
	psCmd.Dir = dir
	psOut, err := psCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker compose ps api: %v", err)
	}
	text := strings.ToLower(string(psOut))
	if strings.Contains(text, "running") && !strings.Contains(text, "restarting") {
		t.Fatalf("api 不应处于稳定 running 状态（应退出/重启），ps 输出:\n%s", psOut)
	}
}
