package commerce

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestDefaultParameterValidator_NilParamsForUnknownAction(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionSearchListings, nil)
	if err == nil {
		t.Error("expected error for missing required params")
	}
}

func TestDefaultParameterValidator_MissingRequiredParam(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionSearchListings, map[string]interface{}{
		"extra": "value",
	})
	if err == nil {
		t.Error("expected error for missing text param")
	}
}

func TestDefaultParameterValidator_EmptyStringParam(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionGetListing, map[string]interface{}{
		"listing_id": "",
	})
	if err == nil {
		t.Error("expected error for empty listing_id")
	}
}

func TestDefaultParameterValidator_ZeroIntParam(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionCreateOrder, map[string]interface{}{
		"listing_id": "l1",
		"quantity":   0,
	})
	if err == nil {
		t.Error("expected error for zero quantity")
	}
}

func TestDefaultParameterValidator_ValidParams(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionSearchListings, map[string]interface{}{
		"text": "test",
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestDefaultParameterValidator_UnknownAction(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionCheckInventory, map[string]interface{}{})
	if err != nil {
		t.Errorf("expected no error for unknown action, got %v", err)
	}
}

func TestDefaultParameterValidator_NilParamsBecomesEmpty(t *testing.T) {
	v := NewDefaultParameterValidator()
	// ActionSearchListings requires "text" - with nil map it should fail with "missing" not "empty"
	err := v.Validate(context.Background(), domain.ActionSearchListings, nil)
	if err == nil {
		t.Error("expected error for nil params on action with required params")
	}
}

func TestDefaultParameterValidator_PublishListing_Missing(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionPublishListing, map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing listing_id")
	}
}

func TestDefaultParameterValidator_PublishListing_Valid(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionPublishListing, map[string]interface{}{
		"listing_id": "l1",
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestDefaultParameterValidator_PauseListing_Missing(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionPauseListing, map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing listing_id")
	}
}

func TestDefaultParameterValidator_PauseListing_Valid(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionPauseListing, map[string]interface{}{
		"listing_id": "l1",
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestDefaultParameterValidator_CreateListing_Missing(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionCreateListing, map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing listing")
	}
}

func TestDefaultParameterValidator_CreateListing_NilListing(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionCreateListing, map[string]interface{}{
		"listing": nil,
	})
	if err == nil {
		t.Error("expected error for nil listing")
	}
}

func TestDefaultParameterValidator_CreateListing_Valid(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionCreateListing, map[string]interface{}{
		"listing": &domain.Listing{},
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestDefaultParameterValidator_CreateOrder_Missing(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionCreateOrder, map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing params")
	}
}

func TestDefaultParameterValidator_CreateOrder_ZeroQuantity(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionCreateOrder, map[string]interface{}{
		"listing_id": "l1",
		"quantity":   0,
	})
	if err == nil {
		t.Error("expected error for zero quantity")
	}
}

func TestDefaultParameterValidator_CreateOrder_Valid(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionCreateOrder, map[string]interface{}{
		"listing_id": "l1",
		"quantity":   2,
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestDefaultParameterValidator_CancelOrder_Missing(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionCancelOrder, map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing order_id")
	}
}

func TestDefaultParameterValidator_CancelOrder_Valid(t *testing.T) {
	v := NewDefaultParameterValidator()
	err := v.Validate(context.Background(), domain.ActionCancelOrder, map[string]interface{}{
		"order_id": "o1",
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
