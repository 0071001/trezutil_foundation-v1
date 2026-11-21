import { createContext, useContext, type ReactNode } from 'react'
import { ordersApi, type OrderSummary } from '@/api/commerce'
import { missionsApi, type Mission } from '@/api/missions'
import { useSession } from '@/lib/session'
import { useAsync, type AsyncState } from '@/lib/useAsync'

/* The account's server data the frame needs everywhere (operations, orders), read once and shared. */

interface Workspace { missions: AsyncState<Mission[]>; orders: AsyncState<OrderSummary[]> }

const WorkspaceContext = createContext<Workspace | null>(null)

export function WorkspaceProvider({ children }: { children: ReactNode }) {
  const { signedIn } = useSession()
  const missions = useAsync(async () => (await missionsApi.list()).items, [signedIn], signedIn)
  const orders = useAsync(() => ordersApi.list(), [signedIn], signedIn)
  return <WorkspaceContext.Provider value={{ missions, orders }}>{children}</WorkspaceContext.Provider>
}

export function useWorkspace(): Workspace {
  const ctx = useContext(WorkspaceContext)
  if (!ctx) throw new Error('useWorkspace outside WorkspaceProvider')
  return ctx
}
