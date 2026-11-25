package saspay

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

func fakeSasPay(t *testing.T, handle func(w http.ResponseWriter, r *http.Request, body map[string]interface{})) *Adapter {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk_test_abc" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"success":false,"error":{"message":"bad key","code":"invalid_api_key"},"code":401}`)
			return
		}
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		handle(w, r, body)
	}))
	t.Cleanup(srv.Close)
	a, err := NewAdapter(Config{ProviderID: "p1", SecretKey: "sk_test_abc", BaseURL: srv.URL, ReturnURL: "https://trezutil.com/"})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

var payer = map[string]interface{}{"phone": "0817 12 72 53", "email": "a@b.cd", "first_name": "Gilio", "last_name": "Kapapa"}

func TestPushesAMobileMoneyPaymentAsSasPayDocumentsIt(t *testing.T) {
	var got map[string]interface{}
	var key, path string
	a := fakeSasPay(t, func(w http.ResponseWriter, r *http.Request, body map[string]interface{}) {
		got, key, path = body, r.Header.Get("Idempotency-Key"), r.Method+" "+r.URL.Path
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"success":true,"data":{"message":"Payment pushed successfully","id":"pay-1","status":"PENDING","checkout_url":"","instructions":""},"code":201}`)
	})
	resp, err := a.CreatePayment(context.Background(), domain.PaymentProviderRequest{
		Amount: domain.Money{AmountMinor: 50000, Currency: "CDF"}, PaymentMethodID: "vodacom_cd", IdempotencyKey: "idem-1", Metadata: payer,
	})
	if err != nil {
		t.Fatal(err)
	}
	if path != "POST /payments/softpay/" || key != "idem-1" {
		t.Errorf("called %s with key %q", path, key)
	}
	c, _ := got["customer"].(map[string]interface{})
	if got["amount"] != "500.00" || got["currency"] != "CDF" || got["country"] != "CD" || got["network"] != "vodacom_cd" || c["phone"] != "+243817127253" || c["email"] != "a@b.cd" {
		t.Errorf("request %v", got)
	}
	if resp.ProviderPaymentID != "pay-1" || resp.Status != "pending" || resp.Data["provider_code"] != "saspay" {
		t.Errorf("response %+v", resp)
	}
}

func TestACardOpensSasPaysPageInUSDAndComesBackToTrezutil(t *testing.T) {
	var got map[string]interface{}
	a := fakeSasPay(t, func(w http.ResponseWriter, r *http.Request, body map[string]interface{}) {
		got = body
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"success":true,"data":{"id":"pay-2","status":"PENDING","checkout_url":"https://pay.saspay.me/checkout/x"},"code":201}`)
	})
	meta := map[string]interface{}{"email": "a@b.cd", "first_name": "G", "last_name": "K", "return_url": "https://evil.example/"}
	resp, err := a.CreatePayment(context.Background(), domain.PaymentProviderRequest{Amount: domain.Money{AmountMinor: 1450, Currency: "USD"}, PaymentMethodID: "card", Metadata: meta})
	if err != nil {
		t.Fatal(err)
	}
	if got["return_url"] != "https://trezutil.com/" || got["amount"] != "14.50" {
		t.Errorf("a foreign return URL was used or the amount is wrong: %v", got)
	}
	if resp.Data["checkout_url"] != "https://pay.saspay.me/checkout/x" {
		t.Errorf("checkout url not handed to the payer: %+v", resp.Data)
	}
	if _, err := a.CreatePayment(context.Background(), domain.PaymentProviderRequest{Amount: domain.Money{AmountMinor: 1000, Currency: "CDF"}, PaymentMethodID: "card", Metadata: meta}); err == nil {
		t.Error("a card payment in CDF was sent (SasPay takes cards in USD only)")
	}
}

func TestRefusesIncompleteRequestsWithoutCallingSasPay(t *testing.T) {
	called := false
	a := fakeSasPay(t, func(w http.ResponseWriter, r *http.Request, body map[string]interface{}) { called = true })
	cases := []domain.PaymentProviderRequest{
		{Amount: domain.Money{AmountMinor: 100, Currency: "CDF"}, PaymentMethodID: "mtn_bj", Metadata: payer},
		{Amount: domain.Money{AmountMinor: 0, Currency: "CDF"}, PaymentMethodID: "airtel_cd", Metadata: payer},
		{Amount: domain.Money{AmountMinor: 100, Currency: "CDF"}, PaymentMethodID: "orange_cd", Metadata: map[string]interface{}{"phone": "0817127253"}},
	}
	for i, c := range cases {
		if _, err := a.CreatePayment(context.Background(), c); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
	if called {
		t.Error("SasPay was called for an invalid request")
	}
}

func TestReadsTheStatusBackFromVerify(t *testing.T) {
	for saspay, want := range map[string]string{"PENDING": "pending", "SUCCESS": "captured", "FAILED": "failed", "CANCELLED": "cancelled"} {
		a := fakeSasPay(t, func(w http.ResponseWriter, r *http.Request, _ map[string]interface{}) {
			if r.Method != http.MethodGet || r.URL.Path != "/payments/pay-1/verify/" {
				t.Errorf("called %s %s", r.Method, r.URL.Path)
			}
			_, _ = io.WriteString(w, `{"success":true,"data":{"id":"pay-1","status":"`+saspay+`","net_amount":"490.00","currency":"CDF"},"code":200}`)
		})
		resp, err := a.GetPayment(context.Background(), "pay-1")
		if err != nil || resp.Status != want {
			t.Errorf("%s -> %q (%v), want %q", saspay, resp.Status, err, want)
		}
	}
}

func TestClassifiesSasPayErrorsAndNeverLeaksTheKey(t *testing.T) {
	cases := map[int]domain.ProviderErrorCode{422: domain.ErrCodeInvalidRequest, 429: domain.ErrCodeRateLimited, 502: domain.ErrCodeProviderUnavailable, 409: domain.ErrCodeConflict}
	for status, want := range cases {
		a := fakeSasPay(t, func(w http.ResponseWriter, r *http.Request, _ map[string]interface{}) {
			w.WriteHeader(status)
			_, _ = io.WriteString(w, `{"success":false,"error":{"message":"x","code":"no_exchange_rate"},"code":`+strconv.Itoa(status)+`}`)
		})
		resp, err := a.CreatePayment(context.Background(), domain.PaymentProviderRequest{Amount: domain.Money{AmountMinor: 1000, Currency: "USD"}, PaymentMethodID: "vodacom_cd", Metadata: payer})
		var se *apiError
		if err == nil || resp.ErrorCode != want || !errors.As(err, &se) {
			t.Errorf("status %d -> %v %v", status, resp.ErrorCode, err)
		}
		if status == 422 && se.Code != "no_exchange_rate" {
			t.Errorf("SasPay's own code is lost: %q", se.Code)
		}
	}
	bad, _ := NewAdapter(Config{ProviderID: "p", SecretKey: "sk_live_secret123", BaseURL: fakeSasPay(t, nil).cfg.BaseURL})
	_, err := bad.GetPayment(context.Background(), "x")
	if err == nil || !strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "secret123") {
		t.Errorf("auth failure: %v", err)
	}
	for _, op := range []func() error{
		func() error { _, err := bad.CancelPayment(context.Background(), "x"); return err },
		func() error { _, err := bad.RefundPayment(context.Background(), "x", nil); return err },
	} {
		if op() == nil {
			t.Error("a cancel or refund was pretended")
		}
	}
}

func TestFormatsCongolesePhoneNumbers(t *testing.T) {
	for in, want := range map[string]string{"0817127253": "+243817127253", "+243 81 712 7253": "+243817127253", "243817127253": "+243817127253", "00243817127253": "+243817127253", "817127253": "+243817127253", "": ""} {
		if got := Phone(in); got != want {
			t.Errorf("Phone(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWebhookIsVerifiedAndMappedLikeSasPayDocumentsIt(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	n := &WebhookNormalizer{SecretFor: func(string) string { return "whsec" }, Now: func() time.Time { return now }}
	body := []byte(`{"event":"transaction.success","data":{"id":"pay-1","status":"SUCCESS","charged":"500.00","currency":"cdf"}}`)
	header := func(ts int64, b []byte) string { return "t=" + strconv.FormatInt(ts, 10) + ",v1=" + Sign("whsec", ts, b) }

	p, err := n.Normalize("org", header(now.Unix(), body), body)
	if err != nil || p == nil || p.Status != "captured" || p.ProviderPaymentID != "pay-1" || p.ProviderMessageID != "transaction.success:pay-1" || p.Currency != "CDF" {
		t.Fatalf("payload %+v, err %v", p, err)
	}
	for name, sig := range map[string]string{
		"tampered body": header(now.Unix(), []byte(`{"event":"transaction.success","data":{"id":"pay-2"}}`)),
		"too old":       header(now.Add(-10*time.Minute).Unix(), body),
		"no timestamp":  "v1=" + Sign("whsec", now.Unix(), body),
	} {
		if _, err := n.Normalize("org", sig, body); !errors.Is(err, domain.ErrWebhookSignatureInvalid) {
			t.Errorf("%s accepted: %v", name, err)
		}
	}
	created := []byte(`{"event":"transaction.created","data":{"id":"pay-1","status":"PENDING"}}`)
	if p, err := n.Normalize("org", header(now.Unix(), created), created); p != nil || err != nil {
		t.Errorf("a creation event changed something: %+v %v", p, err)
	}
	if _, err := (&WebhookNormalizer{SecretFor: func(string) string { return "" }}).Normalize("org", header(now.Unix(), body), body); !errors.Is(err, domain.ErrWebhookSignatureInvalid) {
		t.Error("accepted without a configured secret")
	}
}

func TestReadsSasPaysLiveAnswersAndFieldErrors(t *testing.T) {
	// Shapes seen in production on 2026-11-25: instructions is a list, and validation errors are per field.
	a := fakeSasPay(t, func(w http.ResponseWriter, r *http.Request, body map[string]interface{}) {
		if body["amount"] == "5.00" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"success":false,"error":{"amount":["Le montant minimum est de 799 CDF."]},"code":400}`)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"success":true,"data":{"message":"Payment pushed successfully","id":"b98","status":"PENDING","checkout_url":"","instructions":["Payment pushed successfully"],"bank_transfer":{},"amounts":{"requested":"800.00","fee":"44.00","charged":"844.00","fee_charge_mode":"ADD_ON"}},"code":201}`)
	})
	resp, err := a.CreatePayment(context.Background(), domain.PaymentProviderRequest{Amount: domain.Money{AmountMinor: 80000, Currency: "CDF"}, PaymentMethodID: "vodacom_cd", Metadata: payer})
	if err != nil || resp.ProviderPaymentID != "b98" || resp.Status != "pending" {
		t.Fatalf("a pushed payment was not read: %+v %v", resp, err)
	}
	if _, ok := resp.Data["instructions"]; ok {
		t.Errorf("SasPay's own status message was shown as an instruction: %v", resp.Data["instructions"])
	}
	if resp.Data["charged_amount"] != "844.00" || resp.Data["fee_amount"] != "44.00" {
		t.Errorf("what the payer is debited is lost: %+v", resp.Data)
	}
	_, err = a.CreatePayment(context.Background(), domain.PaymentProviderRequest{Amount: domain.Money{AmountMinor: 500, Currency: "CDF"}, PaymentMethodID: "vodacom_cd", Metadata: payer})
	if err == nil || !strings.Contains(err.Error(), "amount: Le montant minimum est de 799 CDF.") {
		t.Errorf("the field error is lost: %v", err)
	}
}
