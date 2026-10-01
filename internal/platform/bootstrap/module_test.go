package bootstrap

import (
	"net/http"
	"testing"

	"go.uber.org/fx"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
)

type applicationDependencies struct {
	fx.In

	OpenWallet             *application.OpenWalletUseCase
	SubmitWagerTransaction *application.SubmitWagerTransactionUseCase
	ProcessBet             *application.ProcessBetUseCase
	ProcessWin             *application.ProcessWinUseCase
	ProcessLoss            *application.ProcessLossUseCase
	ProcessRefund          *application.ProcessReversalUseCase `name:"process_refund"`
	ProcessRollback        *application.ProcessReversalUseCase `name:"process_rollback"`
	Router                 http.Handler
}

func TestModuleProvidesApplicationGraph(t *testing.T) {
	err := fx.ValidateApp(
		Module,
		fx.Invoke(func(applicationDependencies) {}),
	)
	if err != nil {
		t.Fatalf("Fx application graph is invalid: %v", err)
	}
}
