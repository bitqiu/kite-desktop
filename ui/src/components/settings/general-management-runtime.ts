export function shouldReloadForAnalyticsChange(
  previousEnableAnalytics: boolean | undefined,
  nextEnableAnalytics: boolean
) {
  return (
    typeof previousEnableAnalytics === 'boolean' &&
    previousEnableAnalytics !== nextEnableAnalytics
  )
}

export const browserRuntime = {
  reloadWindow() {
    window.location.reload()
  },
}
