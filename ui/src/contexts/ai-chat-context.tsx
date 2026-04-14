import {
  createContext,
  ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react'
import { useLocation, useParams } from 'react-router-dom'

import { useAIStatus } from '@/lib/api'
import { AI_CHAT_TOGGLE_EVENT } from '@/components/ai-chat/constants'

interface PageContext {
  page: string
  namespace: string
  resourceName: string
  resourceKind: string
}

interface AIChatContextType {
  isOpen: boolean
  isAvailable: boolean
  openChat: () => void
  closeChat: () => void
  toggleChat: () => void
  pageContext: PageContext
}

const AIChatContext = createContext<AIChatContextType | undefined>(undefined)

const singularResourceMap: Record<string, string> = {
  pods: 'pod',
  services: 'service',
  configmaps: 'configmap',
  secrets: 'secret',
  namespaces: 'namespace',
  nodes: 'node',
  persistentvolumeclaims: 'persistentvolumeclaim',
  persistentvolumes: 'persistentvolume',
  serviceaccounts: 'serviceaccount',
  deployments: 'deployment',
  statefulsets: 'statefulset',
  daemonsets: 'daemonset',
  replicasets: 'replicaset',
  jobs: 'job',
  cronjobs: 'cronjob',
  ingresses: 'ingress',
  networkpolicies: 'networkpolicy',
  storageclasses: 'storageclass',
  events: 'event',
}

function toSingularResource(resource: string) {
  if (!resource) return resource
  const normalized = resource.toLowerCase()
  if (singularResourceMap[normalized]) {
    return singularResourceMap[normalized]
  }
  if (normalized.endsWith('s')) {
    return normalized.slice(0, -1)
  }
  return normalized
}

export function AIChatProvider({ children }: { children: ReactNode }) {
  const [isOpen, setIsOpen] = useState(false)
  const location = useLocation()
  const params = useParams()
  const { data: { enabled: aiEnabled } = { enabled: false } } = useAIStatus()

  const isAvailable =
    aiEnabled &&
    !/^\/settings\/?$/.test(location.pathname) &&
    location.pathname !== '/ai-chat-box'

  const openChat = useCallback(() => {
    if (!isAvailable) {
      return
    }
    setIsOpen(true)
  }, [isAvailable])

  const closeChat = useCallback(() => {
    setIsOpen(false)
  }, [])

  const toggleChat = useCallback(() => {
    if (!isAvailable) {
      return
    }
    setIsOpen((prev) => !prev)
  }, [isAvailable])

  useEffect(() => {
    if (isAvailable) {
      return
    }
    setIsOpen(false)
  }, [isAvailable])

  useEffect(() => {
    if (!isAvailable) {
      return
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented) {
        return
      }

      if (
        (event.metaKey || event.ctrlKey) &&
        event.shiftKey &&
        !event.altKey &&
        event.key.toLowerCase() === 'a'
      ) {
        event.preventDefault()
        setIsOpen((prev) => !prev)
      }
    }

    const handleToggle = () => {
      setIsOpen((prev) => !prev)
    }

    document.addEventListener('keydown', handleKeyDown)
    window.addEventListener(AI_CHAT_TOGGLE_EVENT, handleToggle)

    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      window.removeEventListener(AI_CHAT_TOGGLE_EVENT, handleToggle)
    }
  }, [isAvailable])

  const pageContext = useMemo<PageContext>(() => {
    const path = location.pathname
    const searchParams = new URLSearchParams(location.search)

    if (path === '/ai-chat-box') {
      return {
        page: searchParams.get('page') || 'overview',
        namespace: searchParams.get('namespace') || '',
        resourceName: searchParams.get('resourceName') || '',
        resourceKind: toSingularResource(
          searchParams.get('resourceKind') || ''
        ),
      }
    }

    const resource = params.resource || ''
    const name = params.name || ''
    const namespace = params.namespace || ''
    const normalizedKind = toSingularResource(resource)

    let page = 'overview'
    if (path === '/' || path === '/dashboard') {
      page = 'overview'
    } else if (name) {
      page = `${normalizedKind}-detail`
    } else if (resource) {
      page = `${resource}-list`
    }

    return {
      page,
      namespace,
      resourceName: name,
      resourceKind: normalizedKind,
    }
  }, [
    location.pathname,
    location.search,
    params.resource,
    params.name,
    params.namespace,
  ])

  return (
    <AIChatContext.Provider
      value={{
        isOpen,
        isAvailable,
        openChat,
        closeChat,
        toggleChat,
        pageContext,
      }}
    >
      {children}
    </AIChatContext.Provider>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAIChatContext() {
  const context = useContext(AIChatContext)
  if (context === undefined) {
    throw new Error('useAIChatContext must be used within an AIChatProvider')
  }
  return context
}
