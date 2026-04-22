const CLUSTER_ID_PARAM = 'x-cluster-id'
const LEGACY_CLUSTER_NAME_PARAM = 'x-cluster-name'

export function appendClusterNameParam(
  url: string,
  clusterID?: string | null
): string {
  const normalizedClusterID = clusterID?.trim()
  if (!normalizedClusterID) {
    return url
  }

  const [base, hash = ''] = url.split('#', 2)
  const separator = base.includes('?') ? '&' : '?'
  const nextUrl = `${base}${separator}${new URLSearchParams({
    [CLUSTER_ID_PARAM]: normalizedClusterID,
  }).toString()}`

  if (!hash) {
    return nextUrl
  }

  return `${nextUrl}#${hash}`
}

export function stripClusterNameHeader(
  headers: Record<string, string>
): string | undefined {
  const clusterName =
    headers['x-cluster-id'] ??
    headers['X-Cluster-ID'] ??
    headers['x-cluster-name'] ??
    headers['X-Cluster-Name'] ??
    headers[CLUSTER_ID_PARAM] ??
    headers[LEGACY_CLUSTER_NAME_PARAM]

  delete headers['x-cluster-id']
  delete headers['X-Cluster-ID']
  delete headers['x-cluster-name']
  delete headers['X-Cluster-Name']

  return clusterName?.trim() || undefined
}
