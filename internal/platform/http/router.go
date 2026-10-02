package httpadapter

import "net/http"

import platformauth "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/auth"
import "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/observability"

func NewRouter(
	openWalletHandler *OpenWalletHandler,
	healthHandler *HealthHandler,
	wagerTransactionHandler *WagerTransactionHandler,
	wageringQueryHandler *WageringQueryHandler,
	authMiddleware *platformauth.Middleware,
	metrics *observability.Metrics,
) http.Handler {
	router := http.NewServeMux()
	router.Handle("POST /wallets", authMiddleware.RequireInternal(openWalletHandler))
	router.HandleFunc("GET /health/live", healthHandler.Live)
	router.HandleFunc("GET /health/ready", healthHandler.Ready)
	router.Handle("GET /metrics", metrics)
	router.Handle(
		"POST /wagering/transactions",
		authMiddleware.RequireProvider(wagerTransactionHandler),
	)
	router.Handle("GET /wallets/{walletId}", authMiddleware.RequireInternal(http.HandlerFunc(wageringQueryHandler.GetWallet)))
	router.Handle("GET /wallets/{walletId}/ledger", authMiddleware.RequireInternal(http.HandlerFunc(wageringQueryHandler.ListWalletLedger)))
	router.Handle("POST /wallets/{walletId}/reconciliation", authMiddleware.RequireInternal(http.HandlerFunc(wageringQueryHandler.ReconcileWallet)))
	router.Handle("GET /wagering/transactions/{transactionId}", authMiddleware.RequireProvider(http.HandlerFunc(wageringQueryHandler.GetWagerTransaction)))
	router.Handle("GET /providers/{providerId}/wagering/transactions/{externalTransactionId}", authMiddleware.RequireProvider(http.HandlerFunc(wageringQueryHandler.GetWagerTransactionByExternalID)))

	return router
}
