package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/database"
)

// creditsCommand handles reservations held because an action started and its outcome is unknown: "credits-held" lists
// them; "credits-settle" charges or returns one after a person checked whether the action happened.
func creditsCommand(cmd string, args []string) {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	id := fs.String("reservation", "", "credits-settle: reservation id")
	outcome := fs.String("outcome", "", "credits-settle: consume (the action happened) or release (it did not)")
	actor := fs.String("actor", "", "credits-settle: who decides (recorded in the audit trail)")
	_ = fs.Parse(args)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db, err := database.Open(ctx)
	if err != nil {
		fail(err)
	}
	defer db.Close()
	reservations := postgres.NewUsageReservationRepository(db)
	now := time.Now().Unix()

	held, err := reservations.HeldReservations(ctx, 15*time.Minute, now)
	if err != nil {
		fail(err)
	}
	if cmd == "credits-held" {
		for _, r := range held {
			fmt.Printf("%s  account=%s action=%s credits=%d since %s\n", r.ReservationID, r.AccountID, r.ActionType, r.Amount,
				time.Unix(r.ReservedAt, 0).UTC().Format(time.RFC3339))
		}
		fmt.Printf("%d held reservation(s)\n", len(held))
		return
	}

	who := strings.TrimSpace(*actor)
	if who == "" || *id == "" || (*outcome != "consume" && *outcome != "release") {
		fail(errors.New("credits-settle needs -reservation <id> -outcome consume|release -actor <your name or email>"))
	}
	var found *domain.UsageReservation
	for i := range held {
		if held[i].ReservationID == *id {
			found = &held[i]
		}
	}
	if found == nil {
		fail(errors.New("not a held reservation (see credits-held)"))
	}
	if *outcome == "consume" {
		err = reservations.ConsumeReservation(ctx, *id, now)
	} else {
		err = reservations.ReleaseReservation(ctx, *id, now)
	}
	if err != nil {
		fail(err)
	}
	if err := postgres.NewAuditRepository(db).Append(ctx, domain.AuditEntry{ActorID: who, Action: "credits.reservation.settled",
		TargetType: "usage_reservation", TargetID: *id, OccurredAt: now,
		Metadata: map[string]interface{}{"outcome": *outcome, "credits": found.Amount, "action_type": string(found.ActionType)}}); err != nil {
		fail(fmt.Errorf("settled, but not recorded in the audit trail: %w", err))
	}
	fmt.Printf("reservation %s: %sd %d credit(s)\n", *id, *outcome, found.Amount)
}
