package httpadapter

import "net/http"

func NewRouter(
	openWalletHandler *OpenWalletHandler,
	healthHandler *HealthHandler,
) http.Handler {
	router := http.NewServeMux()
	router.Handle("POST /wallets", openWalletHandler)
	router.HandleFunc("GET /health/live", healthHandler.Live)
	router.HandleFunc("GET /health/ready", healthHandler.Ready)

	return router
}
