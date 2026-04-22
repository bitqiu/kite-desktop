import { describe, expect, it } from 'vitest'

import { clearClusterCookie, setClusterCookie } from './cluster-cookie'

describe('cluster cookie helpers', () => {
  it('writes the selected cluster id to cookies', () => {
    setClusterCookie('12')

    expect(document.cookie).toContain('x-cluster-id=12')
  })

  it('clears the cluster cookie', () => {
    setClusterCookie('12')
    clearClusterCookie()

    expect(document.cookie).not.toContain('x-cluster-id=')
  })
})
