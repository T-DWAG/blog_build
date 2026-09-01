package compose

import (
	"os/exec"
	"strings"
	"testing"
)

// TestCompose_Ps 验证 db、api 两个服务都在 running 状态。
func TestCompose_Ps(t *testing.T) {
	cmd := exec.Command("docker", "compose", "ps", "--format", "{{.Service}} {{.State}}")
	cmd.Dir = serverDir(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker compose ps: %v, out=%s", err, out)
	}
	text := string(out)
	for _, svc := range []string{"db", "api"} {
		found := false
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, svc+" ") && strings.Contains(line, "running") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("service %q 未处于 running 状态, ps 输出:\n%s", svc, text)
		}
	}
}
