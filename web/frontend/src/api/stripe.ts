import { t } from '@/lib/i18n'
/**
 * Minimal Stripe.js loader. Card details are entered in Stripe's own Payment
 * Element (an iframe served by Stripe), so they never touch Trezutil servers;
 * the backend only hands the payer a PaymentIntent client_secret and the
 * organization's publishable key.
 */

const STRIPE_JS = 'https://js.stripe.com/v3/'

export interface StripeElement {
  mount(target: HTMLElement): void
  destroy(): void
}

export interface StripeElements {
  create(type: 'payment'): StripeElement
}

export interface StripeClient {
  elements(options: { clientSecret: string }): StripeElements
  confirmPayment(options: {
    elements: StripeElements
    confirmParams: { return_url: string }
    redirect?: 'if_required'
  }): Promise<{ error?: { message?: string }; paymentIntent?: { status: string } }>
}

declare global {
  interface Window {
    Stripe?: (publishableKey: string) => StripeClient
  }
}

let loading: Promise<void> | null = null

function loadScript(): Promise<void> {
  if (window.Stripe) return Promise.resolve()
  if (!loading) {
    loading = new Promise<void>((resolve, reject) => {
      const script = document.createElement('script')
      script.src = STRIPE_JS
      script.async = true
      script.onload = () => resolve()
      script.onerror = () => {
        loading = null
        reject(new Error(t('Could not load the secure payment form. Check your connection and try again.')))
      }
      document.head.appendChild(script)
    })
  }
  return loading
}

export async function loadStripe(publishableKey: string): Promise<StripeClient> {
  await loadScript()
  if (!window.Stripe) throw new Error(t('The secure payment form is unavailable.'))
  return window.Stripe(publishableKey)
}

/** Payer-facing data returned by POST /payments when the payer must act. */
export interface ClientAction {
  provider_code?: string
  client_secret?: string
  publishable_key?: string
}

export function isStripeAction(action: ClientAction | undefined): action is Required<Pick<ClientAction, 'client_secret' | 'publishable_key'>> & ClientAction {
  return !!action && action.provider_code === 'stripe' && !!action.client_secret && !!action.publishable_key
}
