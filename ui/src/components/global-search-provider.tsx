/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  ReactNode,
  useCallback,
  useContext,
  useEffect,
  useState,
} from 'react'

export type GlobalSearchMode = 'all' | 'cluster'

interface GlobalSearchContextType {
  isOpen: boolean
  mode: GlobalSearchMode
  openSearch: (mode?: GlobalSearchMode) => void
  closeSearch: () => void
  toggleSearch: (mode?: GlobalSearchMode) => void
}

const GlobalSearchContext = createContext<GlobalSearchContextType | undefined>(
  undefined
)

export function useGlobalSearch() {
  const context = useContext(GlobalSearchContext)
  if (context === undefined) {
    throw new Error(
      'useGlobalSearch must be used within a GlobalSearchProvider'
    )
  }
  return context
}

interface GlobalSearchProviderProps {
  children: ReactNode
}

export function GlobalSearchProvider({ children }: GlobalSearchProviderProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [mode, setMode] = useState<GlobalSearchMode>('all')

  const openSearch = useCallback((nextMode: GlobalSearchMode = 'all') => {
    setMode(nextMode)
    setIsOpen(true)
  }, [])

  const closeSearch = useCallback(() => {
    setIsOpen(false)
    setMode('all')
  }, [])

  const toggleSearch = useCallback((nextMode: GlobalSearchMode = 'all') => {
    setMode(nextMode)
    setIsOpen((prev) => !prev)
  }, [])

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Command+Shift+K or Ctrl+Shift+K to open cluster switcher
      if ((e.metaKey || e.ctrlKey) && e.shiftKey && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        openSearch('cluster')
        return
      }

      // Command+K or Ctrl+K to open search
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        openSearch('all')
        return
      }

      // Escape to close search
      if (e.key === 'Escape' && isOpen) {
        closeSearch()
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [closeSearch, isOpen, openSearch])

  const value = {
    isOpen,
    mode,
    openSearch,
    closeSearch,
    toggleSearch,
  }

  return (
    <GlobalSearchContext.Provider value={value}>
      {children}
    </GlobalSearchContext.Provider>
  )
}
