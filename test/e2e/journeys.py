"""Real-stack E2E journeys through the public gateway (http://localhost:8080/api/v1).

PostgreSQL (trezutil_e2e) and OpenSearch 2.17.1 are real. OTP codes are read
from the identity-service development log (APP_ENV=development); WhatsApp
webhooks are signed locally with the test app secret. No payment provider is
registered, so payment creation is expected to return 503.
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
st, myres = call("GET", "/reservations", token=customer_token)
check("C: customer lists own reservations", st == 200 and any(i.get("ID") == r1["reservation_id"] for i in myres.get("items", [])), (st, myres))
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
