package domain

import (
	"testing"
)

func TestNewCommerceActionResult(t *testing.T) {
	result := NewCommerceActionResult("action-1", ActionResultSuccess)

	if result.ActionID != "action-1" {
		t.Errorf("expected action-1, got %s", result.ActionID)
	}
	if result.Status != ActionResultSuccess {
		t.Errorf("expected SUCCESS, got %s", result.Status)
	}
	if result.Data == nil {
		t.Error("expected data map to be initialized")
	}
	if result.Errors == nil {
		t.Error("expected errors slice to be initialized")
	}
}

func TestCommerceActionResultAddError(t *testing.T) {
	result := NewCommerceActionResult("action-1", ActionResultValidationFailed)
	err := NewActionError("INVALID_PARAM", "Invalid parameter", "field")

	result.AddError(err)

	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	if result.Errors[0].Code != "INVALID_PARAM" {
		t.Errorf("expected INVALID_PARAM, got %s", result.Errors[0].Code)
	}
}

func TestCommerceActionResultAddData(t *testing.T) {
	result := NewCommerceActionResult("action-1", ActionResultSuccess)
	result.AddData("key", "value")

	if result.Data["key"] != "value" {
		t.Errorf("expected value, got %v", result.Data["key"])
	}
}

func TestCommerceActionResultIsSuccess(t *testing.T) {
	tests := []struct {
		status   ActionResultStatus
		expected bool
	}{
		{ActionResultSuccess, true},
		{ActionResultRequiresConfirmation, false},
		{ActionResultValidationFailed, false},
		{ActionResultUnauthorized, false},
		{ActionResultForbidden, false},
		{ActionResultNotFound, false},
		{ActionResultConflict, false},
		{ActionResultNotImplemented, false},
		{ActionResultTemporaryFailure, false},
		{ActionResultFailed, false},
	}

	for _, tt := range tests {
		result := NewCommerceActionResult("action-1", tt.status)
		if result.IsSuccess() != tt.expected {
			t.Errorf("IsSuccess() for %s: expected %v", tt.status, tt.expected)
		}
	}
}

func TestCommerceActionResultIsFailure(t *testing.T) {
	tests := []struct {
		status   ActionResultStatus
		expected bool
	}{
		{ActionResultSuccess, false},
		{ActionResultRequiresConfirmation, false},
		{ActionResultValidationFailed, true},
		{ActionResultUnauthorized, true},
		{ActionResultForbidden, true},
		{ActionResultNotFound, true},
		{ActionResultConflict, true},
		{ActionResultNotImplemented, true},
		{ActionResultTemporaryFailure, true},
		{ActionResultFailed, true},
	}

	for _, tt := range tests {
		result := NewCommerceActionResult("action-1", tt.status)
		if result.IsFailure() != tt.expected {
			t.Errorf("IsFailure() for %s: expected %v", tt.status, tt.expected)
		}
	}
}

func TestCommerceActionResultRequiresUserAction(t *testing.T) {
	tests := []struct {
		status   ActionResultStatus
		expected bool
	}{
		{ActionResultSuccess, false},
		{ActionResultRequiresConfirmation, true},
		{ActionResultValidationFailed, true},
		{ActionResultUnauthorized, false},
		{ActionResultNotFound, false},
	}

	for _, tt := range tests {
		result := NewCommerceActionResult("action-1", tt.status)
		if result.RequiresUserAction() != tt.expected {
			t.Errorf("RequiresUserAction() for %s: expected %v", tt.status, tt.expected)
		}
	}
}

func TestCommerceActionResultHasErrors(t *testing.T) {
	result := NewCommerceActionResult("action-1", ActionResultSuccess)
	if result.HasErrors() {
		t.Error("expected no errors")
	}

	result.AddError(NewActionError("ERR", "msg", ""))
	if !result.HasErrors() {
		t.Error("expected errors")
	}
}

func TestCommerceActionResultSetNextAction(t *testing.T) {
	result := NewCommerceActionResult("action-1", ActionResultSuccess)
	result.SetNextAction(ActionSearchListings, "Search more listings")

	if result.NextAction == nil {
		t.Fatal("expected next action to be set")
	}
	if result.NextAction.ActionType != ActionSearchListings {
		t.Errorf("expected SEARCH_LISTINGS, got %s", result.NextAction.ActionType)
	}
	if result.NextAction.Description != "Search more listings" {
		t.Errorf("expected description, got %s", result.NextAction.Description)
	}
}

func TestNewActionError(t *testing.T) {
	err := NewActionError("CODE", "message", "field")

	if err.Code != "CODE" {
		t.Errorf("expected CODE, got %s", err.Code)
	}
	if err.Message != "message" {
		t.Errorf("expected message, got %s", err.Message)
	}
	if err.Field != "field" {
		t.Errorf("expected field, got %s", err.Field)
	}
}
