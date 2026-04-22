package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eryajf/kite-desktop/pkg/common"
	"github.com/eryajf/kite-desktop/pkg/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "kite-cluster-tests-*")
	if err != nil {
		panic(err)
	}

	common.DBType = "sqlite"
	common.DBDSN = filepath.Join(tempDir, "cluster-test.db")
	model.InitDB()

	exitCode := m.Run()
	if err := os.RemoveAll(tempDir); err != nil {
		fmt.Fprintf(os.Stderr, "cleanup temp dir %q failed: %v\n", tempDir, err)
		if exitCode == 0 {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

func TestFormatClusterConnectionError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		err         error
		contains    string
		detail      string
		notContains string
	}{
		{
			name:     "timeout",
			err:      context.DeadlineExceeded,
			contains: "timed out after 12s",
			detail:   context.DeadlineExceeded.Error(),
		},
		{
			name:     "dns",
			err:      errors.New("lookup demo.example.invalid: no such host"),
			contains: "Failed to resolve the Kubernetes API Server host.",
			detail:   "lookup demo.example.invalid: no such host",
		},
		{
			name:     "tls",
			err:      errors.New("x509: certificate signed by unknown authority"),
			contains: "TLS certificate validation failed.",
			detail:   "x509: certificate signed by unknown authority",
		},
		{
			name:     "default passthrough",
			err:      errors.New("plain failure"),
			contains: "Cluster connection test failed.",
			detail:   "plain failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := formatClusterConnectionError(tt.err)
			if got == nil {
				t.Fatal("expected formatted error, got nil")
			}
			if !strings.Contains(got.Error(), tt.contains) {
				t.Fatalf("formatted error = %q, want substring %q", got.Error(), tt.contains)
			}
			var connectionErr *clusterConnectionError
			if !errors.As(got, &connectionErr) {
				t.Fatalf("expected clusterConnectionError, got %T", got)
			}
			if connectionErr.Code == "" {
				t.Fatalf("expected error code, got %+v", connectionErr)
			}
			if tt.detail != "" && !strings.Contains(connectionErr.Detail, tt.detail) {
				t.Fatalf("error detail = %q, want substring %q", connectionErr.Detail, tt.detail)
			}
			if tt.notContains != "" && strings.Contains(connectionErr.Detail, tt.notContains) {
				t.Fatalf("error detail = %q, unexpected substring %q", connectionErr.Detail, tt.notContains)
			}
		})
	}
}

func TestTestClusterConnectionSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalTester := clusterConnectionTester
	t.Cleanup(func() {
		clusterConnectionTester = originalTester
	})

	clusterConnectionTester = func(cluster *model.Cluster) (*ClientSet, error) {
		if cluster.Name != "demo" {
			t.Fatalf("cluster.Name = %q, want %q", cluster.Name, "demo")
		}
		if string(cluster.Config) != "apiVersion: v1" {
			t.Fatalf("cluster.Config = %q, want kubeconfig body", string(cluster.Config))
		}
		return &ClientSet{Version: "v1.30.0"}, nil
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/clusters/test",
		strings.NewReader(`{"name":"demo","config":"apiVersion: v1"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	(&ClusterManager{}).TestClusterConnection(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Message string `json:"message"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Version != "v1.30.0" {
		t.Fatalf("version = %q, want %q", response.Version, "v1.30.0")
	}
}

func TestTestClusterConnectionReturnsReadableError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalTester := clusterConnectionTester
	t.Cleanup(func() {
		clusterConnectionTester = originalTester
	})

	clusterConnectionTester = func(cluster *model.Cluster) (*ClientSet, error) {
		return nil, &clusterConnectionError{
			Code:    clusterConnectionErrorTimeout,
			Message: "Connection test timed out after 12s.",
			Detail:  "context deadline exceeded",
		}
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/clusters/test",
		strings.NewReader(`{"name":"demo","config":"apiVersion: v1"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	(&ClusterManager{}).TestClusterConnection(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	var response struct {
		Error       string `json:"error"`
		ErrorCode   string `json:"errorCode"`
		ErrorDetail string `json:"errorDetail"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.ErrorCode != clusterConnectionErrorTimeout {
		t.Fatalf("errorCode = %q, want %q", response.ErrorCode, clusterConnectionErrorTimeout)
	}
	if !strings.Contains(response.Error, "timed out") {
		t.Fatalf("error = %q, want timeout hint", response.Error)
	}
	if response.ErrorDetail == "" {
		t.Fatalf("expected errorDetail, got empty response: %+v", response)
	}
}

func TestImportClustersFromKubeconfigSkipsClustersWithSameAPIServer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupClusterHandlerTestDB(t)

	if err := model.AddCluster(&model.Cluster{
		Name:      "existing",
		Config:    model.SecretString(existingClusterKubeconfig),
		IsDefault: true,
		Enable:    true,
	}); err != nil {
		t.Fatalf("AddCluster() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/clusters/import",
		strings.NewReader(`{"config":"`+jsonEscape(t, sameServerDifferentNameKubeconfig)+`","inCluster":false}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	(&ClusterManager{}).ImportClustersFromKubeconfig(ctx)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}

	count, err := model.CountClusters()
	if err != nil {
		t.Fatalf("CountClusters() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("CountClusters() = %d, want 1", count)
	}

	if _, err := model.GetClusterByName("same-server-new-name"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetClusterByName(same-server-new-name) err = %v, want record not found", err)
	}

	existing, err := model.GetClusterByName("existing")
	if err != nil {
		t.Fatalf("GetClusterByName(existing) error = %v", err)
	}
	if !existing.IsDefault {
		t.Fatal("existing default cluster lost default flag")
	}
}

func TestImportClustersFromKubeconfigAppendsClustersWithDifferentAPIServer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupClusterHandlerTestDB(t)

	if err := model.AddCluster(&model.Cluster{
		Name:      "existing",
		Config:    model.SecretString(existingClusterKubeconfig),
		IsDefault: true,
		Enable:    true,
	}); err != nil {
		t.Fatalf("AddCluster() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/clusters/import",
		strings.NewReader(`{"config":"`+jsonEscape(t, additionalClusterKubeconfig)+`","inCluster":false}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	(&ClusterManager{}).ImportClustersFromKubeconfig(ctx)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}

	count, err := model.CountClusters()
	if err != nil {
		t.Fatalf("CountClusters() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("CountClusters() = %d, want 2", count)
	}

	imported, err := model.GetClusterByName("new-dev")
	if err != nil {
		t.Fatalf("GetClusterByName(new-dev) error = %v", err)
	}
	if imported.IsDefault {
		t.Fatal("imported cluster unexpectedly became default")
	}

	existing, err := model.GetClusterByName("existing")
	if err != nil {
		t.Fatalf("GetClusterByName(existing) error = %v", err)
	}
	if !existing.IsDefault {
		t.Fatal("existing default cluster lost default flag")
	}
}

func TestImportClustersFromKubeconfigAppendsSameNameClusterWithDifferentAPIServer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupClusterHandlerTestDB(t)

	if err := model.AddCluster(&model.Cluster{
		Name:      "existing",
		Config:    model.SecretString(existingClusterKubeconfig),
		IsDefault: true,
		Enable:    true,
	}); err != nil {
		t.Fatalf("AddCluster() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/clusters/import",
		strings.NewReader(`{"config":"`+jsonEscape(t, sameNameDifferentServerKubeconfig)+`","inCluster":false}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	(&ClusterManager{}).ImportClustersFromKubeconfig(ctx)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}

	count, err := model.CountClusters()
	if err != nil {
		t.Fatalf("CountClusters() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("CountClusters() = %d, want 2", count)
	}

	if _, err := model.GetClusterByName("existing"); err != nil {
		t.Fatalf("GetClusterByName(existing) error = %v", err)
	}
	var importedClusters []model.Cluster
	if err := model.DB.Where("name = ?", "existing").Find(&importedClusters).Error; err != nil {
		t.Fatalf("Find(imported clusters) error = %v", err)
	}
	if len(importedClusters) != 2 {
		t.Fatalf("len(importedClusters) = %d, want 2", len(importedClusters))
	}
	defaultCount := 0
	for _, cluster := range importedClusters {
		if cluster.IsDefault {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Fatalf("defaultCount = %d, want 1", defaultCount)
	}
}

func setupClusterHandlerTestDB(t *testing.T) {
	t.Helper()

	if err := model.DB.Exec("DELETE FROM clusters").Error; err != nil {
		t.Fatalf("cleanup clusters failed: %v", err)
	}
}

func jsonEscape(t *testing.T, input string) string {
	t.Helper()

	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return strings.Trim(string(encoded), `"`)
}

const additionalClusterKubeconfig = `apiVersion: v1
kind: Config
current-context: new-dev
clusters:
- name: new-dev
  cluster:
    server: https://example.invalid
users:
- name: new-dev
  user:
    token: test-token
contexts:
- name: new-dev
  context:
    cluster: new-dev
    user: new-dev
`

const existingClusterKubeconfig = `apiVersion: v1
kind: Config
current-context: existing
clusters:
- name: existing
  cluster:
    server: https://demo.example.com
users:
- name: existing
  user:
    token: existing-token
contexts:
- name: existing
  context:
    cluster: existing
    user: existing
`

const sameServerDifferentNameKubeconfig = `apiVersion: v1
kind: Config
current-context: same-server-new-name
clusters:
- name: same-server-new-name
  cluster:
    server: https://demo.example.com:443/
users:
- name: same-server-new-name
  user:
    token: same-server-token
contexts:
- name: same-server-new-name
  context:
    cluster: same-server-new-name
    user: same-server-new-name
`

const sameNameDifferentServerKubeconfig = `apiVersion: v1
kind: Config
current-context: existing
clusters:
- name: existing
  cluster:
    server: https://another.example.com
users:
- name: existing
  user:
    token: another-token
contexts:
- name: existing
  context:
    cluster: existing
    user: existing
`
