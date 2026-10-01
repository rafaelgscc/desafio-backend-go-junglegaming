package bootstrap

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
	httpadapter "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/http"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
)

var Module = fx.Module(
	"bootstrap",
	httpadapter.Module,
	fx.Provide(
		config.LoadPostgresConfig,
		providePostgresPool,
		provideDatabaseHealthChecker,
		fx.Annotate(
			platformpostgres.NewPostgresWageringUnitOfWork,
			fx.As(new(application.WageringUnitOfWork)),
		),
		application.NewOpenWalletUseCase,
		application.NewSubmitWagerTransactionUseCase,
		application.NewProcessBetUseCase,
		application.NewProcessWinUseCase,
		application.NewProcessLossUseCase,
		fx.Annotate(
			application.NewProcessRefundUseCase,
			fx.ResultTags(`name:"process_refund"`),
		),
		fx.Annotate(
			application.NewProcessRollbackUseCase,
			fx.ResultTags(`name:"process_rollback"`),
		),
	),
)

func provideDatabaseHealthChecker(
	pool *pgxpool.Pool,
) httpadapter.DatabaseHealthChecker {
	return pool
}

func providePostgresPool(
	lifecycle fx.Lifecycle,
	postgresConfig config.PostgresConfig,
) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := platformpostgres.OpenPool(ctx, platformpostgres.PoolConfig{
		DatabaseURL:       postgresConfig.DatabaseURL,
		MaxConnections:    postgresConfig.MaxConnections,
		MinConnections:    postgresConfig.MinConnections,
		MaxConnectionIdle: postgresConfig.MaxConnectionIdle,
		MaxConnectionLife: postgresConfig.MaxConnectionLife,
	})
	if err != nil {
		return nil, err
	}

	lifecycle.Append(fx.Hook{
		OnStop: func(context.Context) error {
			pool.Close()
			return nil
		},
	})

	return pool, nil
}
