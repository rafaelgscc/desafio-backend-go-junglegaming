package application

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/domain"
)

func TestIntegrationEventPayloadsUsePublishedCamelCaseContract(t *testing.T) {
	money, _ := domain.NewMoney("25.00", "BRL")
	zero, _ := domain.ZeroMoney("BRL")
	now := time.Date(2026, time.October, 2, 12, 0, 0, 0, time.UTC)
	testCases := []struct {
		name string
		data any
		keys []string
	}{
		{
			name: "processed",
			data: WagerTransactionProcessedData{
				TransactionID: "transaction-1", ProviderID: "provider-1",
				ExternalTransactionID: "external-1", Kind: domain.WagerTransactionKindBet,
				Money: money, Balance: money,
			},
			keys: []string{"transactionId", "providerId", "externalTransactionId", "kind", "money", "balance"},
		},
		{
			name: "rejected",
			data: WagerTransactionRejectedData{
				TransactionID: "transaction-1", ProviderID: "provider-1",
				ExternalTransactionID: "external-1", Kind: domain.WagerTransactionKindBet,
				Money: money, FailureCode: domain.WagerTransactionFailureCodeInsufficientFunds,
			},
			keys: []string{"transactionId", "providerId", "externalTransactionId", "kind", "money", "failureCode"},
		},
		{
			name: "balance changed",
			data: WalletBalanceChangedData{
				WalletID: "wallet-1", TransactionID: "transaction-1",
				Direction: domain.LedgerDirectionCredit, Money: money,
				BalanceBefore: zero, BalanceAfter: money, WalletVersion: 2,
			},
			keys: []string{"walletId", "transactionId", "direction", "money", "balanceBefore", "balanceAfter", "walletVersion"},
		},
		{
			name: "pending reference",
			data: WagerTransactionPendingReferenceData{
				TransactionID: "transaction-1", ProviderID: "provider-1",
				ExternalTransactionID:          "external-1",
				ReferenceExternalTransactionID: "external-bet-1",
				ReferenceAttempts:              1, NextReferenceAttemptAt: now,
			},
			keys: []string{
				"transactionId", "providerId", "externalTransactionId",
				"referenceExternalTransactionId", "referenceAttempts", "nextReferenceAttemptAt",
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			encoded, err := json.Marshal(testCase.data)
			if err != nil {
				t.Fatal(err)
			}
			var object map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &object); err != nil {
				t.Fatal(err)
			}
			for _, key := range testCase.keys {
				if _, ok := object[key]; !ok {
					t.Errorf("payload %s does not contain %q", encoded, key)
				}
			}
			if strings.Contains(string(encoded), "_id") || strings.Contains(string(encoded), "_at") ||
				strings.Contains(string(encoded), "_version") || strings.Contains(string(encoded), "_code") {
				t.Errorf("payload uses snake_case: %s", encoded)
			}
		})
	}
}
