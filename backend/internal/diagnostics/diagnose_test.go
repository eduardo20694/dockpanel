package diagnostics

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		d    Diagnosis
		want Severity
	}{
		{
			name: "ok running container",
			d:    Diagnosis{State: "running", ExitCode: 0},
			want: SeverityOK,
		},
		{
			name: "oom killed",
			d:    Diagnosis{OOMKilled: true, State: "exited"},
			want: SeverityCritical,
		},
		{
			name: "nonzero exit while stopped",
			d:    Diagnosis{State: "exited", ExitCode: 1},
			want: SeverityCritical,
		},
		{
			name: "crash loop",
			d:    Diagnosis{State: "running", RestartCount: 5},
			want: SeverityCritical,
		},
		{
			name: "error lines while running",
			d:    Diagnosis{State: "running", ErrorLines: []string{"connection refused"}},
			want: SeverityWarning,
		},
		{
			name: "restarting state",
			d:    Diagnosis{State: "restarting"},
			want: SeverityWarning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classify(&tt.d); got != tt.want {
				t.Fatalf("classify() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRecommend(t *testing.T) {
	tests := []struct {
		name string
		d    Diagnosis
		want string
	}{
		{
			name: "oom",
			d:    Diagnosis{OOMKilled: true},
			want: "aumente o limite de memória do container (mem_limit no compose) ou investigue vazamento de memória na aplicação",
		},
		{
			name: "crash loop",
			d:    Diagnosis{RestartCount: 5},
			want: "provável crash loop — veja as linhas de erro abaixo e o log completo antes de reiniciar de novo; reiniciar sem entender a causa só adia o problema",
		},
		{
			name: "exit 137",
			d:    Diagnosis{ExitCode: 137},
			want: "exit code 137 costuma ser SIGKILL — geralmente OOM ou um `docker stop` forçado; confira memória disponível no host",
		},
		{
			name: "exit 1 with error lines",
			d:    Diagnosis{ExitCode: 1, ErrorLines: []string{"fatal: boom"}},
			want: "erro de aplicação — veja as linhas de log filtradas abaixo, é provável que a causa esteja ali",
		},
		{
			name: "error lines only",
			d:    Diagnosis{ErrorLines: []string{"timeout"}},
			want: "container rodando, mas com mensagens de erro no log — vale investigar mesmo sem estar caindo",
		},
		{
			name: "exit code without log lines",
			d:    Diagnosis{State: "exited", ExitCode: 1},
			want: "processo encerrou com exit code 1 — confira os logs completos do container",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := recommend(&tt.d); got != tt.want {
				t.Fatalf("recommend() = %q, want %q", got, tt.want)
			}
		})
	}
}
