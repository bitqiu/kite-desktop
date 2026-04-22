import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from './api-client'

describe('apiClient cluster transport', () => {
  const fetchMock = vi.fn()

  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
    apiClient.setClusterProvider(() => '12')
  })

  afterEach(() => {
    apiClient.setClusterProvider(() => null)
  })

  it('moves the cluster id from headers to the query string', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({
        'content-type': 'application/json',
      }),
      json: async () => ({ ok: true }),
    })

    await apiClient.get('/nodes')

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/nodes?x-cluster-id=12',
      expect.objectContaining({
        headers: {
          'Content-Type': 'application/json',
        },
      })
    )
  })
})
