import { http } from '@/api/client'
import { arr, num, obj, str } from '@/api/json'

/*
 * What the Control Center creates, changes and removes (administrators; the server checks the role and writes the
 * audit trail): plans, subscriptions, manual credits, pricing rules, categories and staff roles. Orders, payments,
 * bookings, the credit ledger and the audit trail are never deleted from here.
 */

export interface PlanInput {
  name: string
  slug: string
  price_minor: number
  currency: 'USD' | 'CDF'
  billing_cycle: 'monthly' | 'yearly'
  included_credits: number
  credit_pack: number
  status: 'active' | 'inactive' | 'archived'
}

export interface SubscriptionRow {
  id: string
  planId: string
  planName: string
  userId: string
  userName: string
  userEmail: string
  userPhone: string
  status: string
  periodId: string
  startedAt: string
}

export interface PolicyRow {
  id: string
  action_type: string
  channel: string
  actor_type: string
  cost: number
  active: boolean
}

const mapPolicy = (v: unknown): PolicyRow => {
  const p = obj(v)
  return { id: str(p.id), action_type: str(p.action_type), channel: str(p.channel), actor_type: str(p.actor_type), cost: num(p.cost), active: p.active === true }
}

export const adminManageApi = {
  createPlan: (p: PlanInput) => http.post('/admin/plans', p),
  updatePlan: (id: string, p: PlanInput) => http.patch(`/admin/plans/${encodeURIComponent(id)}`, p),
  /** Deletes a plan nobody used; a used plan is archived instead (archived: true). */
  async deletePlan(id: string): Promise<{ archived: boolean }> {
    const r = obj(await http.del<unknown>(`/admin/plans/${encodeURIComponent(id)}`))
    return { archived: r.archived === true }
  },

  async subscriptions(): Promise<SubscriptionRow[]> {
    return arr(obj(await http.get<unknown>('/admin/subscriptions')).items).map((v) => {
      const s = obj(v)
      return {
        id: str(s.id), planId: str(s.plan_id), planName: str(s.plan_name), userId: str(s.user_id), userName: str(s.user_name),
        userEmail: str(s.user_email), userPhone: str(s.user_phone), status: str(s.status), periodId: str(s.current_period_id), startedAt: str(s.started_at),
      }
    })
  },
  subscribe: (userId: string, planId: string) => http.post('/admin/subscriptions', { user_id: userId, plan_id: planId }),
  cancelSubscription: (id: string) => http.post(`/admin/subscriptions/${encodeURIComponent(id)}/cancel`),

  /** Manual credits for this month; the idempotency key makes a retried click grant once. */
  grantCredits: (userId: string, amount: number, reason: string, idempotencyKey: string) =>
    http.post('/admin/credits/grants', { user_id: userId, amount, reason }, { idempotencyKey }),

  async policies(): Promise<PolicyRow[]> {
    return arr(obj(await http.get<unknown>('/admin/usage-policies')).items).map(mapPolicy)
  },
  async billableActions(): Promise<string[]> {
    return arr(obj(await http.get<unknown>('/admin/usage-policies/actions')).items).map(str)
  },
  createPolicy: (p: Omit<PolicyRow, 'id'>) => http.post('/admin/usage-policies', p),
  updatePolicy: (p: PolicyRow) => http.patch(`/admin/usage-policies/${encodeURIComponent(p.id)}`, p),
  deletePolicy: (id: string) => http.del(`/admin/usage-policies/${encodeURIComponent(id)}`),

  updateCategory: (id: string, c: { name: string; slug: string; parent_id: string | null }) => http.patch(`/admin/categories/${encodeURIComponent(id)}`, c),
  deleteCategory: (id: string) => http.del(`/admin/categories/${encodeURIComponent(id)}`),

  grantRole: (userId: string, role: string) => http.post(`/admin/users/${encodeURIComponent(userId)}/roles`, { role }),
  revokeRole: (userId: string, role: string) => http.del(`/admin/users/${encodeURIComponent(userId)}/roles/${encodeURIComponent(role)}`),
}
