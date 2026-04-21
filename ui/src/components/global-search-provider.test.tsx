import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

const { trackDesktopEvent } = vi.hoisted(() => ({
  trackDesktopEvent: vi.fn(),
}))

vi.mock('@/lib/analytics', () => ({
  trackDesktopEvent,
}))

import { GlobalSearchProvider, useGlobalSearch } from './global-search-provider'

function createStorage() {
  let store: Record<string, string> = {}

  return {
    getItem(key: string) {
      return Object.prototype.hasOwnProperty.call(store, key)
        ? store[key]
        : null
    },
    setItem(key: string, value: string) {
      store[key] = value
    },
    removeItem(key: string) {
      delete store[key]
    },
    clear() {
      store = {}
    },
  }
}

vi.stubGlobal('localStorage', createStorage())
vi.stubGlobal('sessionStorage', createStorage())

function GlobalSearchConsumer() {
  const { isOpen, mode, openSearch, closeSearch } = useGlobalSearch()

  return (
    <div>
      <span data-testid="state">{isOpen ? 'open' : 'closed'}</span>
      <span data-testid="mode">{mode}</span>
      <button type="button" onClick={openSearch}>
        open
      </button>
      <button type="button" onClick={() => openSearch('cluster')}>
        cluster
      </button>
      <button type="button" onClick={closeSearch}>
        close
      </button>
    </div>
  )
}

describe('GlobalSearchProvider', () => {
  it('opens global search from the keyboard and closes on escape', async () => {
    trackDesktopEvent.mockReset()
    render(
      <GlobalSearchProvider>
        <GlobalSearchConsumer />
      </GlobalSearchProvider>
    )

    expect(screen.getByTestId('state')).toHaveTextContent('closed')
    expect(screen.getByTestId('mode')).toHaveTextContent('all')

    fireEvent.keyDown(document, { key: 'k', ctrlKey: true })

    await waitFor(() => {
      expect(screen.getByTestId('state')).toHaveTextContent('open')
      expect(screen.getByTestId('mode')).toHaveTextContent('all')
    })
    expect(trackDesktopEvent).toHaveBeenNthCalledWith(1, 'global_search_open', {
      mode: 'all',
      entry: 'shortcut',
    })

    fireEvent.keyDown(document, { key: 'k', ctrlKey: true, shiftKey: true })

    await waitFor(() => {
      expect(screen.getByTestId('state')).toHaveTextContent('open')
      expect(screen.getByTestId('mode')).toHaveTextContent('cluster')
    })
    expect(trackDesktopEvent).toHaveBeenNthCalledWith(2, 'global_search_open', {
      mode: 'cluster',
      entry: 'shortcut',
    })

    fireEvent.keyDown(document, { key: 'Escape' })

    await waitFor(() => {
      expect(screen.getByTestId('state')).toHaveTextContent('closed')
      expect(screen.getByTestId('mode')).toHaveTextContent('all')
    })
  })
})
