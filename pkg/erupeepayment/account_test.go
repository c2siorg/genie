package erupeepayment

import (
	"testing"
)

func TestCreateAccountSuccess(t *testing.T) {
	mgr := NewInMemoryAccountManager(nil)
	acct, err := mgr.CreateAccount("holder_001", TypePersonal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acct.ID == "" || acct.HolderID != "holder_001" {
		t.Errorf("account creation failed: %+v", acct)
	}
	if acct.BalancePaise != 0 || acct.Status != StatusActive {
		t.Errorf("account initial state incorrect: balance=%d, status=%s", acct.BalancePaise, acct.Status)
	}
}

func TestCreateAccountEmptyHolderID(t *testing.T) {
	mgr := NewInMemoryAccountManager(nil)
	_, err := mgr.CreateAccount("", TypePersonal)
	if err == nil {
		t.Errorf("expected error for empty holder ID")
	}
}

func TestCreateAccountInvalidType(t *testing.T) {
	mgr := NewInMemoryAccountManager(nil)
	_, err := mgr.CreateAccount("holder_001", "invalid")
	if err == nil {
		t.Errorf("expected error for invalid account type")
	}
}

func TestCreateMerchantAccount(t *testing.T) {
	mgr := NewInMemoryAccountManager(nil)
	acct, err := mgr.CreateAccount("merchant_001", TypeMerchant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acct.Type != TypeMerchant {
		t.Errorf("account type mismatch: got %s, want %s", acct.Type, TypeMerchant)
	}
}

func TestGetAccount(t *testing.T) {
	mgr := NewInMemoryAccountManager(nil)
	acct1, _ := mgr.CreateAccount("holder_001", TypePersonal)
	acct2 := mgr.GetAccount(acct1.ID)
	if acct2 == nil {
		t.Errorf("account retrieval failed")
	}
	if acct2.HolderID != "holder_001" {
		t.Errorf("account mismatch: got %s", acct2.HolderID)
	}
}

func TestGetAccountNotFound(t *testing.T) {
	mgr := NewInMemoryAccountManager(nil)
	acct := mgr.GetAccount("nonexistent")
	if acct != nil {
		t.Errorf("expected nil for nonexistent account, got %+v", acct)
	}
}

func TestUpdateBalancePositive(t *testing.T) {
	mgr := NewInMemoryAccountManager(nil)
	acct, _ := mgr.CreateAccount("holder_001", TypePersonal)
	err := mgr.UpdateBalance(acct.ID, 10_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updated := mgr.GetAccount(acct.ID)
	if updated.BalancePaise != 10_000 {
		t.Errorf("balance not updated: got %d, want 10000", updated.BalancePaise)
	}
}

func TestUpdateBalanceNegative(t *testing.T) {
	mgr := NewInMemoryAccountManager(nil)
	acct, _ := mgr.CreateAccount("holder_001", TypePersonal)
	mgr.UpdateBalance(acct.ID, 10_000)
	err := mgr.UpdateBalance(acct.ID, -5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updated := mgr.GetAccount(acct.ID)
	if updated.BalancePaise != 5_000 {
		t.Errorf("balance not updated: got %d, want 5000", updated.BalancePaise)
	}
}

func TestUpdateBalanceInsufficientFunds(t *testing.T) {
	mgr := NewInMemoryAccountManager(nil)
	acct, _ := mgr.CreateAccount("holder_001", TypePersonal)
	mgr.UpdateBalance(acct.ID, 5_000)
	err := mgr.UpdateBalance(acct.ID, -10_000)
	if err == nil {
		t.Errorf("expected error for insufficient balance")
	}
	// Verify balance unchanged
	updated := mgr.GetAccount(acct.ID)
	if updated.BalancePaise != 5_000 {
		t.Errorf("balance should be unchanged after failed update: got %d", updated.BalancePaise)
	}
}

func TestUpdateBalanceAccountNotFound(t *testing.T) {
	mgr := NewInMemoryAccountManager(nil)
	err := mgr.UpdateBalance("nonexistent", 1_000)
	if err == nil {
		t.Errorf("expected error for nonexistent account")
	}
}

func TestUpdateStatus(t *testing.T) {
	mgr := NewInMemoryAccountManager(nil)
	acct, _ := mgr.CreateAccount("holder_001", TypePersonal)
	err := mgr.UpdateStatus(acct.ID, StatusFrozen)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updated := mgr.GetAccount(acct.ID)
	if updated.Status != StatusFrozen {
		t.Errorf("status not updated: got %s, want %s", updated.Status, StatusFrozen)
	}
}

func TestUpdateStatusInvalid(t *testing.T) {
	mgr := NewInMemoryAccountManager(nil)
	acct, _ := mgr.CreateAccount("holder_001", TypePersonal)
	err := mgr.UpdateStatus(acct.ID, "invalid")
	if err == nil {
		t.Errorf("expected error for invalid status")
	}
}

func TestUpdateStatusAccountNotFound(t *testing.T) {
	mgr := NewInMemoryAccountManager(nil)
	err := mgr.UpdateStatus("nonexistent", StatusFrozen)
	if err == nil {
		t.Errorf("expected error for nonexistent account")
	}
}

func TestGetAccountIsCopy(t *testing.T) {
	mgr := NewInMemoryAccountManager(nil)
	acct, _ := mgr.CreateAccount("holder_001", TypePersonal)
	mgr.UpdateBalance(acct.ID, 10_000)
	retrieved1 := mgr.GetAccount(acct.ID)
	retrieved1.BalancePaise = 99_999 // modify the copy
	retrieved2 := mgr.GetAccount(acct.ID)
	if retrieved2.BalancePaise != 10_000 {
		t.Errorf("account isolation broken: got %d, want 10000", retrieved2.BalancePaise)
	}
}

func TestConcurrentAccountCreation(t *testing.T) {
	mgr := NewInMemoryAccountManager(nil)
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			_, err := mgr.CreateAccount("holder_"+string(rune(idx)), TypePersonal)
			if err != nil {
				t.Errorf("concurrent creation failed: %v", err)
			}
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestConcurrentBalanceUpdates(t *testing.T) {
	mgr := NewInMemoryAccountManager(nil)
	acct, _ := mgr.CreateAccount("holder_001", TypePersonal)
	mgr.UpdateBalance(acct.ID, 100_000)

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			mgr.UpdateBalance(acct.ID, -1_000)
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	final := mgr.GetAccount(acct.ID)
	if final.BalancePaise != 90_000 {
		t.Errorf("concurrent updates failed: got %d, want 90000", final.BalancePaise)
	}
}
