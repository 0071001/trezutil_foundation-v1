// Command trezutil-admin performs operator tasks that must not be exposed over
// HTTP, such as bootstrapping the first platform administrator.
//
//	trezutil-admin create-admin  -email admin@example.com [-name "Jane Doe"] [-roles admin,operator]
//	trezutil-admin set-password  -email admin@example.com
//	trezutil-admin grant-role    -phone +243812345678 -role operator
//	trezutil-admin revoke-role   -phone +243812345678 -role seller
//
//	trezutil-admin reconcile      [-json]                        list payment records that disagree (read only)
//	trezutil-admin repair-ledger  -actor <name> [-apply] [-json]   add missing accounting lines (audited)
//	trezutil-admin credits-held                                   list credits held for actions of unknown outcome
//	trezutil-admin credits-settle -reservation <id> -outcome consume|release -actor <name>
//
// create-admin and set-password read the password from the environment
// variable TREZUTIL_ADMIN_PASSWORD, or from the first line of standard input
// with -password-stdin. There is deliberately no -password flag: a password
// on the command line ends up in the shell history and the process list.
// create-admin creates the account, or, when the email already exists, sets its
// password and adds the missing roles.
//
// grant-role and revoke-role need the user to have signed in once (phone OTP)
// so the account exists.
// Roles: customer, seller, staff, admin, operator, moderator, delivery_partner.
// Staff roles (admin, operator, moderator) can sign in with email and password.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application/identity"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/database"
)

const usage = `usage:
  trezutil-admin create-admin -email <address> [-name <display name>] [-roles admin,operator,moderator] [-password-stdin]
  trezutil-admin set-password -email <address> [-password-stdin]
  trezutil-admin grant-role   (-phone <E.164> | -email <address>) -role <role>
  trezutil-admin revoke-role  (-phone <E.164> | -email <address>) -role <role>
  trezutil-admin reconcile    [-json]
  trezutil-admin repair-ledger -actor <name> [-apply] [-json]
  trezutil-admin credits-held
  trezutil-admin credits-settle -reservation <id> -outcome consume|release -actor <name>
the password comes from $TREZUTIL_ADMIN_PASSWORD or, with -password-stdin, from standard input`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	switch cmd := os.Args[1]; cmd {
	case "grant-role", "revoke-role":
		roleCommand(cmd, os.Args[2:])
	case "create-admin", "set-password":
		staffCommand(cmd, os.Args[2:])
	case "reconcile", "repair-ledger":
		reconcileCommand(cmd, os.Args[2:])
	case "credits-held", "credits-settle":
		creditsCommand(cmd, os.Args[2:])
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
}

func openDB(ctx context.Context) *postgres.UserRepository {
	users, _ := openStores(ctx)
	return users
}

// openStores opens the users and the audit trail (role changes are recorded there).
func openStores(ctx context.Context) (*postgres.UserRepository, *postgres.AuditRepository) {
	db, err := database.Open(ctx)
	if err != nil {
		fail(err)
	}
	return postgres.NewUserRepository(db), postgres.NewAuditRepository(db)
}

func staffCommand(cmd string, args []string) {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	email := fs.String("email", "", "email address of the staff account")
	name := fs.String("name", "", "display name (create-admin)")
	roleList := fs.String("roles", "admin", "comma-separated staff roles (create-admin)")
	fromStdin := fs.Bool("password-stdin", false, "read the password from the first line of standard input")
	_ = fs.Parse(args)

	password, err := readPassword(*fromStdin)
	if err != nil {
		fail(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	users := openDB(ctx)
	now := time.Now().Unix()

	var roles []domain.Role
	for _, r := range strings.Split(*roleList, ",") {
		if r = strings.TrimSpace(r); r != "" {
			roles = append(roles, domain.Role(r))
		}
	}

	if cmd == "set-password" {
		normalized, err := domain.NormalizeEmail(*email)
		if err != nil {
			fail(err)
		}
		existing, err := users.FindUserByEmail(ctx, normalized)
		if err != nil {
			fail(fmt.Errorf("no account with email %s: %w", normalized, err))
		}
		// Keep the account's staff roles as they are.
		roles = nil
		for _, r := range existing.Roles {
			if isStaffRole(r) {
				roles = append(roles, r)
			}
		}
		if len(roles) == 0 {
			fail(errors.New("this account has no staff role; use create-admin"))
		}
	}

	res, err := identity.ProvisionStaff(ctx, users, users, identity.StaffInput{Email: *email, DisplayName: *name, Password: password, Roles: roles}, now)
	if err != nil {
		fail(err)
	}
	verb := "updated"
	if res.Created {
		verb = "created"
	}
	roleNames := make([]string, 0, len(res.User.Roles))
	for _, r := range res.User.Roles {
		roleNames = append(roleNames, string(r))
	}
	fmt.Printf("%s staff account %s (%s), roles: %s\n", verb, *res.User.Email, res.User.ID, strings.Join(roleNames, ", "))
}

// readPassword takes the password from standard input or from the environment,
// never from the command line.
func readPassword(fromStdin bool) (string, error) {
	if fromStdin {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", errors.New("no password on standard input")
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	if p := os.Getenv("TREZUTIL_ADMIN_PASSWORD"); p != "" {
		return p, nil
	}
	return "", errors.New("set TREZUTIL_ADMIN_PASSWORD, or pass -password-stdin and type the password on standard input")
}

func roleCommand(cmd string, args []string) {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	phone := fs.String("phone", "", "user phone number")
	email := fs.String("email", "", "user email address (an account opened with Google has no phone)")
	roleName := fs.String("role", "", "platform role")
	_ = fs.Parse(args)
	if (*phone == "") == (*email == "") {
		fail(errors.New("give either -phone or -email"))
	}

	role := domain.Role(*roleName)
	if !role.IsValid() {
		fail(fmt.Errorf("unknown role %q", *roleName))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	users, audit := openStores(ctx)
	var user *domain.User
	var err error
	who := strings.TrimSpace(*email)
	if who != "" {
		user, err = users.FindUserByEmail(ctx, who)
	} else {
		normalized, perr := domain.NormalizePhone(*phone)
		if perr != nil {
			fail(fmt.Errorf("invalid phone %q", *phone))
		}
		who = normalized
		user, err = users.FindByPhone(ctx, normalized)
	}
	if err != nil || user == nil {
		fail(fmt.Errorf("no account for %s (the user must sign in once first): %v", who, err))
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
	action := "user.role_granted"
	if cmd == "revoke-role" {
		action = "user.role_revoked"
	}
	if aerr := audit.Append(ctx, domain.AuditEntry{ActorID: "cli:trezutil-admin", Action: action, TargetType: "user", TargetID: user.ID,
		Metadata: map[string]interface{}{"role": string(role), "via": "trezutil-admin " + cmd}}); aerr != nil {
		fmt.Fprintf(os.Stderr, "warning: the role change was made but could not be written to the audit trail: %v\n", aerr)
	}
	fmt.Printf("%s %s for user %s (%s)\n", cmd, role, user.ID, who)
}

func isStaffRole(r domain.Role) bool {
	for _, s := range domain.StaffRoles {
		if s == r {
			return true
		}
	}
	return false
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "trezutil-admin:", err)
	os.Exit(1)
}
