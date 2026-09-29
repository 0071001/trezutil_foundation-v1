// Command fakeproviders serves the Stripe and Anthropic API emulators on fixed
// local ports for the real-stack E2E journeys (test/e2e/README.md). It is a
// test tool: it is not built into any image and must never be deployed.
//
//	FAKE_STRIPE_ADDR=127.0.0.1:12111  FAKE_STRIPE_KEY=sk_test_e2e
//	FAKE_ANTHROPIC_ADDR=127.0.0.1:12112 FAKE_ANTHROPIC_KEY=sk-ant-e2e
//
// POST /_test/confirm/{payment_intent_id} on the Stripe port simulates the
// payer confirming a PaymentIntent in the browser (Stripe.js).
package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/trezutil/trezutil/internal/infrastructure/ai/anthropic/anthropicfake"
	"github.com/trezutil/trezutil/internal/infrastructure/payment/stripe/stripefake"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	stripeSrv := stripefake.New(env("FAKE_STRIPE_KEY", "sk_test_e2e"))
	defer stripeSrv.Close()
	stripeMux := http.NewServeMux()
	stripeMux.HandleFunc("/_test/confirm/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/_test/confirm/")
		stripeSrv.Confirm(id)
		pi, ok := stripeSrv.Intent(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + pi.ID + `","status":"` + pi.Status + `"}`))
	})
	stripeMux.Handle("/", stripeSrv.Handler())

	ai := &anthropicfake.Handler{APIKey: env("FAKE_ANTHROPIC_KEY", "sk-ant-e2e")}

	errs := make(chan error, 2)
	go func() { errs <- http.ListenAndServe(env("FAKE_STRIPE_ADDR", "127.0.0.1:12111"), stripeMux) }()
	go func() { errs <- http.ListenAndServe(env("FAKE_ANTHROPIC_ADDR", "127.0.0.1:12112"), ai) }()
	log.Printf("fake Stripe on %s, fake Anthropic on %s", env("FAKE_STRIPE_ADDR", "127.0.0.1:12111"), env("FAKE_ANTHROPIC_ADDR", "127.0.0.1:12112"))
	log.Fatal(<-errs)
}
