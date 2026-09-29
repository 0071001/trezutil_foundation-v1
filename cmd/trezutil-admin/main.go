// Command trezutil-admin performs operator tasks that must not be exposed over
// HTTP, such as bootstrapping the first platform operator.
//
//	trezutil-admin grant-role  -phone +243812345678 -role operator
//	trezutil-admin revoke-role -phone +243812345678 -role seller
//
// The user must have signed in once (phone OTP) so the account exists.
// Roles: customer, seller, staff, admin, operator, moderator, delivery_partner.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/database"
)

func main() {
	if len(os.Args) < 2 || (os.Args[1] != "grant-role" && os.Args[1] != "revoke-role") {
		fmt.Fprintln(os.Stderr, "usage: trezutil-admin grant-role|revoke-role -phone <E.164> -role <role>")
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	phone := fs.String("phone", "", "user phone number")
	roleName := fs.String("role", "", "platform role")
	_ = fs.Parse(os.Args[2:])

	role := domain.Role(*roleName)
	if !role.IsValid() {
		fail(fmt.Errorf("unknown role %q", *roleName))
	}
	normalized, err := domain.NormalizePhone(*phone)
	if err != nil {
		fail(fmt.Errorf("invalid phone %q", *phone))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx)
	if err != nil {
		fail(err)
	}
	defer db.Close()

	users := postgres.NewUserRepository(db)
	user, err := users.FindByPhone(ctx, normalized)
	if err != nil {
		fail(fmt.Errorf("no account for %s (the user must sign in once first): %w", normalized, err))
	}

	switch cmd {
	case "grant-role":
		err = users.AssignRole(ctx, user.ID, role)
		if errors.Is(err, domain.ErrRoleAlreadyAssigned) {
			err = nil
		}
	case "revoke-role":
		err = users.RemoveRole(ctx, user.ID, role)
	}
	if err != nil {
		fail(err)
	}
	fmt.Printf("%s %s for user %s (%s)\n", cmd, role, user.ID, normalized)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "trezutil-admin:", err)
	os.Exit(1)
}
