package cluster

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/eryajf/kite-desktop/pkg/kube"
	"github.com/eryajf/kite-desktop/pkg/model"
	"github.com/eryajf/kite-desktop/pkg/prometheus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"
)

type ClientSet struct {
	ID         uint
	Name       string
	APIServer  string
	Version    string // Kubernetes version
	K8sClient  *kube.K8sClient
	PromClient *prometheus.Client

	DiscoveredPrometheusURL string
	config                  string
	prometheusURL           string
}

type ClusterManager struct {
	clusters         map[uint]*ClientSet
	errors           map[uint]string
	defaultClusterID uint
}

func createClientSetInCluster(name, prometheusURL string) (*ClientSet, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, formatClusterConnectionError(err)
	}

	return newClientSet(name, config, prometheusURL)
}

func createClientSetFromConfig(name, content, prometheusURL string) (*ClientSet, error) {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(content))
	if err != nil {
		klog.Warningf("Failed to create REST config for cluster %s: %v", name, err)
		return nil, formatClusterConnectionError(err)
	}
	cs, err := newClientSet(name, restConfig, prometheusURL)
	if err != nil {
		return nil, err
	}
	cs.config = content

	return cs, nil
}

func newClientSet(name string, k8sConfig *rest.Config, prometheusURL string) (*ClientSet, error) {
	cs := &ClientSet{
		Name:          name,
		prometheusURL: prometheusURL,
	}
	var err error
	cs.K8sClient, err = kube.NewClient(k8sConfig)
	if err != nil {
		klog.Warningf("Failed to create k8s client for cluster %s: %v", name, err)
		return nil, formatClusterConnectionError(err)
	}
	if prometheusURL == "" {
		prometheusURL = discoveryPrometheusURL(cs.K8sClient)
		if prometheusURL != "" {
			cs.DiscoveredPrometheusURL = prometheusURL
			klog.Infof("Discovered Prometheus URL for cluster %s: %s", name, cs.DiscoveredPrometheusURL)
		}
	}
	if prometheusURL != "" {
		var rt = http.DefaultTransport
		var err error
		if isClusterLocalURL(prometheusURL) {
			rt, err = createK8sProxyTransport(k8sConfig, prometheusURL)
			if err != nil {
				klog.Warningf("Failed to create k8s proxy transport for cluster %s: %v, using direct connection", name, err)
			} else {
				klog.Infof("Using k8s API proxy for Prometheus in cluster %s", name)
			}
		}
		cs.PromClient, err = prometheus.NewClientWithRoundTripper(prometheusURL, rt)
		if err != nil {
			klog.Warningf("Failed to create Prometheus client for cluster %s, some features may not work as expected, err: %v", name, err)
		}
	}
	v, err := cs.K8sClient.ClientSet.Discovery().ServerVersion()
	if err != nil {
		klog.Warningf("Failed to get server version for cluster %s: %v", name, err)
	} else {
		cs.Version = v.String()
	}
	klog.Infof("Loaded K8s client for cluster: %s, version: %s", name, cs.Version)
	return cs, nil
}

func isClusterLocalURL(urlStr string) bool {
	return strings.Contains(urlStr, ".svc.cluster.local") || strings.Contains(urlStr, ".svc:")
}

func createK8sProxyTransport(k8sConfig *rest.Config, prometheusURL string) (*k8sProxyTransport, error) {
	parsedURL, err := url.Parse(prometheusURL)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(parsedURL.Host, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid cluster local URL format")
	}
	svcName := parts[0]
	namespace := parts[1]

	transport, err := rest.TransportFor(k8sConfig)
	if err != nil {
		return nil, err
	}

	transportWrapper := &k8sProxyTransport{
		transport:    transport,
		apiServerURL: k8sConfig.Host,
		namespace:    namespace,
		svcName:      svcName,
		scheme:       parsedURL.Scheme,
	}
	transportWrapper.port = parsedURL.Port()
	if transportWrapper.port == "" {
		if parsedURL.Scheme == "https" {
			transportWrapper.port = "443"
		} else {
			transportWrapper.port = "80"
		}
	}

	return transportWrapper, nil
}

type k8sProxyTransport struct {
	transport    http.RoundTripper
	apiServerURL string
	namespace    string
	svcName      string
	scheme       string
	port         string
}

func (t *k8sProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	proxyURL, err := url.Parse(t.apiServerURL)
	if err != nil {
		return nil, err
	}
	req.URL.Scheme = proxyURL.Scheme
	req.URL.Host = proxyURL.Host

	servicePath := fmt.Sprintf("/api/v1/namespaces/%s/services/%s:%s/proxy", t.namespace, t.svcName, t.port)
	req.URL.Path = servicePath + req.URL.Path

	return t.transport.RoundTrip(req)
}

func (cm *ClusterManager) GetClientSet(clusterName string) (*ClientSet, error) {
	if len(cm.clusters) == 0 {
		return nil, fmt.Errorf("no clusters available")
	}
	if clusterName == "" {
		if cm.defaultClusterID == 0 {
			// If no default context is set, return the first available cluster
			for _, cs := range cm.clusters {
				return cs, nil
			}
		}
		return cm.GetClientSetByID(cm.defaultClusterID)
	}

	if clusterID, err := strconv.ParseUint(clusterName, 10, 32); err == nil {
		return cm.GetClientSetByID(uint(clusterID))
	}

	for _, cluster := range cm.clusters {
		if cluster.Name == clusterName {
			return cluster, nil
		}
	}

	return nil, fmt.Errorf("cluster not found: %s", clusterName)
}

func (cm *ClusterManager) GetClientSetByID(clusterID uint) (*ClientSet, error) {
	if len(cm.clusters) == 0 {
		return nil, fmt.Errorf("no clusters available")
	}
	if clusterID == 0 {
		if cm.defaultClusterID == 0 {
			for _, cs := range cm.clusters {
				return cs, nil
			}
			return nil, fmt.Errorf("no clusters available")
		}
		clusterID = cm.defaultClusterID
	}
	if cluster, ok := cm.clusters[clusterID]; ok {
		return cluster, nil
	}
	return nil, fmt.Errorf("cluster not found: %d", clusterID)
}

func ImportClustersFromKubeconfig(kubeconfig *clientcmdapi.Config) int64 {
	if len(kubeconfig.Contexts) == 0 {
		return 0
	}

	hasDefaultCluster, err := model.HasDefaultCluster()
	if err != nil {
		klog.Warningf("failed to check existing default cluster: %v", err)
		hasDefaultCluster = false
	}

	existingAPIServers := make(map[string]struct{})
	clusters, err := model.ListClusters()
	if err != nil {
		klog.Warningf("failed to list clusters for api server dedupe: %v", err)
	} else {
		for _, existing := range clusters {
			apiServer, err := getNormalizedAPIServerAddress(string(existing.Config))
			if err != nil || apiServer == "" {
				continue
			}
			existingAPIServers[apiServer] = struct{}{}
		}
	}

	importedCount := 0
	for contextName, context := range kubeconfig.Contexts {
		clusterConfig, ok := kubeconfig.Clusters[context.Cluster]
		if !ok || clusterConfig == nil {
			continue
		}

		apiServer, err := normalizeAPIServerAddress(clusterConfig.Server)
		if err != nil {
			klog.Warningf("failed to normalise api server for cluster %s: %v", contextName, err)
			continue
		}
		if _, exists := existingAPIServers[apiServer]; exists {
			klog.Infof("Skipped importing cluster %s because api server %s already exists", contextName, apiServer)
			continue
		}

		config := clientcmdapi.NewConfig()
		config.Contexts = map[string]*clientcmdapi.Context{
			contextName: context,
		}
		config.CurrentContext = contextName
		config.Clusters = map[string]*clientcmdapi.Cluster{
			context.Cluster: kubeconfig.Clusters[context.Cluster],
		}
		config.AuthInfos = map[string]*clientcmdapi.AuthInfo{
			context.AuthInfo: kubeconfig.AuthInfos[context.AuthInfo],
		}
		configStr, err := clientcmd.Write(*config)
		if err != nil {
			continue
		}
		cluster := model.Cluster{
			Name:      contextName,
			Config:    model.SecretString(configStr),
			IsDefault: !hasDefaultCluster && contextName == kubeconfig.CurrentContext,
		}
		if err := model.AddCluster(&cluster); err != nil {
			continue
		}
		if cluster.IsDefault {
			hasDefaultCluster = true
		}
		existingAPIServers[apiServer] = struct{}{}
		importedCount++
		klog.Infof("Imported cluster success: %s", cluster.Name)
	}
	return int64(importedCount)
}

func getNormalizedAPIServerAddress(config string) (string, error) {
	if strings.TrimSpace(config) == "" {
		return "", nil
	}

	kubeconfig, err := clientcmd.Load([]byte(config))
	if err != nil {
		return "", err
	}
	if kubeconfig.CurrentContext == "" {
		return "", nil
	}

	ctx, ok := kubeconfig.Contexts[kubeconfig.CurrentContext]
	if !ok || ctx == nil {
		return "", nil
	}
	clusterConfig, ok := kubeconfig.Clusters[ctx.Cluster]
	if !ok || clusterConfig == nil {
		return "", nil
	}

	return normalizeAPIServerAddress(clusterConfig.Server)
}

func normalizeAPIServerAddress(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", nil
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid api server url: %s", rawURL)
	}

	scheme := strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	switch {
	case port != "":
	case scheme == "https":
		port = "443"
	case scheme == "http":
		port = "80"
	}

	path := strings.TrimRight(parsed.EscapedPath(), "/")
	if path == "/" {
		path = ""
	}

	if port != "" {
		return fmt.Sprintf("%s://%s:%s%s", scheme, host, port, path), nil
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, path), nil
}

var (
	syncNow = make(chan chan error, 1)
)

func requestClusterSync(wait bool) error {
	var done chan error
	if wait {
		done = make(chan error, 1)
	}

	syncNow <- done
	if done == nil {
		return nil
	}

	return <-done
}

func syncClusters(cm *ClusterManager) error {
	clusters, err := model.ListClusters()
	if err != nil {
		klog.Warningf("list cluster err: %v", err)
		time.Sleep(5 * time.Second)
		return err
	}
	dbClusterMap := make(map[uint]struct{})
	cm.defaultClusterID = 0
	type buildResult struct {
		cluster   *model.Cluster
		clientSet *ClientSet
		err       error
	}
	buildQueue := make([]*model.Cluster, 0)
	for _, cluster := range clusters {
		dbClusterMap[cluster.ID] = struct{}{}
		if cluster.IsDefault {
			cm.defaultClusterID = cluster.ID
		}
		current, currentExist := cm.clusters[cluster.ID]
		if shouldUpdateCluster(current, cluster) {
			if currentExist {
				delete(cm.clusters, cluster.ID)
				current.K8sClient.Stop(cluster.Name)
			}
			if cluster.Enable {
				buildQueue = append(buildQueue, cluster)
			} else {
				delete(cm.errors, cluster.ID)
			}
		}
	}
	results := make(chan buildResult, len(buildQueue))
	var wg sync.WaitGroup
	for _, cluster := range buildQueue {
		wg.Add(1)
		go func(cluster *model.Cluster) {
			defer wg.Done()
			clientSet, err := buildClientSet(cluster)
			results <- buildResult{
				cluster:   cluster,
				clientSet: clientSet,
				err:       err,
			}
		}(cluster)
	}
	wg.Wait()
	close(results)
	for result := range results {
		if result.err != nil {
			klog.Errorf("Failed to build k8s client for cluster %s, in cluster: %t, err: %v", result.cluster.Name, result.cluster.InCluster, result.err)
			cm.errors[result.cluster.ID] = result.err.Error()
			continue
		}
		delete(cm.errors, result.cluster.ID)
		cm.clusters[result.cluster.ID] = result.clientSet
	}
	for id, clientSet := range cm.clusters {
		if _, ok := dbClusterMap[id]; !ok {
			delete(cm.clusters, id)
			clientSet.K8sClient.Stop(clientSet.Name)
		}
	}
	for id := range cm.errors {
		if _, ok := dbClusterMap[id]; !ok {
			delete(cm.errors, id)
		}
	}

	return nil
}

// shouldUpdateCluster decides whether the cached ClientSet needs to be updated
// based on the desired state from the database.
func shouldUpdateCluster(cs *ClientSet, cluster *model.Cluster) bool {
	// enable/disable toggle
	if (cs == nil && cluster.Enable) || (cs != nil && !cluster.Enable) {
		klog.Infof("Cluster %s status changed, updating, enabled -> %v", cluster.Name, cluster.Enable)
		return true
	}
	if cs == nil && !cluster.Enable {
		return false
	}

	if cs == nil || cs.K8sClient == nil || cs.K8sClient.ClientSet == nil {
		return true
	}

	// kubeconfig change
	if cs.config != string(cluster.Config) {
		klog.Infof("Kubeconfig changed for cluster %s, updating", cluster.Name)
		return true
	}

	// prometheus URL change
	if cs.prometheusURL != cluster.PrometheusURL {
		klog.Infof("Prometheus URL changed for cluster %s, updating", cluster.Name)
		return true
	}

	// k8s version change
	// TODO: Replace direct ClientSet.Discovery() call with a small DiscoveryInterface.
	// current code depends on *kubernetes.Clientset, which is hard to mock in tests.
	version, err := cs.K8sClient.ClientSet.Discovery().ServerVersion()
	if err != nil {
		klog.Warningf("Failed to get server version for cluster %s: %v", cluster.Name, err)
	} else if version.String() != cs.Version {
		klog.Infof("Server version changed for cluster %s, updating, old: %s, new: %s", cluster.Name, cs.Version, version.String())
		return true
	}

	return false
}

func buildClientSet(cluster *model.Cluster) (*ClientSet, error) {
	if cluster.InCluster {
		clientSet, err := createClientSetInCluster(cluster.Name, cluster.PrometheusURL)
		if err != nil {
			return nil, err
		}
		clientSet.ID = cluster.ID
		return clientSet, nil
	}
	clientSet, err := createClientSetFromConfig(cluster.Name, string(cluster.Config), cluster.PrometheusURL)
	if err != nil {
		return nil, err
	}
	clientSet.ID = cluster.ID
	if apiServer, apiErr := getNormalizedAPIServerAddress(string(cluster.Config)); apiErr == nil {
		clientSet.APIServer = apiServer
	}
	return clientSet, nil
}

func NewClusterManager() (*ClusterManager, error) {
	cm := new(ClusterManager)
	cm.clusters = make(map[uint]*ClientSet)
	cm.errors = make(map[uint]string)
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := syncClusters(cm); err != nil {
					klog.Warningf("Failed to sync clusters: %v", err)
				}
			case done := <-syncNow:
				err := syncClusters(cm)
				if err != nil {
					klog.Warningf("Failed to sync clusters: %v", err)
				}
				if done != nil {
					done <- err
					close(done)
				}
			}
		}
	}()

	if err := syncClusters(cm); err != nil {
		klog.Warningf("Failed to sync clusters: %v", err)
	}
	return cm, nil
}
