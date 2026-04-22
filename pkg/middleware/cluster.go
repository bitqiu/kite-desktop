package middleware

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/eryajf/kite-desktop/pkg/cluster"
	"github.com/gin-gonic/gin"
)

const (
	ClusterIDHeader   = "x-cluster-id"
	ClusterNameHeader = "x-cluster-name"
	ClusterIDKey      = "cluster-id"
	ClusterNameKey    = "cluster-name"
	K8sClientKey      = "k8s-client"
	PromClientKey     = "prom-client"
)

// ClusterMiddleware extracts cluster name from header and injects clients into context
func ClusterMiddleware(cm *cluster.ClusterManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID := readClusterIDRequestValue(c)
		if clusterID != 0 {
			cluster, err := cm.GetClientSetByID(clusterID)
			if err != nil {
				c.JSON(404, gin.H{"error": err.Error()})
				c.Abort()
				return
			}
			c.Set("cluster", cluster)
			c.Set(ClusterIDKey, cluster.ID)
			c.Set(ClusterNameKey, cluster.Name)
			c.Next()
			return
		}

		clusterName := strings.TrimSpace(c.GetHeader(ClusterNameHeader))
		if clusterName == "" {
			if v, ok := c.GetQuery(ClusterNameHeader); ok {
				clusterName = strings.TrimSpace(v)
			}
			if clusterName == "" {
				clusterName = ReadClusterNameCookie(c)
			}
		}
		cluster, err := cm.GetClientSet(clusterName)
		if err != nil {
			c.JSON(404, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		c.Set("cluster", cluster)
		c.Set(ClusterIDKey, cluster.ID)
		c.Set(ClusterNameKey, cluster.Name)
		c.Next()
	}
}

func readClusterIDRequestValue(c *gin.Context) uint {
	if raw := strings.TrimSpace(c.GetHeader(ClusterIDHeader)); raw != "" {
		if value, err := strconv.ParseUint(raw, 10, 32); err == nil {
			return uint(value)
		}
	}
	if raw, ok := c.GetQuery(ClusterIDHeader); ok {
		if value, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 32); err == nil {
			return uint(value)
		}
	}
	return ReadClusterIDCookie(c)
}

func ReadClusterIDCookie(c *gin.Context) uint {
	rawValue, err := c.Cookie(ClusterIDHeader)
	if err != nil {
		return 0
	}
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return 0
	}
	decoded, err := url.QueryUnescape(trimmed)
	if err != nil {
		decoded = trimmed
	}
	value, err := strconv.ParseUint(strings.TrimSpace(decoded), 10, 32)
	if err != nil {
		return 0
	}
	return uint(value)
}

func ReadClusterNameCookie(c *gin.Context) string {
	rawValue, err := c.Cookie(ClusterNameHeader)
	if err != nil {
		return ""
	}
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return ""
	}
	decoded, err := url.QueryUnescape(trimmed)
	if err != nil {
		return trimmed
	}
	return strings.TrimSpace(decoded)
}
