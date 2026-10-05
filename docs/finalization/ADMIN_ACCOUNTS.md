# Platform administration accounts

Customers and sellers sign in with a phone number and a one-time code. The Trezutil team (administrators, operators, moderators) signs in differently: **an email address and a password**, on its own page, and reaches its own console.

| | |
|---|---|
| Sign-in page | `/admin/login` |
| Console | `/admin` (overview), `/admin/users` (accounts) |
| Sign-in API | `POST /api/v1/auth/staff/login` `{ "email", "password" }` |
| Console API | `GET /api/v1/admin/overview` (admin, operator), `GET /api/v1/admin/analytics?days=` (admin, operator), `GET /api/v1/admin/users?q=&page=&page_size=` (admin), `GET`/`PUT /api/v1/admin/site/content` and `POST /api/v1/admin/site/assets` (admin; see [DASHBOARDS_AND_SITE_CONTENT.md](../web/DASHBOARDS_AND_SITE_CONTENT.md)) |
| Roles that can sign in with a password | `admin`, `operator`, `moderator` |

## Creating the first administrator

The account is created by an operator who has access to the database, never over HTTP. The password is read from the environment (or standard input), never from a command-line flag, so it does not end up in a shell history or a process list.

```bash
TREZUTIL_ADMIN_PASSWORD='<a long password>' \
docker compose -p trezutil -f deployments/docker/docker-compose.app.yml --env-file <env file> \
  run --rm -e TREZUTIL_ADMIN_PASSWORD migrate \
  /app/bin/trezutil-admin create-admin -email admin@example.com -name "Jane Doe"
```

- `create-admin` creates the account, or, when the email already exists, sets its password and adds the missing roles. `-roles admin,operator` adds more than one role.
- `set-password -email …` changes the password of an existing staff account.
- Without Docker: `printf '%s\n' "$PASSWORD" | trezutil-admin create-admin -email … -password-stdin` with `DATABASE_URL` set.
- A password must have at least 10 characters (at most 128), mix at least three kinds of characters (lower case, upper case, digits, symbols), not be a well-known password and not contain the email address.

A staff account has an email and **no phone number**: the phone column is optional since migration `000023_staff_credentials.sql`, and the email is unique ignoring case.

## What protects the sign-in

- **Only a hash is stored**: PBKDF2-HMAC-SHA256, 600 000 iterations, a random 16-byte salt, with the parameters stored in the hash so the cost can be raised later. Comparison is constant-time.
- **One answer for every failure**: unknown email, no password, wrong password, locked account, suspended account and a non-staff account all return the same `401 INVALID_CREDENTIALS`, and each spends the time of a password check, so the response reveals nothing about which accounts exist.
- **Lockout**: 5 wrong passwords in a row lock the account for 15 minutes (`STAFF_LOGIN_MAX_FAILURES`, `STAFF_LOGIN_LOCKOUT_MINUTES`). A success or a password reset clears the count; a wrong password after an expired lock starts a fresh count. The count is kept under a row lock, so parallel guesses cannot slip past it.
- **Per-address limit**: 10 sign-in attempts per client IP per 10 minutes (`STAFF_LOGIN_MAX_PER_IP_10MIN`), answered `429` before the password is looked at.
- **Roles are checked on the server on every request.** The console API reads the caller's roles from the database, not from the token, so a revoked role or a suspended account stops working at once. The account list (emails, phone numbers) is for administrators only; operators see the overview.
- Responses of the sign-in and of the console API are `Cache-Control: no-store`. The password never appears in logs; sign-in outcomes are logged without it.
- Tokens are the same short-lived access tokens and refresh tokens as for customers.

## Operating it

- **A forgotten password or a locked account**: run `set-password` (it also clears the lock).
- **Revoking access**: remove the role (`trezutil-admin revoke-role` works with a phone number; for an email-only account, delete the `user_roles` row or suspend the user in the database), or suspend the account. The next request is refused.
- **Production**: put the console behind the same HTTPS gateway as the site. Consider restricting `/admin` and `/api/v1/admin` by network or by a second factor at the proxy; this release has no built-in two-factor sign-in.

## Not in this release

The console is read-only: no suspending accounts, no granting or revoking roles from the page, no password change page, no two-factor sign-in, no sign-in history. These are the next steps for staff accounts.
