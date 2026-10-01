package httpadapter

import (
	"go.uber.org/fx"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
)

var Module = fx.Module(
	"http",
	fx.Provide(
		config.LoadHTTPConfig,
		provideOpenWalletExecutor,
		NewRandomIDGenerator,
		NewSystemClock,
		NewOpenWalletHandler,
		NewHealthHandler,
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

func provideOpenWalletExecutor(
	useCase *application.OpenWalletUseCase,
) OpenWalletExecutor {
	return useCase
}
