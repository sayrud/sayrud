package conf

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestInitObservability(t *testing.T) {
	restoreConfigAfterTest(t)

	tests := []struct {
		name        string
		config      string
		wantTracing TracingConfig
		wantMetrics MetricsConfig
	}{
		{
			name: "both enabled with independent settings",
			config: `observability:
  tracing:
    enabled: true
    endpoint: collector:4317
    token: example-token
  metrics:
    enabled: true
    whitelist:
      - "127.0.0.1"
      - "::1"
      - "10.0.0.0/24"
      - "2001:db8::/64"
`,
			wantTracing: TracingConfig{Enabled: true, Endpoint: "collector:4317", Token: "example-token"},
			wantMetrics: MetricsConfig{Enabled: true, Whitelist: []string{"127.0.0.1", "::1", "10.0.0.0/24", "2001:db8::/64"}},
		},
		{
			name:   "omitted observability defaults both to disabled",
			config: "app:\n  port: 2830\n",
		},
		{
			name: "metrics only leaves tracing disabled",
			config: `observability:
  metrics:
    enabled: true
    whitelist: ["127.0.0.1"]
`,
			wantMetrics: MetricsConfig{Enabled: true, Whitelist: []string{"127.0.0.1"}},
		},
		{
			name: "tracing only leaves metrics disabled",
			config: `observability:
  tracing:
    enabled: true
    endpoint: collector:4317
`,
			wantTracing: TracingConfig{Enabled: true, Endpoint: "collector:4317"},
		},
		{
			name: "omitted fields retain zero defaults",
			config: `observability:
  tracing:
    enabled: true
  metrics:
    enabled: true
`,
			wantTracing: TracingConfig{Enabled: true},
			wantMetrics: MetricsConfig{Enabled: true},
		},
		{
			name: "disabled tracing preserves settings while metrics enabled",
			config: `observability:
  tracing:
    enabled: false
    endpoint: collector:4317
    token: example-token
  metrics:
    enabled: true
`,
			wantTracing: TracingConfig{Endpoint: "collector:4317", Token: "example-token"},
			wantMetrics: MetricsConfig{Enabled: true},
		},
		{
			name: "disabled metrics preserves whitelist while tracing enabled",
			config: `observability:
  tracing:
    enabled: true
  metrics:
    enabled: false
    whitelist: ["127.0.0.1"]
`,
			wantTracing: TracingConfig{Enabled: true},
			wantMetrics: MetricsConfig{Whitelist: []string{"127.0.0.1"}},
		},
		{
			name: "omitted enable flags default independently to false",
			config: `observability:
  tracing:
    endpoint: collector:4317
  metrics:
    whitelist: ["127.0.0.1"]
`,
			wantTracing: TracingConfig{Endpoint: "collector:4317"},
			wantMetrics: MetricsConfig{Whitelist: []string{"127.0.0.1"}},
		},
		{
			name: "legacy top-level configuration is ignored",
			config: `tracing:
  enabled: true
  endpoint: legacy-collector:4317
  token: legacy-token
metrics:
  enabled: true
  whitelist: ["127.0.0.1"]
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Observability.Tracing = TracingConfig{}
			Observability.Metrics = MetricsConfig{}
			if err := Init(writeTestConfig(t, test.config)); err != nil {
				t.Fatalf("Init() failed: %v", err)
			}
			if Observability.Tracing != test.wantTracing {
				t.Errorf("Observability.Tracing = %#v, want %#v", Observability.Tracing, test.wantTracing)
			}
			if !reflect.DeepEqual(Observability.Metrics, test.wantMetrics) {
				t.Errorf("Observability.Metrics = %#v, want %#v", Observability.Metrics, test.wantMetrics)
			}
		})
	}
}

func TestInitInvalidObservabilityConfig(t *testing.T) {
	restoreConfigAfterTest(t)
	tests := []struct {
		name   string
		config string
	}{
		{"tracing enabled", "observability:\n  tracing:\n    enabled: invalid\n"},
		{"metrics enabled", "observability:\n  metrics:\n    enabled: invalid\n"},
		{"tracing endpoint", "observability:\n  tracing:\n    endpoint: [collector:4317]\n"},
		{"metrics whitelist", "observability:\n  metrics:\n    whitelist: {invalid: value}\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Observability.Tracing = TracingConfig{}
			Observability.Metrics = MetricsConfig{}
			err := Init(writeTestConfig(t, test.config))
			if err == nil || !strings.Contains(err.Error(), "parse observability") {
				t.Fatalf("Init() error = %v, want an observability parse error", err)
			}
		})
	}
}

func TestInitApp(t *testing.T) {
	restoreConfigAfterTest(t)
	t.Setenv("IP_HEADER", "X-Environment-IP")
	App.IPHeader = ""

	if err := Init(writeTestConfig(t, "app:\n  port: 2830\n")); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if App.Port != 2830 || App.IPHeader != "" {
		t.Errorf("App = %#v, want port 2830 and no IP header", App)
	}

	if err := Init(writeTestConfig(t, "app:\n  port: 2831\n  ip_header: X-Real-IP\n")); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if App.Port != 2831 || App.IPHeader != "X-Real-IP" {
		t.Errorf("App = %#v, want the YAML values regardless of environment variables", App)
	}
}

func TestInitRedis(t *testing.T) {
	restoreConfigAfterTest(t)
	t.Setenv("REDIS_ADDRESS", "env-redis:6380")
	t.Setenv("REDIS_PASSWORD", "env-password")

	config := `redis:
  address: yaml-redis:6379
  username: yaml-user
  password: yaml-password
`
	if err := Init(writeTestConfig(t, config)); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if Redis.Address != "yaml-redis:6379" || Redis.Username != "yaml-user" || Redis.Password != "yaml-password" {
		t.Errorf("Redis = %#v, want the YAML values regardless of environment variables", Redis)
	}
}

func TestInitShortcut(t *testing.T) {
	restoreConfigAfterTest(t)

	if err := Init(writeTestConfig(t, "app:\n  port: 2830\n")); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if Shortcut.Workers != 4 {
		t.Errorf("defaults = %#v", Shortcut)
	}

	if err := Init(writeTestConfig(t, "shortcut:\n  workers: 2\n")); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if Shortcut.Workers != 2 {
		t.Errorf("configured = %#v", Shortcut)
	}
}

func TestInitStorage(t *testing.T) {
	restoreConfigAfterTest(t)

	Storage = StorageConfig{}
	if err := Init(writeTestConfig(t, "app:\n  port: 2830\n")); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	want := StorageConfig{Type: "local", URLExpiry: time.Hour, Local: LocalStorageConfig{Path: "./data/storage"}}
	if Storage != want {
		t.Errorf("defaults = %#v", Storage)
	}

	Storage = StorageConfig{}
	if err := Init(writeTestConfig(t, `storage:
  type: s3
  url_expiry: 15m
  s3:
    endpoint: http://127.0.0.1:9000
    region: auto
    bucket: sayrud
    access_key_id: id
    secret_access_key: secret
    use_path_style: true
    base_path: files
    public_url: https://cdn.example.com
`)); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	want = StorageConfig{
		Type:      "s3",
		URLExpiry: 15 * time.Minute,
		Local:     LocalStorageConfig{Path: "./data/storage"},
		S3: S3StorageConfig{
			Endpoint: "http://127.0.0.1:9000", Region: "auto", Bucket: "sayrud",
			AccessKeyID: "id", SecretAccessKey: "secret", UsePathStyle: true,
			BasePath: "files", PublicURL: "https://cdn.example.com",
		},
	}
	if Storage != want {
		t.Errorf("configured = %#v", Storage)
	}
}

func restoreConfigAfterTest(t *testing.T) {
	t.Helper()
	previousApp, previousPostgres, previousRedis, previousObservability := App, Postgres, Redis, Observability
	previousShortcut, previousStorage := Shortcut, Storage
	t.Cleanup(func() {
		App, Postgres, Redis, Observability = previousApp, previousPostgres, previousRedis, previousObservability
		Shortcut, Storage = previousShortcut, previousStorage
	})
}

func writeTestConfig(t *testing.T, config string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sayrud.yaml")
	if err := os.WriteFile(path, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
