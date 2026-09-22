package domain

import "testing"

func TestAIModel_IsActive(t *testing.T) {
	m := AIModel{Status: AIModelStatusActive}
	if !m.IsActive() {
		t.Fatal("expected ACTIVE model to be active")
	}
	m.Status = AIModelStatusInactive
	if m.IsActive() {
		t.Fatal("expected INACTIVE model to be inactive")
	}
}

func TestAIModel_SupportsCapability(t *testing.T) {
	m := AIModel{Capabilities: []AIModelCapability{AIModelCapabilityTextGeneration, AIModelCapabilityStructuredOutput}}

	if !m.SupportsCapability(AIModelCapabilityTextGeneration) {
		t.Fatal("expected model to support TEXT_GENERATION")
	}
	if !m.SupportsCapability(AIModelCapabilityStructuredOutput) {
		t.Fatal("expected model to support STRUCTURED_OUTPUT")
	}
	if m.SupportsCapability(AIModelCapabilityVision) {
		t.Fatal("expected model to NOT support VISION")
	}
}

func TestAITaskType_RequiredCapability(t *testing.T) {
	tests := []struct {
		task     AITaskType
		expected AIModelCapability
	}{
		{AITaskIntentClassification, AIModelCapabilityTextClassification},
		{AITaskParameterExtraction, AIModelCapabilityStructuredOutput},
		{AITaskResponseGeneration, AIModelCapabilityTextGeneration},
		{AITaskType("UNKNOWN"), ""},
	}

	for _, tc := range tests {
		got := tc.task.RequiredCapability()
		if got != tc.expected {
			t.Errorf("task %s: expected %s, got %s", tc.task, tc.expected, got)
		}
	}
}

func TestAITaskType_RequiresStructuredOutput(t *testing.T) {
	if !AITaskIntentClassification.RequiresStructuredOutput() {
		t.Fatal("expected INTENT_CLASSIFICATION to require structured output")
	}
	if !AITaskParameterExtraction.RequiresStructuredOutput() {
		t.Fatal("expected PARAMETER_EXTRACTION to require structured output")
	}
	if AITaskResponseGeneration.RequiresStructuredOutput() {
		t.Fatal("expected RESPONSE_GENERATION to NOT require structured output")
	}
}

func TestAIIntent_Validate(t *testing.T) {
	intent := NewAIIntent()
	intent.Intent = ActionSearchListings
	intent.Confidence = 0.95
	intent.Parameters = map[string]interface{}{"query": "test"}

	if err := intent.Validate(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	intent.Intent = ActionInitiatePayment
	if err := intent.Validate(); err == nil {
		t.Fatal("expected error for non-allowlisted action")
	}

	intent.Intent = ActionSearchListings
	intent.Confidence = 1.5
	if err := intent.Validate(); err == nil {
		t.Fatal("expected error for invalid confidence")
	}

	intent.Confidence = 0.95
	intent.Parameters["actor_id"] = "hacker"
	if err := intent.Validate(); err == nil {
		t.Fatal("expected error for untrusted field")
	}
}

func TestAIIntent_HasUntrustedFields(t *testing.T) {
	tests := []struct {
		field string
		value interface{}
		want  bool
	}{
		{"query", "test", false},
		{"listing_id", "abc", false},
		{"actor_id", "x", true},
		{"tenant_id", "y", true},
		{"user_id", "z", true},
		{"organization_id", "w", true},
		{"role", "admin", true},
		{"permissions", []string{"all"}, true},
		{"confirmation_state", "approved", true},
		{"secret_ref", "key", true},
	}

	for _, tc := range tests {
		intent := NewAIIntent()
		intent.Parameters[tc.field] = tc.value
		if got := intent.HasUntrustedFields(); got != tc.want {
			t.Errorf("field %s: expected %v, got %v", tc.field, tc.want, got)
		}
	}
}

func TestAIIntent_IsAllowlisted(t *testing.T) {
	allowlisted := []CommerceActionType{
		ActionSearchListings,
		ActionGetListing,
		ActionCompareListings,
		ActionCheckAvailability,
		ActionGetOrder,
		ActionGetReservation,
	}

	for _, a := range allowlisted {
		intent := NewAIIntent()
		intent.Intent = a
		if !intent.IsAllowlisted() {
			t.Errorf("expected %s to be allowlisted", a)
		}
	}

	nonAllowlisted := []CommerceActionType{
		ActionCreateListing,
		ActionCreateOrder,
		ActionInitiatePayment,
		ActionCreateReservation,
	}

	for _, a := range nonAllowlisted {
		intent := NewAIIntent()
		intent.Intent = a
		if intent.IsAllowlisted() {
			t.Errorf("expected %s to NOT be allowlisted", a)
		}
	}
}

func TestAIIntent_ConfidenceOK(t *testing.T) {
	intent := NewAIIntent()
	intent.Confidence = 0.8

	if !intent.ConfidenceOK(0.7) {
		t.Fatal("expected confidence 0.8 to pass threshold 0.7")
	}
	if intent.ConfidenceOK(0.9) {
		t.Fatal("expected confidence 0.8 to fail threshold 0.9")
	}
}

func TestAIError_IsRetryable(t *testing.T) {
	tests := []struct {
		code      AIErrorCode
		retryable bool
	}{
		{AIErrProviderUnavailable, true},
		{AIErrProviderTimeout, true},
		{AIErrProviderRateLimited, true},
		{AIErrUnknown, true},
		{AIErrProviderAuthFailed, false},
		{AIErrInvalidResponse, false},
		{AIErrSchemaValidationFailed, false},
		{AIErrUnsupportedTask, false},
		{AIErrModelUnavailable, false},
		{AIErrContextTooLarge, false},
	}

	for _, tc := range tests {
		err := NewAIError(tc.code, "test")
		if err.IsRetryable() != tc.retryable {
			t.Errorf("code %s: expected retryable=%v, got %v", tc.code, tc.retryable, err.IsRetryable())
		}
	}
}

func TestAIProviderEntity_Lifecycle(t *testing.T) {
	p := NewAIProviderEntity("openai", "OpenAI", "openai")
	if p.Status != AIProviderStatusActive {
		t.Fatalf("expected default status ACTIVE, got %s", p.Status)
	}
	if !p.IsUsable() {
		t.Fatal("expected new provider to be usable")
	}

	if err := p.Disable(); err != nil {
		t.Fatalf("expected no error on disable, got: %v", err)
	}
	if p.Status != AIProviderStatusInactive {
		t.Fatalf("expected INACTIVE, got %s", p.Status)
	}
	if p.IsUsable() {
		t.Fatal("expected disabled provider to not be usable")
	}

	if err := p.Enable(); err != nil {
		t.Fatalf("expected no error on enable, got: %v", err)
	}
	if p.Status != AIProviderStatusActive {
		t.Fatalf("expected ACTIVE, got %s", p.Status)
	}

	if err := p.Enable(); err == nil {
		t.Fatal("expected error enabling already-active provider")
	}
}

func TestAIProviderEntity_Validation(t *testing.T) {
	tests := []struct {
		name    string
		entity  *AIProviderEntity
		wantErr bool
	}{
		{"valid", NewAIProviderEntity("code", "Name", "type"), false},
		{"empty code", &AIProviderEntity{Code: "", Name: "Test", Type: "type"}, true},
		{"empty name", &AIProviderEntity{Code: "code", Name: "", Type: "type"}, true},
		{"empty type", &AIProviderEntity{Code: "code", Name: "Test", Type: ""}, true},
		{"invalid status", &AIProviderEntity{Code: "code", Name: "Test", Type: "type", Status: "UNKNOWN"}, true},
	}

	for _, tc := range tests {
		err := tc.entity.Validate()
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: expected error=%v, got: %v", tc.name, tc.wantErr, err)
		}
	}
}
