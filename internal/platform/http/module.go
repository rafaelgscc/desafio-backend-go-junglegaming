package httpadapter

import (
	"go.uber.org/fx"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	platformauth "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/auth"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
)

var Module = fx.Module(
	"http",
	platformauth.Module,
	fx.Provide(
		config.LoadHTTPConfig,
		provideOpenWalletExecutor,
		provideExecuteWagerTransactionExecutor,
		provideWageringQueryExecutor,
		NewRandomIDGenerator,
		NewSystemClock,
		NewOpenWalletHandler,
		NewHealthHandler,
		NewWagerTransactionHandler,
		NewWageringQueryHandler,
		NewRouter,
		NewServer,
	),
	fx.Invoke(registerServerLifecycle),
)

func registerServerLifecycle(lifecycle fx.Lifecycle, server *Server) {
	lifecycle.Append(fx.Hook{
		OnStart: server.Start,
		OnStop:  server.Stop,
	})
}

func provideWageringQueryExecutor(
	service *application.WageringQueryService,
) WageringQueryExecutor {
	return service
}

func provideOpenWalletExecutor(
	useCase *application.OpenWalletUseCase,
) OpenWalletExecutor {
	return useCase
}

func provideExecuteWagerTransactionExecutor(
	useCase *application.ExecuteWagerTransactionUseCase,
) ExecuteWagerTransactionExecutor {
	return useCase
}
