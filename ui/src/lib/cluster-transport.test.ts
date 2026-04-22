import { describe, expect, it } from 'vitest'

import {
  appendClusterNameParam,
  stripClusterNameHeader,
} from './cluster-transport'

describe('appendClusterNameParam', () => {
  it('appends a cluster id as a query parameter', () => {
    expect(appendClusterNameParam('/api/v1/nodes', '12')).toBe(
      '/api/v1/nodes?x-cluster-id=12'
    )
  })

  it('preserves existing query parameters and hashes', () => {
    expect(appendClusterNameParam('/api/v1/nodes?limit=20#table', '7')).toBe(
      '/api/v1/nodes?limit=20&x-cluster-id=7#table'
    )
  })
})

describe('stripClusterNameHeader', () => {
  it('removes cluster headers and returns the trimmed value', () => {
    const headers = {
      'Content-Type': 'application/json',
      'x-cluster-id': '  12  ',
    }

    expect(stripClusterNameHeader(headers)).toBe('12')
    expect(headers).toEqual({
      'Content-Type': 'application/json',
    })
  })
})
