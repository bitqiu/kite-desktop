import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flexRender, getCoreRowModel, useReactTable } from '@tanstack/react-table'

const {
  useClusterList,
  createCluster,
  updateCluster,
  deleteCluster,
  testClusterConnection,
} = vi.hoisted(() => ({
  useClusterList: vi.fn(),
  createCluster: vi.fn(),
  updateCluster: vi.fn(),
  deleteCluster: vi.fn(),
  testClusterConnection: vi.fn(),
}))

const { successToast, errorToast } = vi.hoisted(() => ({
  successToast: vi.fn(),
  errorToast: vi.fn(),
}))

const { invalidateClusterQueries } = vi.hoisted(() => ({
  invalidateClusterQueries: vi.fn(() => Promise.resolve()),
}))

const { importKubeconfig } = vi.hoisted(() => ({
  importKubeconfig: vi.fn(),
}))

const { useRuntime } = vi.hoisted(() => ({
  useRuntime: vi.fn(),
}))

vi.mock('react-i18next', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-i18next')>()
  return {
    ...actual,
    useTranslation: () => ({
      t: (
        _key: string,
        fallbackOrOptions?: string | { defaultValue?: string }
      ) => {
        if (typeof fallbackOrOptions === 'string') {
          return fallbackOrOptions
        }
        return fallbackOrOptions?.defaultValue ?? _key
      },
    }),
  }
})

vi.mock('sonner', () => ({
  toast: {
    success: successToast,
    error: errorToast,
  },
}))

vi.mock('@/lib/api', () => ({
  useClusterList,
  createCluster,
  updateCluster,
  deleteCluster,
  testClusterConnection,
}))

vi.mock('@/lib/analytics', () => ({
  trackDesktopEvent: vi.fn(),
}))

vi.mock('@/lib/cluster-query', () => ({
  invalidateClusterQueries,
}))

vi.mock('@/lib/desktop', () => ({
  importKubeconfig,
}))

vi.mock('@/contexts/runtime-context', () => ({
  useRuntime,
}))

vi.mock('../action-table', () => ({
  ActionTable: ({
    data,
    columns,
  }: {
    data: Array<Record<string, unknown>>
    columns: any[]
  }) => {
    const table = useReactTable({
      data,
      columns,
      getCoreRowModel: getCoreRowModel(),
    })

    return (
      <table>
        <thead>
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <th key={header.id}>
                  {header.isPlaceholder
                    ? null
                    : flexRender(
                        header.column.columnDef.header,
                        header.getContext()
                      )}
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody>
          {table.getRowModel().rows.map((row) => (
            <tr key={row.id}>
              {row.getVisibleCells().map((cell) => (
                <td key={cell.id}>
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    )
  },
}))

vi.mock('./cluster-dialog', () => ({
  ClusterDialog: () => null,
}))

vi.mock('@/components/delete-confirmation-dialog', () => ({
  DeleteConfirmationDialog: () => null,
}))

import { ClusterManagement } from './cluster-management'

function LocationProbe() {
  const location = useLocation()
  return <div data-testid="location-search">{location.search}</div>
}

function renderComponent(initialEntry: string) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route
            path="/settings"
            element={
              <>
                <ClusterManagement />
                <LocationProbe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('ClusterManagement', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useRuntime.mockReturnValue({
      isDesktop: true,
      isReady: true,
    })
    importKubeconfig.mockResolvedValue({
      importedCount: 1,
      ok: true,
    })
    useClusterList.mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
    })
  })

  it('shows an import success toast once and clears the desktop import marker', async () => {
    renderComponent('/settings?tab=clusters&desktopImport=success')

    await waitFor(() => {
      expect(successToast).toHaveBeenCalledWith(
        'Kubeconfig imported successfully'
      )
    })
    await waitFor(() => {
      expect(invalidateClusterQueries).toHaveBeenCalled()
    })
    expect(errorToast).not.toHaveBeenCalled()

    await waitFor(() => {
      expect(screen.getByTestId('location-search')).toHaveTextContent(
        '?tab=clusters'
      )
    })
  })

  it('shows a skipped import toast when no new clusters were added', async () => {
    renderComponent('/settings?tab=clusters&desktopImport=skipped')

    await waitFor(() => {
      expect(successToast).toHaveBeenCalledWith(
        'No new clusters were imported because matching clusters already exist'
      )
    })
    await waitFor(() => {
      expect(invalidateClusterQueries).toHaveBeenCalled()
    })
  })

  it('shows the api server column and value in the cluster table', () => {
    useClusterList.mockReturnValue({
      data: [
        {
          id: 1,
          name: 'demo',
          apiServer: 'https://demo.example.com:443',
          enabled: true,
          inCluster: false,
          isDefault: true,
          createdAt: '',
          updatedAt: '',
          prometheusURL: '',
        },
      ],
      isLoading: false,
      error: null,
    })

    renderComponent('/settings?tab=clusters')

    expect(screen.getByRole('columnheader', { name: 'API Server' })).toBeInTheDocument()
    expect(screen.getByText('https://demo.example.com:443')).toBeInTheDocument()
  })

  it('shows an import cluster button in desktop mode', () => {
    renderComponent('/settings?tab=clusters')

    expect(
      screen.getByRole('button', { name: 'Import Cluster' })
    ).toBeInTheDocument()
  })

  it('hides the import cluster button outside desktop mode', () => {
    useRuntime.mockReturnValue({
      isDesktop: false,
      isReady: true,
    })

    renderComponent('/settings?tab=clusters')

    expect(
      screen.queryByRole('button', { name: 'Import Cluster' })
    ).not.toBeInTheDocument()
  })

  it('imports clusters from the page button and refreshes the list', async () => {
    const user = userEvent.setup()

    renderComponent('/settings?tab=clusters')

    await user.click(screen.getByRole('button', { name: 'Import Cluster' }))

    await waitFor(() => {
      expect(importKubeconfig).toHaveBeenCalledWith()
    })
    await waitFor(() => {
      expect(invalidateClusterQueries).toHaveBeenCalled()
    })
    expect(successToast).toHaveBeenCalledWith(
      'Kubeconfig imported successfully'
    )
  })
})
