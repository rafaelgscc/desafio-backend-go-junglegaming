package httpadapter

import "net/http"

import platformauth "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/auth"

func NewRouter(
	openWalletHandler *OpenWalletHandler,
	healthHandler *HealthHandler,
	wagerTransactionHandler *WagerTransactionHandler,
	authMiddleware *platformauth.Middleware,
) http.Handler {
	router := http.NewServeMux()
	router.Handle("POST /wallets", authMiddleware.RequireInternal(openWalletHandler))
	router.HandleFunc("GET /health/live", healthHandler.Live)
	router.HandleFunc("GET /health/ready", healthHandler.Ready)
	router.Handle(
		"POST /wagering/transactions",
		authMiddleware.RequireProvider(wagerTransactionHandler),
	)

	return router
}
