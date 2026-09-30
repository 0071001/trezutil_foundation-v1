"""Real-stack E2E journeys through the public gateway (http://localhost:8080/api/v1).

PostgreSQL (trezutil_e2e) and OpenSearch 2.17.1 are real. OTP codes are read
from the identity-service development log (APP_ENV=development); WhatsApp
webhooks are signed locally with the test app secret. Journey D runs before
any payment provider exists (payment creation -> 503); journey S then adds a
Stripe provider row pointing at the Stripe emulator (test/e2e/fakeproviders)
and exercises real payment flows with signed webhooks. The conversation service
runs with AI_PROVIDER=anthropic against the Anthropic emulator.
"""
import hashlib, hmac, json, os, re, subprocess, sys, time, uuid, urllib.request, urllib.error

# Configuration (see test/e2e/README.md)
GW = os.environ.get("E2E_GATEWAY", "http://localhost:8080") + "/api/v1"
LOG = os.environ["E2E_IDENTITY_LOG"]  # identity-service stdout; APP_ENV=development logs OTP codes
PSQL = os.environ.get("E2E_PSQL", "psql")
DB = os.environ.get("E2E_DB", "trezutil_e2e")
DB_USER = os.environ.get("E2E_DB_USER", "trezutil")
DB_PASSWORD = os.environ.get("E2E_DB_PASSWORD", "trezutil")
WA_SECRET = os.environ.get("E2E_WHATSAPP_APP_SECRET", "wa-e2e-secret").encode()
WA_VERIFY = os.environ.get("E2E_WHATSAPP_VERIFY_TOKEN", "wa-verify")
results = []
RUN = uuid.uuid4().hex[:8]

def call(method, path, body=None, token=None, headers=None, raw=None):
    data = raw if raw is not None else (json.dumps(body).encode() if body is not None else None)
    req = urllib.request.Request(GW + path if path.startswith("/") else path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    for k, v in (headers or {}).items():
        req.add_header(k, v)
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            txt = r.read().decode()
            return r.status, (json.loads(txt) if txt.strip().startswith(("{", "[")) else txt)
    except urllib.error.HTTPError as e:
        txt = e.read().decode()
        try:
            return e.code, json.loads(txt)
        except Exception:
            return e.code, txt

def check(name, cond, detail=""):
    results.append((name, bool(cond)))
    print(("PASS " if cond else "FAIL ") + name + ("" if cond else f"  -> {detail}"))

def sql(q):
    env = dict(os.environ, PGPASSWORD=DB_PASSWORD)
    out = subprocess.run([PSQL, "-h", "localhost", "-U", DB_USER, "-d", DB, "-tAc", q],
                         capture_output=True, text=True, env=env)
    if out.returncode != 0:
        raise RuntimeError(out.stderr)
    return out.stdout.strip()

def login(phone):
    st, body = call("POST", "/auth/request-otp", {"phone": phone})
    assert st == 200, (st, body)
    rid = body["request_id"]
    time.sleep(0.3)
    code = None
    for line in open(LOG, encoding="utf-8", errors="ignore"):
        if rid and '"otp_code"' in line:
            rec = json.loads(line)
            code = rec["otp_code"]
    st, sess = call("POST", "/auth/verify-otp", {"request_id": rid, "code": code})
    assert st == 200, (st, sess)
    return sess["tokens"]["access_token"], sess["user"]["id"], rid

# ---------- Identity ----------
st, body = call("POST", "/auth/request-otp", {"phone": "+243811110000"})
rid = body.get("request_id")
st2, bad = call("POST", "/auth/verify-otp", {"request_id": rid, "code": "000000"})
check("AUTH: wrong OTP code rejected (401)", st2 == 401, (st2, bad))
seller_token, seller_id, _ = login("+243811111111")
check("AUTH: seller signs in with phone OTP -> JWT", seller_token.count(".") == 2)
st, me = call("GET", "/auth/users/me", token=seller_token)
check("AUTH: /auth/users/me returns verified identity", st == 200 and me["id"] == seller_id and me["status"] == "active", me)
st, _ = call("GET", "/auth/users/me", token=seller_token + "x")
check("AUTH: tampered token rejected (401)", st == 401)
customer_token, customer_id, _ = login("0822222222")
check("AUTH: local DRC number normalized (+243) and customer provisioned", customer_id != seller_id)

# ---------- Seller onboarding (operator-assisted, via the public API) ----------
ADMIN_CLI = os.environ.get("E2E_ADMIN_CLI")  # trezutil-admin binary; falls back to SQL

def grant_role(phone, role):
    if ADMIN_CLI:
        env = dict(os.environ, DATABASE_URL=f"postgres://{DB_USER}:{DB_PASSWORD}@localhost:5432/{DB}?sslmode=disable")
        out = subprocess.run([ADMIN_CLI, "grant-role", "-phone", phone, "-role", role], capture_output=True, text=True, env=env)
        assert out.returncode == 0, out.stderr
    else:
        sql(f"INSERT INTO user_roles (user_id, role_id) SELECT u.id, r.id FROM users u, roles r WHERE u.phone='{phone}' AND r.name='{role}' ON CONFLICT DO NOTHING")

operator_phone = "+24380000" + RUN[:4].translate(str.maketrans("abcdef", "123456"))
operator_token, operator_id, _ = login(operator_phone)
grant_role(operator_phone, "operator")
st, org_body = call("POST", "/organizations", {"Type": "business", "LegalName": "Gombe Stays SARL", "DisplayName": "Gombe Stays",
                                               "Phone": "+243811111111", "Country": "CD", "Status": "active"}, token=seller_token)
check("ONBOARD: non-operator cannot create an organization", st >= 400, (st, org_body))
st, org_body = call("POST", "/organizations", {"Type": "business", "LegalName": "Gombe Stays SARL", "DisplayName": "Gombe Stays",
                                               "Phone": "+243811111111", "Country": "CD", "Status": "active"}, token=operator_token)
org = org_body.get("ID") if isinstance(org_body, dict) else None
check("ONBOARD: operator creates organization", st == 201 and org, (st, org_body))
st, m = call("POST", f"/organizations/members?organization_id={org}", {"user_id": seller_id, "membership_role": "owner"}, token=operator_token)
check("ONBOARD: operator adds seller account as owner", st == 201, (st, m))
st, mem = call("GET", f"/organizations/members?organization_id={org}", token=customer_token)
check("ONBOARD: non-member cannot list organization members", st == 403, st)
grant_role("+243811111111", "seller")
st, sp = call("POST", "/sellers", {"OrganizationID": org, "SellerType": "organization", "DisplayName": "Gombe Stays",
                                   "Phone": "+243811111111", "UserID": customer_id, "TrustLevel": 5, "VerificationStatus": "verified"}, token=seller_token)
seller = sp.get("ID") if isinstance(sp, dict) else None
check("ONBOARD: seller creates seller profile", st == 201 and seller, (st, sp))
check("ONBOARD: profile bound to caller; trust not self-declared",
      sql(f"SELECT user_id||'|'||trust_level||'|'||verification_status FROM seller_profiles WHERE id='{seller}'") == f"{seller_id}|0|unverified")
st, so = call("POST", "/stores", {"OrganizationID": org, "Name": "Gombe Stays", "Slug": "gombe-stays-" + RUN, "Country": "CD",
                                  "Phone": "+243811111111", "Status": "active"}, token=seller_token)
store = so.get("ID") if isinstance(so, dict) else None
check("ONBOARD: owner creates store", st == 201 and store, (st, so))
st, cat_body = call("POST", "/categories", {"name": "Apartments", "slug": "apartments-" + RUN}, token=seller_token)
check("ONBOARD: only operators manage categories", st == 403, st)
st, cat_body = call("POST", "/categories", {"name": "Apartments", "slug": "apartments-" + RUN}, token=operator_token)
cat = cat_body.get("id") if isinstance(cat_body, dict) else None
check("ONBOARD: operator creates category", st == 201 and cat, (st, cat_body))
st, cats = call("GET", "/categories")
check("ONBOARD: category catalogue is public", st == 200 and any(c["id"] == cat for c in cats.get("items", [])), st)

# ---------- Journey E: Seller -> Create -> Publish -> Projection -> Discoverable ----------
listing = {"listing": {"StoreID": store, "SellerID": seller, "CategoryID": cat, "Title": "Riverside apartment Gombe",
                       "ListingType": "real_estate", "PricingModel": "fixed", "PriceAmount": 85.5, "PriceCurrency": "USD",
                       "Status": "published", "ID": "attacker-chosen", "Metadata": {"bedrooms": 2}}}
st, created = call("POST", "/listings", listing, token=customer_token)
check("E: non-member cannot create listing in seller's store (403/500 forbidden)", st >= 400, (st, created))
st, created = call("POST", "/listings", listing, token=seller_token)
lid = created.get("ID") if isinstance(created, dict) else None
check("E: seller creates listing (201)", st == 201 and lid, (st, created))
check("E: server assigns ID and forces draft status", lid != "attacker-chosen" and created.get("Status") == "draft", created)
check("E: listing metadata persisted", sql(f"SELECT metadata->>'bedrooms' FROM listings WHERE id='{lid}'") == "2")

# ---------- Verticals: a listing belongs to one commerce vertical (migration 000019) ----------
check("V: listing without a vertical gets its listing type's default (REAL_ESTATE)",
      created.get("Vertical") == "REAL_ESTATE" and sql(f"SELECT vertical FROM listings WHERE id='{lid}'") == "REAL_ESTATE", created.get("Vertical"))
room = {"listing": dict(listing["listing"], Title="Gombe Stays deluxe room", Vertical="hotel", ID=None)}
st, room_body = call("POST", "/listings", room, token=seller_token)
room_id = room_body.get("ID") if isinstance(room_body, dict) else None
check("V: the same seller lists a room in the HOTEL vertical (stays a draft)",
      st == 201 and room_body.get("Vertical") == "HOTEL" and sql(f"SELECT vertical||'|'||status FROM listings WHERE id='{room_id}'") == "HOTEL|draft", (st, room_body))
st, det = call("GET", f"/listings?listing_id={room_id}", token=seller_token)
check("V: listing detail returns the vertical", st == 200 and det.get("Vertical") == "HOTEL", (st, det.get("Vertical") if isinstance(det, dict) else det))
st, bad = call("POST", "/listings", {"listing": dict(listing["listing"], Vertical="SPACESHIPS", ID=None)}, token=seller_token)
check("V: unknown vertical is refused (400)", st == 400 and bad.get("error", {}).get("code") == "INVALID_PARAMETER", (st, bad))
st, bad = call("POST", "/listings", {"listing": dict(listing["listing"], Vertical="RETAIL", ID=None)}, token=seller_token)
check("V: vertical that does not accept the listing type is refused (400)", st == 400, (st, bad))
check("V: refused listings were not written",
      sql(f"SELECT count(*) FROM listings WHERE organization_id='{org}'") == "2")
st, res = call("POST", "/search", {"text": "Riverside apartment"})
check("E: draft listing not discoverable", st == 200 and all(i.get("ListingID") != lid for i in res.get("items", [])), res)
st, _ = call("POST", f"/listings/publish?listing_id={lid}", token=customer_token)
check("E: customer cannot publish seller's listing", st >= 400, st)
st, pub = call("POST", f"/listings/publish?listing_id={lid}", token=seller_token)
check("E: seller publishes listing", st == 200, (st, pub))
found = False
for _ in range(15):
    time.sleep(1)
    st, res = call("POST", "/search", {"text": "Riverside apartment"})
    if st == 200 and any(i.get("ListingID") == lid for i in res.get("items", [])):
        found = True; break
check("E: published listing projected to OpenSearch and discoverable", found, res)

# ---------- Journey A: Discovery (guest) ----------
st, res = call("POST", "/search", {"text": "apartment gombe"})
check("A: guest search returns results without signing in", st == 200 and res.get("total", 0) >= 1, (st, res))
st, sug = call("GET", "/search/suggest?text=River")
check("A: suggest served (guest)", st == 200 and "suggestions" in sug, (st, sug))
st, det = call("GET", f"/listings?listing_id={lid}")
check("A: guest views published listing detail", st == 200 and det.get("ID") == lid, (st, det))

# ---------- Journey B: Conversation ----------
sender = str(uuid.uuid4())
st, conv = call("POST", "/conversations/message", {"channel": "web", "message_text": "find an apartment in Gombe",
                                                     "sender_external_id": sender, "message_idempotency_key": "m1",
                                                     "actor": {"id": seller_id, "type": "platform_operator"}})
text = conv.get("response", {}).get("text", "") if isinstance(conv, dict) else ""
check("B: guest conversation -> intent -> search -> results", st == 200 and "Riverside apartment Gombe" in text, (st, conv))
cid = conv.get("conversation_id")
check("B: conversation created with UUID", bool(re.match(r"^[0-9a-f-]{36}$", cid or "")), cid)
check("B: result cards returned for web", len(conv.get("response", {}).get("cards", [])) >= 1, conv)
st, again = call("POST", "/conversations/message", {"channel": "web", "message_text": "find an apartment in Gombe",
                                                      "sender_external_id": sender, "message_idempotency_key": "m1"})
check("B: repeated message (same key) replays reply, no re-execution", st == 200 and again["response"].get("metadata", {}).get("duplicate") is True, again)
st, pick = call("POST", "/conversations/message", {"channel": "web", "message_text": "1", "sender_external_id": sender, "message_idempotency_key": "m2"})
check("B: contextual reference '1' resolves to the listed result", st == 200 and pick["response"].get("metadata", {}).get("action_type") == "GET_LISTING", pick)
msgs = int(sql(f"SELECT COUNT(*) FROM messages WHERE conversation_id='{cid}'"))
check("B: inbound + outbound messages persisted (4 = 2 in + 2 out)", msgs == 4, msgs)
ctxrow = sql(f"SELECT COUNT(*) FROM conversation_contexts WHERE conversation_id='{cid}'")
check("B: conversation context persisted", ctxrow == "1", ctxrow)
st, book = call("POST", "/conversations/message", {"channel": "web", "message_text": "book it", "sender_external_id": sender, "message_idempotency_key": "m3"})
check("B: guest asked to sign in before booking", st == 200 and book["response"].get("metadata", {}).get("requires_authentication") is True, book)
st, hijack = call("POST", "/conversations/message", {"channel": "web", "message_text": "find cars", "sender_external_id": sender}, token=customer_token)
check("B: signed-in customer claims own guest conversation", st == 200, (st, hijack))
st, hijack2 = call("POST", "/conversations/message", {"channel": "web", "message_text": "find cars", "sender_external_id": sender}, token=seller_token)
check("B: another user cannot take over the conversation (403)", st == 403, (st, hijack2))

# ---------- Journey C: Booking ----------
start = int(time.time()) + 86400 * 3; end = start + 86400 * 2
st, av = call("GET", f"/availability?listing_id={lid}&start_at={start}&end_at={end}", token=customer_token)
check("C: availability check (single-unit listing, no inventory rows)", st == 200 and av.get("available") is True, (st, av))
st, r1 = call("POST", "/reservations", {"listing_id": lid, "start_at": start, "end_at": end, "quantity": 1, "party_size": 2},
              token=customer_token, headers={"Idempotency-Key": "book-1-" + RUN})
check("C: customer creates reservation (confirmed)", st == 201 and r1.get("status") == "confirmed", (st, r1))
st, r1b = call("POST", "/reservations", {"listing_id": lid, "start_at": start, "end_at": end, "quantity": 1, "party_size": 2},
               token=customer_token, headers={"Idempotency-Key": "book-1-" + RUN})
check("C: retried booking returns same reservation", st == 201 and r1b.get("reservation_id") == r1.get("reservation_id"), (st, r1b))
st, r2 = call("POST", "/reservations", {"listing_id": lid, "start_at": start + 3600, "end_at": end + 3600, "quantity": 1},
              token=seller_token, headers={"Idempotency-Key": "book-2-" + RUN})
check("C: overlapping booking rejected (409)", st == 409, (st, r2))
st, g = call("GET", f"/reservations/{r1['reservation_id']}", token=seller_token)
check("C: seller (org owner) can view reservation for own listing", st == 200, (st, g))
_, other_token_id = None, None
other_token, _, _ = login("+243833333333")
st, g = call("GET", f"/reservations/{r1['reservation_id']}", token=other_token)
check("C: unrelated user cannot view reservation", st in (403, 404), (st, g))
# The list is paged (the customer accumulates bookings across runs): walk the pages.
listed, offset = False, 0
while True:
    st, myres = call("GET", f"/reservations?limit=100&offset={offset}", token=customer_token)
    page = myres.get("items", []) if st == 200 else []
    if any(i.get("ID") == r1["reservation_id"] for i in page):
        listed = True
        break
    if st != 200 or len(page) < 100:
        break
    offset += 100
check("C: customer lists own reservations", st == 200 and listed, (st, offset))
st, c = call("POST", f"/reservations/{r1['reservation_id']}/cancel", token=customer_token)
check("C: customer cancels reservation", st == 200, (st, c))
st, av2 = call("GET", f"/availability?listing_id={lid}&start_at={start}&end_at={end}", token=customer_token)
check("C: cancelled slot available again", st == 200 and av2.get("available") is True, av2)
# concurrent booking race: 8 customers, same slot, exactly one wins
import threading
tokens = [login(f"+24384444{i:04d}")[0] for i in range(8)]
s2 = start + 86400 * 10
codes = []
def book(t, i):
    codes.append(call("POST", "/reservations", {"listing_id": lid, "start_at": s2, "end_at": s2 + 7200}, token=t, headers={"Idempotency-Key": f"race-{i}-{RUN}"})[0])
ths = [threading.Thread(target=book, args=(t, i)) for i, t in enumerate(tokens)]
[t.start() for t in ths]; [t.join() for t in ths]
check("C: concurrent bookings of one slot -> exactly one 201, no overbooking", codes.count(201) == 1, codes)

# ---------- Journey D: Commerce ----------
st, o = call("POST", "/orders", {"listing_id": lid, "quantity": 2}, token=customer_token, headers={"Idempotency-Key": "order-1-" + RUN})
check("D: customer (non-member) creates order on published listing", st == 201, (st, o))
check("D: order total uses exact minor units (2 x 85.50 = 17100)", o.get("total", {}).get("AmountMinor") == 17100, o)
st, o2 = call("POST", "/orders", {"listing_id": lid, "quantity": 2}, token=customer_token, headers={"Idempotency-Key": "order-1-" + RUN})
check("D: retried checkout returns same order", o2.get("order_id") == o.get("order_id"), o2)
st, mine = call("GET", "/orders", token=customer_token)
check("D: customer lists own orders (TRACK)", st == 200 and any(i.get("ID") == o["order_id"] for i in mine.get("items", [])), (st, mine))
st, theirs = call("GET", "/orders", token=other_token)
check("D: other user's order list does not include it", st == 200 and all(i.get("ID") != o["order_id"] for i in theirs.get("items", [])), theirs)
st, go = call("GET", f"/orders/{o['order_id']}", token=other_token)
check("D: other user cannot read the order", st in (403, 404), st)
st, pay = call("POST", f"/payments?organization_id={org}", {"order_id": o["order_id"], "amount_minor": 17100, "currency": "USD",
               "payment_method_type": "MOBILE_MONEY", "payment_method_id": "tok_mm_1", "idempotency_key": "pay-1-" + RUN}, token=customer_token)
check("D: payment creation reaches Payment Core; no provider -> 503 (external blocker)", st == 503 and pay["error"]["code"] == "PAYMENT_PROVIDER_UNAVAILABLE", (st, pay))
st, pay2 = call("POST", f"/payments?organization_id={org}", {"order_id": o["order_id"], "amount_minor": 17100, "currency": "USD",
                "payment_method_type": "MOBILE_MONEY", "payment_method_id": "tok_mm_1", "idempotency_key": "pay-2-" + RUN}, token=other_token)
check("D: stranger cannot pay/charge someone else's order (403)", st == 403, (st, pay2))
st, cap = call("POST", f"/payments/{uuid.uuid4()}/capture?organization_id={org}", {}, token=customer_token)
check("D: customer cannot capture payments", st in (403, 404), (st, cap))

# ---------- Journey G: WhatsApp ----------
waba = "waba-" + org[:8]
sql(f"INSERT INTO channel_accounts (id, organization_id, channel, external_id, status) VALUES (gen_random_uuid(),'{org}','whatsapp','{waba}','active')")
st, ch = call("GET", GW + "/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=" + WA_VERIFY + "&hub.challenge=12345")
check("G: Meta subscription handshake echoes challenge", st == 200 and str(ch) == "12345", (st, ch))
payload = {"entry": [{"changes": [{"value": {"wa_id": waba, "contacts": [{"wa_id": "243855555555"}],
           "messages": [{"id": "wamid.E2E-" + RUN, "from": "243855555555", "timestamp": str(int(time.time())), "type": "text", "text": {"body": "find apartment Gombe"}}]}}]}]}
raw = json.dumps(payload).encode()
st, bad = call("POST", "/webhooks/whatsapp", raw=raw, headers={"X-Hub-Signature-256": "sha256=deadbeef"})
check("G: bad HMAC signature rejected (401)", st == 401, (st, bad))
sig = "sha256=" + hmac.new(WA_SECRET, raw, hashlib.sha256).hexdigest()
st, wa = call("POST", "/webhooks/whatsapp", raw=raw, headers={"X-Hub-Signature-256": sig})
check("G: signed webhook -> user auto-provisioned -> conversation -> search", st == 200 and "Riverside apartment Gombe" in wa.get("text", ""), (st, wa))
check("G: sender provisioned as verified customer", sql("SELECT status FROM users WHERE phone='+243855555555'") == "active")
st, wa2 = call("POST", "/webhooks/whatsapp", raw=raw, headers={"X-Hub-Signature-256": sig})
n = sql(f"SELECT COUNT(*) FROM messages m JOIN conversations c ON c.id=m.conversation_id WHERE c.channel='whatsapp' AND c.external_id LIKE '{waba}:%'")
check("G: redelivered webhook does not re-process (2 messages total)", st == 200 and n == "2", (st, n))

# ---------- Seller console (server-side isolation) ----------
st, ctx = call("GET", "/sellers/me", token=seller_token)
check("SELLER: /sellers/me lists the caller's organization and role", st == 200 and any(o["organization_id"] == org and o["role"] == "owner" for o in ctx.get("organizations", [])), (st, ctx))
st, ctx2 = call("GET", "/sellers/me", token=other_token)
check("SELLER: non-member sees no organizations", st == 200 and ctx2.get("organizations") == [], (st, ctx2))
st, man = call("GET", f"/listings/manage?organization_id={org}", token=seller_token)
check("SELLER: owner lists all organization listings", st == 200 and any(i.get("ID") == lid for i in man.get("items", [])), (st, man))
st, _ = call("GET", f"/listings/manage?organization_id={org}", token=other_token)
check("SELLER: another account cannot list the organization's listings (403)", st == 403, st)
st, _ = call("GET", f"/orders/incoming?tenant_id={org}", token=other_token)
check("SELLER: another account cannot read incoming orders (403)", st == 403, st)
st, inc = call("GET", f"/orders/incoming?tenant_id={org}", token=seller_token)
check("SELLER: owner reads incoming orders", st == 200 and any(i.get("ID") == o["order_id"] for i in inc.get("items", [])), (st, inc))
st, _ = call("GET", f"/reservations/incoming?tenant_id={org}", token=seller_token)
check("SELLER: owner reads incoming reservations", st == 200, st)
st, up = call("POST", f"/listings/update?listing_id={lid}", {"title": "Hijacked"}, token=other_token)
check("SELLER: another account cannot edit the listing", st in (403, 404), (st, up))
st, up = call("POST", f"/listings/update?listing_id={lid}", {"price_amount": 99}, token=seller_token)
check("SELLER: a live listing must be paused before editing (409)", st == 409, (st, up))
call("POST", f"/listings/pause?listing_id={lid}", token=seller_token)
st, up = call("POST", f"/listings/update?listing_id={lid}", {"description": "Two bedrooms, river view"}, token=seller_token)
check("SELLER: paused listing edited; price and ownership untouched", st == 200 and up.get("PriceAmount") == 85.5 and up.get("OrganizationID") == org, (st, up))
st, _ = call("POST", f"/listings/resume?listing_id={lid}", token=seller_token)
check("SELLER: listing resumed", st == 200, st)
st, det = call("GET", f"/listings?listing_id={lid}")
check("SELLER: listing detail exposes its media list", st == 200 and isinstance(det.get("Media"), list), (st, det))

# ---------- Journey S: Stripe payments (provider emulator, signed webhooks) ----------
config = json.dumps({"settings": {"base_url": "http://127.0.0.1:12111", "capture_method": "automatic",
                                   "publishable_key": "pk_test_e2e", "webhook_secret_ref": "env:STRIPE_E2E_WEBHOOK"}})
sql(f"INSERT INTO providers (organization_id, code, name, type, status, capabilities, configuration, secret_ref, health_status) "
    f"VALUES ('{org}','stripe','Stripe','PAYMENT','ACTIVE','{{PAYMENT}}','{config}','env:STRIPE_E2E_SECRET','HEALTHY')")

def new_order(tag):
    st, od = call("POST", "/orders", {"listing_id": lid, "quantity": 1}, token=customer_token, headers={"Idempotency-Key": f"ord-{tag}-{RUN}"})
    assert st == 201, (st, od)
    return od["order_id"]

def pay(order_id, method_id, key, token=None):
    return call("POST", f"/payments?organization_id={org}", {"order_id": order_id, "amount_minor": 8550, "currency": "USD",
                "payment_method_type": "CARD", "payment_method_id": method_id, "idempotency_key": key}, token=token or customer_token)

def order_status(order_id):
    return call("GET", f"/orders/{order_id}", token=customer_token)[1].get("Status")

o_card = new_order("card")
for _ in range(20):  # the provider row is picked up by the payment service within seconds
    st, p1 = pay(o_card, "pm_card_visa", "pay-card-" + RUN)
    if st != 503:
        break
    time.sleep(1)
check("S: provider configured -> card payment captured by Stripe", st == 201 and p1.get("status") == "captured", (st, p1))
check("S: paid order moves to 'paid'", order_status(o_card) == "paid", order_status(o_card))
st, p1b = pay(o_card, "pm_card_visa", "pay-card-" + RUN)
check("S: replayed payment request returns the same payment (no double charge)", p1b.get("payment_id") == p1.get("payment_id"), p1b)

o_dec = new_order("declined")
st, pd = pay(o_dec, "pm_card_chargeDeclined", "pay-dec-" + RUN)
check("S: declined card -> payment failed, order still awaiting payment", pd.get("status") == "failed" and order_status(o_dec) in ("pending_payment", "draft"), (st, pd))

o_web = new_order("web")
st, pw = pay(o_web, "card", "pay-web-" + RUN)
ca = pw.get("client_action") or {}
check("S: browser payment returns Stripe.js client action (secret + publishable key)", st == 201 and ca.get("client_secret") and ca.get("publishable_key") == "pk_test_e2e", (st, pw))
pi = pw.get("provider_payment_id")
urllib.request.urlopen(urllib.request.Request(f"http://127.0.0.1:12111/_test/confirm/{pi}", data=b"", method="POST"), timeout=10).read()
event = json.dumps({"id": "evt_" + RUN, "object": "event", "type": "payment_intent.succeeded", "created": int(time.time()),
                    "data": {"object": {"id": pi, "object": "payment_intent", "status": "succeeded", "amount": 8550, "amount_received": 8550, "currency": "usd"}}}).encode()
def stripe_sig(secret, body, ts=None):
    ts = ts or int(time.time())
    return f"t={ts},v1=" + hmac.new(secret, f"{ts}.".encode() + body, hashlib.sha256).hexdigest()
hook = f"/webhooks/payments/stripe?organization_id={org}"
st, _ = call("POST", hook, raw=event, headers={"Stripe-Signature": stripe_sig(b"whsec_wrong", event)})
check("S: webhook with a bad signature rejected", st in (400, 401), st)
st, _ = call("POST", hook, raw=event, headers={"Stripe-Signature": stripe_sig(b"whsec_e2e", event, int(time.time()) - 900)})
check("S: replayed (stale) webhook signature rejected", st in (400, 401), st)
st, wh = call("POST", hook, raw=event, headers={"Stripe-Signature": stripe_sig(b"whsec_e2e", event)})
check("S: signed payment_intent.succeeded webhook accepted", st == 200, (st, wh))
check("S: webhook settles the browser-confirmed order to 'paid'", order_status(o_web) == "paid", order_status(o_web))
st, _ = call("POST", hook, raw=event, headers={"Stripe-Signature": stripe_sig(b"whsec_e2e", event)})
n = sql(f"SELECT COUNT(*) FROM payment_webhook_events WHERE provider_msg_id='evt_{RUN}'")
check("S: redelivered webhook acknowledged once (idempotent)", st == 200 and n == "1", (st, n))

st, _ = call("POST", f"/orders/{o_card}/confirm", token=customer_token)
check("S: customer cannot confirm their own order as seller", st in (403, 404), st)
st, cf = call("POST", f"/orders/{o_card}/confirm", token=seller_token)
check("S: seller confirms the paid order", st == 200 and order_status(o_card) == "confirmed", (st, cf))
pid = p1["payment_id"]
st, _ = call("POST", f"/payments/{pid}/refund?organization_id={org}", {"amount_minor": 1000, "currency": "USD"}, token=customer_token, headers={"Idempotency-Key": "rf-c-" + RUN})
check("S: customer cannot refund", st == 403, st)
st, rf = call("POST", f"/payments/{pid}/refund?organization_id={org}", {"amount_minor": 1000, "currency": "USD"}, token=seller_token, headers={"Idempotency-Key": "rf-1-" + RUN})
check("S: seller issues a partial refund", st in (200, 201), (st, rf))
st, rf2 = call("POST", f"/payments/{pid}/refund?organization_id={org}", {"amount_minor": 1000, "currency": "USD"}, token=seller_token, headers={"Idempotency-Key": "rf-1-" + RUN})
refunds = sql(f"SELECT COUNT(*) FROM refunds WHERE payment_id='{pid}'")
check("S: replayed refund creates no second refund", refunds == "1", (st, refunds))
st, rf3 = call("POST", f"/payments/{pid}/refund?organization_id={org}", {"amount_minor": 100000, "currency": "USD"}, token=seller_token, headers={"Idempotency-Key": "rf-2-" + RUN})
check("S: over-refund rejected", st >= 400, (st, rf3))

# ---------- Journey AI: model-interpreted conversation -> action ----------
conv_log = LOG.replace("identity.log", "conversation.log")
before = open(conv_log, encoding="utf-8", errors="ignore").read().count("AI provider call completed")
ai_sender = str(uuid.uuid4())
def say(text, key):
    return call("POST", "/conversations/message", {"channel": "web", "message_text": text, "sender_external_id": ai_sender, "message_idempotency_key": key}, token=customer_token)
st, r = say("I'm looking for a 2 bedroom apartment in Kinshasa", "ai-1")
after = open(conv_log, encoding="utf-8", errors="ignore").read().count("AI provider call completed")
check("AI: natural-language request interpreted by the model (AI gateway call recorded)", after > before, (before, after))
check("AI: interpreted as a listing search with results", st == 200 and r["response"].get("metadata", {}).get("action_type") == "SEARCH_LISTINGS" and len(r["response"].get("cards", [])) >= 1, (st, r))
ai_cid = r.get("conversation_id")
st, r = say("1", "ai-2")
check("AI: contextual reference opens the first result", st == 200 and r["response"].get("metadata", {}).get("action_type") == "GET_LISTING", (st, r))
st, r = say("buy it", "ai-3")
check("AI: 'buy it' on the listing in focus asks for the missing quantity", st == 200 and r.get("is_clarification") is True and any(p.get("name") == "quantity" for p in r.get("missing_params", [])), (st, r))
st, r = say("1", "ai-3b")
conf = (r.get("response") or {}).get("confirmation") or {}
token_c = (conf.get("details") or {}).get("confirmation_token")
check("AI: with the quantity, the order waits for explicit confirmation (nothing executed yet)", st == 200 and r.get("is_confirmation") is True and token_c, (st, r))
st, r = call("POST", f"/conversations/{ai_cid}/confirm", {"token": token_c}, token=other_token)
check("AI: another account cannot confirm this action (403)", st == 403, (st, r))
st, r = call("POST", f"/conversations/{ai_cid}/confirm", {"token": token_c}, token=customer_token)
oid = ((r.get("response") or {}).get("metadata") or {}).get("result_data", {}).get("order_id")
check("AI: confirmed -> order created through the commerce runtime", st == 200 and oid, (st, r))
st, r = call("POST", f"/conversations/{ai_cid}/confirm", {"token": token_c}, token=customer_token)
check("AI: confirmation token is single-use", st == 400, (st, r))

# ---------- Hotel Commerce Pack (H1-H5; H6 runs on real PostgreSQL in Go) ----------
import datetime as _dt

def night(i):
    return (_dt.datetime.now(_dt.timezone.utc).date() + _dt.timedelta(days=20 + i)).isoformat()

def night_ts(i):
    d = _dt.datetime.now(_dt.timezone.utc).date() + _dt.timedelta(days=20 + i)
    return int(_dt.datetime(d.year, d.month, d.day, tzinfo=_dt.timezone.utc).timestamp())

# H4 - Seller: HOTEL -> room type -> rooms -> nightly capacity -> publish -> calendar
room_title = "Deluxe room " + RUN
st, rt = call("POST", "/hotels/room-types", {"store_id": store, "seller_id": seller, "category_id": cat, "title": room_title,
              "description": "River view, breakfast included", "price_amount": 90, "price_currency": "USD", "max_occupancy": 2,
              "bed_configuration": "1 king bed", "amenities": ["Wi-Fi", "Breakfast"], "units": 3}, token=seller_token)
room = rt.get("listing_id") if isinstance(rt, dict) else None
check("H4: hotel seller creates a room type (HOTEL listing + room facts + 3 rooms)",
      st == 201 and room and rt.get("units") == 3 and rt.get("max_occupancy") == 2, (st, rt))
check("H4: the room type is a HOTEL listing with ROOM inventory in the Commerce Core",
      sql(f"SELECT l.vertical||'|'||i.resource_type||'|'||i.total FROM listings l JOIN inventory_resources i ON i.listing_id=l.id WHERE l.id='{room}'") == "HOTEL|ROOM|3")
st, cap = call("POST", f"/hotels/room-types/{room}/capacity", {"from": night(0), "to": night(5), "rooms": 3}, token=seller_token)
check("H4: seller opens 3 rooms on 5 nights", st == 200 and len(cap.get("nights", [])) == 5, (st, cap))
st, cap = call("POST", f"/hotels/room-types/{room}/capacity", {"from": night(1), "to": night(2), "rooms": 2}, token=seller_token)
check("H4: one night drops to 2 rooms (a room under repair)", st == 200 and cap["nights"][0]["capacity"] == 2, (st, cap))
st, _ = call("POST", f"/hotels/room-types/{room}/capacity", {"from": night(0), "to": night(1), "rooms": 4}, token=seller_token)
check("H4: a night cannot offer more rooms than the room type has (400)", st == 400, st)
st, pub = call("POST", f"/listings/publish?listing_id={room}", token=seller_token)
check("H4: seller publishes the room type", st == 200, (st, pub))

# H1 - Hotel discovery: HOTEL vertical -> search -> results
found = None
for _ in range(15):
    st, res = call("POST", "/search", {"vertical": "hotel", "location": "Gombe"})
    if st == 200 and any(i.get("ListingID") == room for i in res.get("items", [])):
        found = res; break
    time.sleep(1)
check("H1: search by vertical HOTEL + location Gombe finds the room type", found is not None, res)
check("H1: every result is in the HOTEL vertical", found is not None and all(i.get("Vertical") == "HOTEL" for i in found["items"]), found)
st, res = call("POST", "/search", {"vertical": "RETAIL", "text": room_title})
check("H1: the room type is not a RETAIL result", st == 200 and all(i.get("ListingID") != room for i in res.get("items", [])), res)
st, bad = call("POST", "/search", {"vertical": "SPACESHIPS"})
check("H1: unknown vertical -> 400, never OTHER", st == 400 and bad.get("error", {}).get("code") == "INVALID_VERTICAL", (st, bad))
hotel_sender = "hotel-" + RUN
st, conv = call("POST", "/conversations/message", {"channel": "web", "message_text": "Find me a hotel in Gombe for two nights for two people",
                "sender_external_id": hotel_sender, "message_idempotency_key": "h1-" + RUN}, token=customer_token)
text = (conv.get("response") or {}).get("text", "") if isinstance(conv, dict) else ""
check("H1: 'a hotel in Gombe for two nights for two people' -> vertical HOTEL, location, nights, party size",
      st == 200 and "Hotels & accommodation in Gombe · 2 nights · 2 guests" in text and room_title in text, (st, text))

# H2 - Hotel availability: property -> dates -> guests -> availability per room type
st, prop = call("GET", f"/hotels/property?store_id={store}")
check("H2: guests see the property and its published room types",
      st == 200 and any(r["listing_id"] == room for r in prop.get("room_types", [])), (st, prop))
st, quote = call("GET", f"/hotels/availability?store_id={store}&check_in={night(0)}&check_out={night(2)}&guests=2")
offer = next((o for o in quote.get("offers", []) if o["listing_id"] == room), {}) if st == 200 else {}
check("H2: two nights, two guests -> available, the tightest night decides (2 rooms left)",
      offer.get("available") is True and offer.get("available_rooms") == 2 and offer.get("nights") == 2 and offer.get("total_price") == 180, (st, offer))
st, quote = call("GET", f"/hotels/availability?store_id={store}&check_in={night(0)}&check_out={night(2)}&guests=5")
offer5 = next((o for o in quote.get("offers", []) if o["listing_id"] == room), {}) if st == 200 else {}
check("H2: five guests need three double rooms -> not available on the 2-room night", offer5.get("rooms") == 3 and offer5.get("available") is False, offer5)
st, _ = call("GET", f"/hotels/availability?store_id={store}&check_in={night(2)}&check_out={night(0)}&guests=2")
check("H2: check-out before check-in -> 400", st == 400, st)

# H3 - Hotel reservation: room -> reservation -> payment -> confirmation
def book_stay(token, first, last, rooms, guests, key, start_shift=0):
    return call("POST", "/reservations", {"listing_id": room, "start_at": night_ts(first) + start_shift, "end_at": night_ts(last),
                "quantity": rooms, "party_size": guests}, token=token, headers={"Idempotency-Key": key + RUN})
st, r1 = book_stay(customer_token, 0, 2, 1, 2, "stay-a-")
stay1 = r1.get("reservation_id") if isinstance(r1, dict) else None
check("H3: customer books a double room for 2 nights (confirmed)", st == 201 and r1.get("status") == "confirmed", (st, r1))
check("H3: the stay draws from nightly capacity (capacity-managed reservation)", sql(f"SELECT capacity_managed FROM reservations WHERE id='{stay1}'") == "t")
st, r2 = book_stay(seller_token, 0, 2, 1, 1, "stay-b-")
check("H3: a second guest books the same nights (rooms are shared, not exclusive)", st == 201, (st, r2))
st, r3 = book_stay(other_token, 1, 2, 1, 1, "stay-c-")
check("H3: the 2-room night is now full -> 409", st == 409, (st, r3))
st, r4 = book_stay(other_token, 3, 4, 1, 3, "stay-d-")
check("H3: three guests in one double room -> 400", st == 400 and r4.get("error", {}).get("code") == "INVALID_PARAMETER", (st, r4))
st, r5 = book_stay(other_token, 3, 4, 1, 1, "stay-e-", start_shift=14 * 3600)
check("H3: a stay is whole nights (check-in at 14:00 as a timestamp) -> 400", st == 400, (st, r5))
st, stay_order = call("POST", f"/reservations/{stay1}/checkout", token=customer_token)
check("H3: the stay's order = 2 nights x 1 room x $90", st == 201 and stay_order.get("total", {}).get("AmountMinor") == 18000, (st, stay_order))
st, again = call("POST", f"/reservations/{stay1}/checkout", token=customer_token)
check("H3: checkout is idempotent (same order)", st == 201 and again.get("order_id") == stay_order.get("order_id"), (st, again))
st, _ = call("POST", f"/reservations/{stay1}/checkout", token=other_token)
check("H3: another customer cannot pay (or read) my stay (403)", st == 403, st)
st, sp = call("POST", f"/payments?organization_id={org}", {"order_id": stay_order.get("order_id"), "amount_minor": 18000, "currency": "USD",
              "payment_method_type": "CARD", "payment_method_id": "pm_card_visa", "idempotency_key": "stay-pay-" + RUN}, token=customer_token)
check("H3: the stay is paid through Payment Core (Stripe emulator) -> captured", st == 201 and sp.get("status") == "captured", (st, sp))
check("H3: provider attempts are recorded with request id and attempt number",
      sql(f"SELECT count(*) FROM provider_executions WHERE organization_id='{org}' AND request_id IS NOT NULL AND attempt >= 1 AND status='SUCCESS'") not in ("", "0"))
st, cal = call("GET", f"/hotels/room-types/{room}/capacity?from={night(0)}&to={night(3)}", token=seller_token)
nights = cal.get("nights", []) if st == 200 else []
check("H3: the seller calendar shows the bookings night by night",
      [n["reserved"] for n in nights] == [2, 2, 0] and nights[1]["available"] == 0, (st, nights))
st, _ = call("POST", f"/reservations/{r2.get('reservation_id')}/cancel", token=seller_token)
st2, cal = call("GET", f"/hotels/room-types/{room}/capacity?from={night(0)}&to={night(2)}", token=seller_token)
check("H3: cancelling a stay gives its rooms back", st == 200 and [n["reserved"] for n in cal.get("nights", [])] == [1, 1], (st, cal))

# H4 (continued) - the seller cannot oversell what is booked
st, _ = call("POST", f"/hotels/room-types/{room}/capacity", {"from": night(0), "to": night(1), "rooms": 0}, token=seller_token)
check("H4: a booked night cannot be closed (409)", st == 409, st)
st, _ = call("POST", f"/hotels/room-types/{room}/units", {"units": 2}, token=seller_token)
check("H4: rooms cannot drop below an open night (409)", st == 409, st)

# H5 - Isolation: another hotel, customers and non-hotel IDs
lodge_phone = "+24381" + str(int(RUN, 16) % 10**7).zfill(7)
lodge_token, lodge_id, _ = login(lodge_phone)
st, lodge_org = call("POST", "/organizations", {"Type": "business", "LegalName": "Kin Lodge SARL", "DisplayName": "Kin Lodge",
                     "Phone": lodge_phone, "Country": "CD", "Status": "active"}, token=operator_token)
org_b = lodge_org.get("ID")
call("POST", f"/organizations/members?organization_id={org_b}", {"user_id": lodge_id, "membership_role": "owner"}, token=operator_token)
grant_role(lodge_phone, "seller")
st, sp_b = call("POST", "/sellers", {"OrganizationID": org_b, "SellerType": "organization", "DisplayName": "Kin Lodge", "Phone": lodge_phone}, token=lodge_token)
st, st_b = call("POST", "/stores", {"OrganizationID": org_b, "Name": "Kin Lodge", "Slug": "kin-lodge-" + RUN, "Country": "CD", "Phone": lodge_phone, "Status": "active"}, token=lodge_token)
st, rt_b = call("POST", "/hotels/room-types", {"store_id": st_b.get("ID"), "seller_id": sp_b.get("ID"), "category_id": cat, "title": "Lodge room " + RUN,
                "price_amount": 40, "max_occupancy": 2, "units": 2}, token=lodge_token)
room_b = rt_b.get("listing_id")
check("H5: a second hotel sets up its own room type", st == 201 and room_b, (st, rt_b))
st, _ = call("POST", f"/hotels/room-types/{room_b}/capacity", {"from": night(0), "to": night(2), "rooms": 1}, token=seller_token)
check("H5: seller A cannot change seller B's nights (403)", st == 403, st)
st, _ = call("POST", f"/hotels/room-types/{room}/units", {"units": 10}, token=lodge_token)
check("H5: seller B cannot change seller A's rooms (403)", st == 403, st)
st, _ = call("POST", f"/hotels/room-types/{room}", {"max_occupancy": 8}, token=customer_token)
check("H5: a customer cannot change a room type (403)", st == 403, st)
st, _ = call("GET", f"/hotels/room-types/{room}/capacity?from={night(0)}&to={night(1)}", token=lodge_token)
check("H5: seller B cannot read seller A's calendar (403)", st == 403, st)
st, _ = call("GET", f"/hotels/room-types?organization_id={org}", token=lodge_token)
check("H5: seller B cannot list seller A's room types (403)", st == 403, st)
st, _ = call("POST", "/hotels/room-types", {"store_id": store, "seller_id": sp_b.get("ID"), "category_id": cat, "title": "Hijack",
             "price_amount": 1, "max_occupancy": 1, "units": 1}, token=lodge_token)
check("H5: seller B cannot create a room type in seller A's hotel (403)", st == 403, st)
st, nr = call("GET", f"/hotels/room-types/{lid}/capacity?from={night(0)}&to={night(1)}", token=seller_token)
check("H5: a non-hotel listing ID is not a room type (404)", st == 404 and nr.get("error", {}).get("code") == "ROOM_TYPE_NOT_FOUND", (st, nr))
st, _ = call("GET", "/providers?organization_id=" + org, token=customer_token)
st2, _ = call("GET", "/provider-executions?organization_id=" + org, token=seller_token)
check("H5: provider and execution records are not exposed over the API", st == 404 and st2 == 404, (st, st2))

# A retail product for the browser checkout journey (retail is bought, not booked).
st, prod = call("POST", "/listings", {"listing": {"StoreID": store, "SellerID": seller, "CategoryID": cat, "Title": "Bluetooth speaker " + RUN,
                "ListingType": "physical_product", "Vertical": "RETAIL", "PricingModel": "fixed", "PriceAmount": 25, "PriceCurrency": "USD"}}, token=seller_token)
st2, _ = call("POST", f"/listings/publish?listing_id={prod.get('ID')}", token=seller_token)
check("V: seller publishes a RETAIL product", st == 201 and st2 == 200 and prod.get("Vertical") == "RETAIL", (st, st2, prod))

# ---------- Cart: several items of one store become ONE order with order lines ----------
def retail(title, price, store_id=None, currency="USD"):
    st_, body = call("POST", "/listings", {"listing": {"StoreID": store_id or store, "SellerID": seller, "CategoryID": cat, "Title": f"{title} {RUN}",
                     "ListingType": "physical_product", "Vertical": "RETAIL", "PricingModel": "fixed", "PriceAmount": price, "PriceCurrency": currency}}, token=seller_token)
    lid_ = body.get("ID") if isinstance(body, dict) else None
    st2_, _ = call("POST", f"/listings/publish?listing_id={lid_}", token=seller_token)
    return lid_ if st_ == 201 and st2_ == 200 else None

cart_a, cart_b, cart_c = prod.get("ID"), retail("Cart cable", 12.5), retail("Cart case", 4)
st, store2_body = call("POST", "/stores", {"OrganizationID": org, "Name": "Second shop", "Slug": "second-shop-" + RUN, "Country": "CD",
                                           "Phone": "+243811111111", "Status": "active"}, token=seller_token)
cart_other_store = retail("Other store item", 9, store_id=store2_body.get("ID")) if st == 201 else None
cart_other_ccy = retail("Other currency item", 9000, currency="CDF")
check("CART: three items of one store and items from another store and currency are published",
      all([cart_a, cart_b, cart_c, cart_other_store, cart_other_ccy]), (cart_a, cart_b, cart_c, cart_other_store, cart_other_ccy))

key = "cart-1-" + RUN
items = [{"listing_id": cart_a, "quantity": 2}, {"listing_id": cart_b, "quantity": 1}, {"listing_id": cart_c, "quantity": 3}]
st, co = call("POST", "/orders", {"items": items}, token=customer_token, headers={"Idempotency-Key": key})
check("CART: POST /orders with items creates ONE order (201)", st == 201 and co.get("order_id") and co.get("items") == 3, (st, co))
oid = co.get("order_id")
st, od = call("GET", f"/orders/{oid}", token=customer_token)
lines = {i["ListingID"]: i for i in od.get("Items", [])} if isinstance(od, dict) else {}
check("CART: the order holds one line per item with the requested quantities",
      len(lines) == 3 and [lines[k]["Quantity"] for k in (cart_a, cart_b, cart_c)] == [2, 1, 3], od)
check("CART: each line snapshots the unit price in minor units",
      [lines[k]["UnitPrice"]["AmountMinor"] for k in (cart_a, cart_b, cart_c)] == [2500, 1250, 400], lines)
check("CART: the order total is the sum of the lines and the order is in the listings' currency",
      od["TotalAmount"]["AmountMinor"] == 2 * 2500 + 1250 + 3 * 400 and od["TotalAmount"]["Currency"] == "USD", od.get("TotalAmount"))
check("CART: the order belongs to the customer, the store's organization and store",
      od["CustomerID"] == customer_id and od["OrganizationID"] == org and od["StoreID"] == store and od["Status"] == "draft",
      {k: od.get(k) for k in ("CustomerID", "OrganizationID", "StoreID", "Status")})
check("CART: one order row holding three order lines was written",
      sql(f"SELECT count(*) FROM orders WHERE id='{oid}'") == "1" and sql(f"SELECT jsonb_array_length(items) FROM orders WHERE id='{oid}'") == "3")
sql(f"UPDATE listings SET price_amount=99 WHERE id='{cart_a}'")
st, od2 = call("GET", f"/orders/{oid}", token=customer_token)
check("CART: a later price change does not alter the order (snapshot)", od2["TotalAmount"]["AmountMinor"] == od["TotalAmount"]["AmountMinor"], od2.get("TotalAmount"))
sql(f"UPDATE listings SET price_amount=25 WHERE id='{cart_a}'")

st, again = call("POST", "/orders", {"items": items}, token=customer_token, headers={"Idempotency-Key": key})
check("CART: replaying the same key and items returns the same order, no second order",
      st in (200, 201) and again.get("order_id") == oid and sql(f"SELECT count(*) FROM orders WHERE customer_id='{customer_id}' AND idempotency_key='{key}'") == "1", (st, again))
st, again = call("POST", "/orders", {"items": list(reversed(items))}, token=customer_token, headers={"Idempotency-Key": key})
check("CART: the same items in another order are still the same order", st in (200, 201) and again.get("order_id") == oid, (st, again))
st, conflict = call("POST", "/orders", {"items": items[:2]}, token=customer_token, headers={"Idempotency-Key": key})
check("CART: the same key with different items -> 409 IDEMPOTENCY_CONFLICT", st == 409 and conflict.get("error", {}).get("code") == "IDEMPOTENCY_CONFLICT", (st, conflict))
st, conflict = call("POST", "/orders", {"items": [dict(items[0], quantity=5)] + items[1:]}, token=customer_token, headers={"Idempotency-Key": key})
check("CART: the same key with a changed quantity -> 409", st == 409, (st, conflict))

n_orders = sql(f"SELECT count(*) FROM orders WHERE customer_id='{customer_id}'")
st, mixed = call("POST", "/orders", {"items": [items[0], {"listing_id": cart_other_store, "quantity": 1}]}, token=customer_token)
check("CART: items from two stores are refused (400 MIXED_STORES)", st == 400 and mixed.get("error", {}).get("code") == "MIXED_STORES", (st, mixed))
st, mixed = call("POST", "/orders", {"items": [items[0], {"listing_id": cart_other_ccy, "quantity": 1}]}, token=customer_token)
check("CART: items in two currencies are refused (400 MIXED_STORES)", st == 400 and mixed.get("error", {}).get("code") == "MIXED_STORES", (st, mixed))
st, bad = call("POST", "/orders", {"items": [{"listing_id": cart_a, "quantity": -1}]}, token=customer_token)
check("CART: a negative quantity is refused (400 INVALID_ORDER_ITEMS)", st == 400 and bad.get("error", {}).get("code") == "INVALID_ORDER_ITEMS", (st, bad))
st, bad = call("POST", "/orders", {"items": [{"listing_id": cart_a, "quantity": 100}]}, token=customer_token)
check("CART: more than 99 of one item is refused (400)", st == 400, (st, bad))
st, bad = call("POST", "/orders", {"items": [{"listing_id": str(uuid.uuid4()), "quantity": 1}]}, token=customer_token)
check("CART: an unknown listing is refused (404)", st == 404, (st, bad))
st, dr = call("POST", "/listings", {"listing": {"StoreID": store, "SellerID": seller, "CategoryID": cat, "Title": "Cart draft " + RUN, "ListingType": "physical_product", "Vertical": "RETAIL", "PricingModel": "fixed", "PriceAmount": 3, "PriceCurrency": "USD"}}, token=seller_token)
st, bad = call("POST", "/orders", {"items": [{"listing_id": dr.get("ID"), "quantity": 1}]}, token=customer_token)
check("CART: an unpublished listing is refused (409 LISTING_UNAVAILABLE)", st == 409 and bad.get("error", {}).get("code") == "LISTING_UNAVAILABLE", (st, bad))
st, _ = call("POST", "/orders", {"items": items})
check("CART: no token -> 401", st == 401, st)
check("CART: refused carts wrote no orders", sql(f"SELECT count(*) FROM orders WHERE customer_id='{customer_id}'") == n_orders)

st, _ = call("GET", f"/orders/{oid}", token=other_token)
check("CART: another customer cannot read the order", st in (403, 404), st)
st, inc = call("GET", f"/orders/incoming?tenant_id={org}", token=seller_token)
check("CART: the seller sees the order in their incoming orders", st == 200 and any(i.get("ID") == oid for i in inc.get("items", [])), (st, inc))
st, _ = call("GET", f"/orders/incoming?tenant_id={org}", token=other_token)
check("CART: another user cannot read the seller's incoming orders", st in (403, 404), st)
st, _ = call("POST", f"/orders/{oid}/confirm", token=customer_token)
check("CART: the buyer cannot confirm their own cart order", st in (403, 409), st)

# ---------- Security spot checks ----------
st, _ = call("POST", "/orders?user_id=" + customer_id, {"listing_id": lid, "quantity": 1})
check("SEC: forged ?user_id without token -> 401", st == 401, st)
st, _ = call("GET", "/inventory?tenant_id=" + org, token=customer_token)
check("SEC: inventory requires org staff (403 for customer)", st == 403, st)
st, _ = call("GET", "/inventory?tenant_id=" + org, token=seller_token)
check("SEC: org owner can read inventory", st == 200, st)
st, _ = call("POST", "/packs/booking/enable?org_id=" + org, {}, token=seller_token)
check("SEC: pack activation restricted to platform operators", st == 403, st)
big = b'{"message_text":"' + b"a" * (2 * 1024 * 1024) + b'"}'
try:
    st, _ = call("POST", "/conversations/message", raw=big)
except (ConnectionAbortedError, ConnectionResetError, urllib.error.URLError):
    st = 413  # gateway replied 413 and closed before the upload finished
check("SEC: oversized body rejected (413)", st == 413, st)

passed = sum(1 for _, ok in results if ok)
print(f"\n{passed}/{len(results)} checks passed")
sys.exit(0 if passed == len(results) else 1)
