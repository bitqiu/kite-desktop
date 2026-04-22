/* eslint-disable react-refresh/only-export-components */
import React, { createContext, useEffect, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'

import { trackEvent } from '@/lib/analytics'
import { getCurrentAnalyticsPageKey } from '@/lib/analytics-route'
import { clearClusterCookie, setClusterCookie } from '@/lib/cluster-cookie'
import { Cluster } from '@/types/api'
import { clusterQueryKey } from '@/lib/cluster-query'
import { withSubPath } from '@/lib/subpath'

const recentClustersStorageKey = 'recent-clusters'

interface ClusterContextType {
  clusters: Cluster[]
  currentCluster: string | null
  currentClusterId: string | null
  currentClusterData: Cluster | null
  setCurrentCluster: (clusterId: string) => void
  isLoading: boolean
  isSwitching?: boolean
  error: Error | null
}

export const ClusterContext = createContext<ClusterContextType | undefined>(
  undefined
)

export const ClusterProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const [currentClusterId, setCurrentClusterIdState] = useState<string | null>(
    localStorage.getItem('current-cluster-id')
  )
  const queryClient = useQueryClient()
  const [isSwitching, setIsSwitching] = useState(false)

  const saveRecentCluster = (clusterId: string) => {
    const recentClusters = JSON.parse(
      localStorage.getItem(recentClustersStorageKey) || '[]'
    ) as string[]
    const nextRecentClusters = [
      clusterId,
      ...recentClusters.filter((id) => id !== clusterId),
    ].slice(0, 8)
    localStorage.setItem(
      recentClustersStorageKey,
      JSON.stringify(nextRecentClusters)
    )
  }

  useEffect(() => {
    if (currentClusterId) {
      setClusterCookie(currentClusterId)
      return
    }
    clearClusterCookie()
  }, [currentClusterId])

  // Fetch clusters from API (this request shouldn't need cluster header)
  const {
    data: clusters = [],
    isLoading,
    error,
  } = useQuery<Cluster[]>({
    queryKey: clusterQueryKey,
    queryFn: async () => {
      const response = await fetch(withSubPath('/api/v1/clusters'), {
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
        },
      })

      if (response.status === 403) {
        const errorData = await response.json().catch(() => ({}))
        const redirectUrl = response.headers.get('Location')
        if (redirectUrl) {
          window.location.href = redirectUrl
        }
        throw new Error(`${errorData.error || response.status}`)
      }

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}))
        throw new Error(`${errorData.error || response.status}`)
      }

      return response.json()
    },
    staleTime: 5 * 60 * 1000, // 5 minutes
  })

  const currentClusterData =
    clusters.find((cluster) => String(cluster.id) === currentClusterId) ?? null
  const currentCluster = currentClusterData?.name ?? null

  // Set default cluster if none is selected
  useEffect(() => {
    if (clusters.length > 0 && !currentClusterId) {
      const defaultCluster = clusters.find((c) => c.isDefault)
      if (defaultCluster) {
        const defaultClusterId = String(defaultCluster.id)
        setCurrentClusterIdState(defaultClusterId)
        setClusterCookie(defaultClusterId)
        localStorage.setItem('current-cluster-id', defaultClusterId)
        localStorage.setItem('current-cluster', defaultCluster.name)
      } else {
        // If no default cluster, use the first one
        const firstClusterId = String(clusters[0].id)
        setCurrentClusterIdState(firstClusterId)
        localStorage.setItem('current-cluster-id', firstClusterId)
        localStorage.setItem('current-cluster', clusters[0].name)
        setClusterCookie(firstClusterId)
      }
    }
    if (clusters.length === 0 && currentClusterId) {
      setCurrentClusterIdState(null)
      localStorage.removeItem('current-cluster-id')
      localStorage.removeItem('current-cluster')
      clearClusterCookie()
    }
    if (
      currentClusterId &&
      clusters.length > 0 &&
      !clusters.some((c) => String(c.id) === currentClusterId)
    ) {
      // If current cluster is not in the list, reset it
      setCurrentClusterIdState(null)
      localStorage.removeItem('current-cluster-id')
      localStorage.removeItem('current-cluster')
      clearClusterCookie()
    }
  }, [clusters, currentClusterId])

  const setCurrentCluster = (clusterId: string) => {
    const nextCluster = clusters.find(
      (cluster) => String(cluster.id) === clusterId
    )
    if (!nextCluster) {
      return
    }

    if (clusterId !== currentClusterId && !isSwitching) {
      try {
        setIsSwitching(true)
        setCurrentClusterIdState(clusterId)
        localStorage.setItem('current-cluster-id', clusterId)
        localStorage.setItem('current-cluster', nextCluster.name)
        saveRecentCluster(clusterId)
        setClusterCookie(clusterId)
        trackEvent('cluster_switch', {
          runtime: 'desktop',
          page: getCurrentAnalyticsPageKey(),
        })
        setTimeout(async () => {
          await queryClient.invalidateQueries({
            predicate: (query) => {
              const key = query.queryKey[0] as string
              return !['user', 'auth', 'clusters'].includes(key)
            },
          })
          setIsSwitching(false)
          toast.success(`Switched to cluster: ${nextCluster.name}`, {
            id: 'cluster-switch',
          })
        }, 300)
      } catch (error) {
        console.error('Failed to switch cluster:', error)
        setIsSwitching(false)
        toast.error('Failed to switch cluster', {
          id: 'cluster-switch',
        })
      }
    }
  }

  const value: ClusterContextType = {
    clusters,
    currentCluster,
    currentClusterId,
    currentClusterData,
    setCurrentCluster,
    isLoading,
    isSwitching,
    error: error as Error | null,
  }

  return (
    <ClusterContext.Provider value={value}>{children}</ClusterContext.Provider>
  )
}
