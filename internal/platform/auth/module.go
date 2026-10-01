package auth

import (
	"go.uber.org/fx"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
)

var Module = fx.Module(
	"auth",
	fx.Provide(
		config.LoadOIDCConfig,
		fx.Annotate(
			NewOIDCTokenVerifier,
			fx.As(new(TokenVerifier)),
		),
		NewMiddleware,
	),
)
