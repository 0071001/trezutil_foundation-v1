import { http } from '@/api/client'
import { arr, num, obj, str } from '@/api/json'

/* GET /me/credits: the signed-in person's own credits (their personal space), distinct from an organisation's. */

export interface PersonalCredits {
  periodId: string
  /** False until the person's credits were first used (balance 0, nothing created by reading). */
  opened: boolean
  granted: number
  reserved: number
  consumed: number
  /** granted − reserved − consumed, as the server checks it before a metered action. */
  balance: number
  ledger: { id: string; actionType: string; kind: string; amount: number; createdAt: string }[]
  /** False when no active policy charges a personal account: actions are free. */
  meteringActive: boolean
  costs: { actionType: string; channel: string; cost: number }[]
}

export const creditsApi = {
  async mine(): Promise<PersonalCredits> {
    const raw = obj(await http.get<unknown>('/me/credits'))
    const p = obj(raw.personal)
    const m = obj(raw.metering)
    return {
      periodId: str(raw.period_id),
      opened: p.opened === true,
      granted: num(p.granted), reserved: num(p.reserved), consumed: num(p.consumed), balance: num(p.balance),
      ledger: arr(raw.ledger).map((v) => {
        const e = obj(v)
        return { id: str(e.id), actionType: str(e.action_type), kind: str(e.kind), amount: num(e.amount), createdAt: str(e.created_at) }
      }),
      meteringActive: m.active === true,
      costs: arr(m.costs).map((v) => {
        const c = obj(v)
        return { actionType: str(c.action_type), channel: str(c.channel), cost: num(c.cost) }
      }),
    }
  },
}
