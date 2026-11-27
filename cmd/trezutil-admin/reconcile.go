package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application/payment"
	"github.com/trezutil/trezutil/packages/database"
)

// reconcileCommand runs the payment reconciliation: "reconcile" only reads; "repair-ledger" adds missing accounting
// lines when asked with -apply. Neither calls a payment provider or moves money.
func reconcileCommand(cmd string, args []string) {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	asJSON := fs.Bool("json", false, "print the result as JSON")
	apply := fs.Bool("apply", false, "repair-ledger: write the missing lines (without it, only list them)")
	actor := fs.String("actor", "", "repair-ledger: who asks for the repair (recorded in the audit trail)")
	_ = fs.Parse(args)
	if cmd == "repair-ledger" && strings.TrimSpace(*actor) == "" {
		fail(errors.New("repair-ledger needs -actor <your name or email>"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := database.Open(ctx)
	if err != nil {
		fail(err)
	}
	defer db.Close()
	reconciler := &payment.Reconciler{
		Source:     postgres.NewPaymentReconciliation(db),
		Payments:   postgres.NewPaymentRepository(db),
		Refunds:    postgres.NewRefundRepository(db),
		Accounting: postgres.NewPaymentAccounting(db),
		Locker:     postgres.NewAdvisoryLocker(db),
		Audit:      postgres.NewAuditRepository(db),
	}

	if cmd == "reconcile" {
		findings, err := reconciler.Report(ctx)
		if err != nil {
			fail(err)
		}
		if *asJSON {
			_ = json.NewEncoder(os.Stdout).Encode(findings)
		} else {
			for _, f := range findings {
				fmt.Printf("%-24s org=%s payment=%s refund=%s  %s\n", f.Kind, f.TenantID, f.PaymentID, f.RefundID, f.Detail)
			}
			fmt.Printf("%d finding(s)\n", len(findings))
		}
		// A non-zero status lets a scheduled run raise an alert.
		if len(findings) > 0 {
			os.Exit(3)
		}
		return
	}

	repairs, err := reconciler.RepairLedger(ctx, strings.TrimSpace(*actor), *apply)
	if err != nil {
		fail(err)
	}
	failed := 0
	for _, r := range repairs {
		if r.Error != "" {
			failed++
		}
	}
	if *asJSON {
		_ = json.NewEncoder(os.Stdout).Encode(repairs)
	} else {
		for _, r := range repairs {
			state := "would repair"
			if *apply {
				state = fmt.Sprintf("added %d line(s)", r.Posted)
			}
			if r.Error != "" {
				state = "not repaired: " + r.Error
			}
			fmt.Printf("%-24s payment=%s refund=%s  %s\n", r.Finding.Kind, r.Finding.PaymentID, r.Finding.RefundID, state)
		}
		if !*apply {
			fmt.Printf("%d record(s); run again with -apply to write the missing lines\n", len(repairs))
		}
	}
	if failed > 0 {
		os.Exit(1)
	}
}
