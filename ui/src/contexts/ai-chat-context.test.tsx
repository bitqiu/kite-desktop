import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { AI_CHAT_TOGGLE_EVENT } from '@/components/ai-chat/constants'

import { AIChatProvider, useAIChatContext } from './ai-chat-context'

const { useAIStatus } = vi.hoisted(() => ({
  useAIStatus: vi.fn(),
}))

vi.mock('@/lib/api', () => ({
  useAIStatus,
}))

function AIChatStateProbe() {
  const { isAvailable, isOpen } = useAIChatContext()

  return (
    <div>
      <span data-testid="available">{String(isAvailable)}</span>
      <span data-testid="state">{isOpen ? 'open' : 'closed'}</span>
    </div>
  )
}

function renderProvider(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route
          path="*"
          element={
            <AIChatProvider>
              <AIChatStateProbe />
            </AIChatProvider>
          }
        />
      </Routes>
    </MemoryRouter>
  )
}

describe('AIChatProvider', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useAIStatus.mockReturnValue({
      data: { enabled: true },
    })
  })

  it('toggles AI chat from the keyboard shortcut and desktop event when enabled', async () => {
    renderProvider('/pods')

    expect(screen.getByTestId('available')).toHaveTextContent('true')
    expect(screen.getByTestId('state')).toHaveTextContent('closed')

    fireEvent.keyDown(document, { key: 'A', ctrlKey: true, shiftKey: true })

    await waitFor(() => {
      expect(screen.getByTestId('state')).toHaveTextContent('open')
    })

    fireEvent.keyDown(document, { key: 'A', ctrlKey: true, shiftKey: true })

    await waitFor(() => {
      expect(screen.getByTestId('state')).toHaveTextContent('closed')
    })

    fireEvent(window, new Event(AI_CHAT_TOGGLE_EVENT))

    await waitFor(() => {
      expect(screen.getByTestId('state')).toHaveTextContent('open')
    })

    fireEvent(window, new Event(AI_CHAT_TOGGLE_EVENT))

    await waitFor(() => {
      expect(screen.getByTestId('state')).toHaveTextContent('closed')
    })
  })

  it('does not expose the shortcut on unavailable pages or when AI is disabled', async () => {
    useAIStatus.mockReturnValueOnce({
      data: { enabled: false },
    })
    const { unmount } = renderProvider('/pods')

    expect(screen.getByTestId('available')).toHaveTextContent('false')
    fireEvent.keyDown(document, { key: 'A', ctrlKey: true, shiftKey: true })

    await waitFor(() => {
      expect(screen.getByTestId('state')).toHaveTextContent('closed')
    })

    unmount()

    renderProvider('/settings')

    expect(screen.getByTestId('available')).toHaveTextContent('false')
    fireEvent(window, new Event(AI_CHAT_TOGGLE_EVENT))

    await waitFor(() => {
      expect(screen.getByTestId('state')).toHaveTextContent('closed')
    })
  })
})
