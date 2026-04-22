const CLUSTER_ID_COOKIE_NAME = 'x-cluster-id'
const LEGACY_CLUSTER_NAME_COOKIE_NAME = 'x-cluster-name'

export function setClusterCookie(clusterId: string) {
  document.cookie = `${CLUSTER_ID_COOKIE_NAME}=${encodeURIComponent(clusterId)}; path=/`
}

export function clearClusterCookie() {
  document.cookie =
    `${CLUSTER_ID_COOKIE_NAME}=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT`
  document.cookie =
    `${LEGACY_CLUSTER_NAME_COOKIE_NAME}=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT`
}
