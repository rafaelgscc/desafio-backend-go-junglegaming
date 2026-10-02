# Jungle Gaming Wallet

Serviço distribuído em Go para processar `BET`, `WIN`, `LOSS`, `REFUND` e
`ROLLBACK` com precisão monetária, ledger append-only, idempotência persistente,
PostgreSQL, SQS FIFO, OIDC e transactional outbox. A composição da aplicação e o
ciclo de vida do servidor e dos workers usam Uber Fx.

## Pré-requisitos

- Go 1.27.1;
- Docker Desktop com Docker Compose;
- portas `5433`, `4566`, `8080` e `8081` disponíveis.

## Inicialização rápida

Copie `.env.example` para `.env` se quiser executar a aplicação diretamente com Go.
O Compose já possui valores locais próprios e não depende desse arquivo.

```bash
docker compose up --build
```

Esse comando inicia PostgreSQL, aplica as migrations, importa o realm do Keycloak,
cria as três filas no LocalStack e inicia a API em `http://localhost:8080`.

Serviços locais:

| Serviço | Endereço |
| --- | --- |
| API | `http://localhost:8080` |
| Keycloak | `http://localhost:8081` |
| PostgreSQL | `localhost:5433` |
| LocalStack SQS | `http://localhost:4566` |

Em Docker Desktop, a aplicação usa
`http://host.docker.internal:8081/realms/jungle` como issuer. Solicite tokens usando
esse mesmo host para que o claim `iss` corresponda ao issuer configurado.

## Identidades locais

| Identidade | Client ID | Client secret | Permissão |
| --- | --- | --- | --- |
| Provedor A | `provider-a` | `provider-a-secret` | operações do provider-a |
| Provedor B | `provider-b` | `provider-b-secret` | operações do provider-b |
| Serviço interno | `internal-service` | `internal-service-secret` | carteiras e reconciliação |

Exemplo em PowerShell:

```powershell
$tokenResponse = Invoke-RestMethod -Method Post `
  -Uri "http://host.docker.internal:8081/realms/jungle/protocol/openid-connect/token" `
  -ContentType "application/x-www-form-urlencoded" `
  -Body @{ grant_type="client_credentials"; client_id="provider-a"; client_secret="provider-a-secret" }
$providerToken = $tokenResponse.access_token
```

Para execução local com `go run`, use `localhost` no lugar de
`host.docker.internal` tanto no token endpoint quanto em `OIDC_ISSUER_URL`.

## Variáveis de ambiente

As variáveis completas de exemplo estão em [.env.example](.env.example). As principais
são:

- `DATABASE_URL`, `DATABASE_MAX_CONNECTIONS`, `DATABASE_MIN_CONNECTIONS`;
- `HTTP_ADDRESS`;
- `OIDC_ISSUER_URL`, `OIDC_AUDIENCE`;
- `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`;
- `SQS_ENDPOINT`, `SQS_QUEUE_NAME`, `SQS_EVENT_QUEUE_NAME`.

Para executar fora do container:

```powershell
Get-Content .env | ForEach-Object {
  if ($_ -and -not $_.StartsWith('#')) {
    $name, $value = $_ -split '=', 2
    Set-Item -Path "Env:$name" -Value $value
  }
}
go run ./cmd/api
```

## Migrations

As migrations ficam em `migrations/` e são aplicadas automaticamente pelo Compose.

```bash
docker compose run --rm migrate up
docker compose run --rm migrate down 1
docker compose run --rm migrate down
```

Pare a aplicação antes de reverter uma migration utilizada por ela. Cada alteração
possui arquivos `up` e `down` versionados.

## Filas

`deploy/localstack/init-aws.sh` provisiona automaticamente:

- `wager-transactions.fifo`;
- `wager-transactions-dlq.fifo`, com redrive após 5 recebimentos;
- `integration-events.fifo`.

Na entrada, use `walletId` como `MessageGroupId` e `messageId` como
`MessageDeduplicationId`. Nos eventos de saída, esses valores são `aggregateId` e
`eventId`, respectivamente. Consulte [MESSAGING.md](MESSAGING.md) e
[EVENTS.md](EVENTS.md).

## Exemplos HTTP

Abra uma carteira com o token de `internal-service`:

```bash
curl -X POST http://localhost:8080/wallets \
  -H "Authorization: Bearer $INTERNAL_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"playerId":"player-1","initialBalance":{"amount":"100.00","currency":"BRL"}}'
```

Envie uma aposta com o token de `provider-a`, substituindo `WALLET_ID`:

```bash
curl -X POST http://localhost:8080/wagering/transactions \
  -H "Authorization: Bearer $PROVIDER_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: provider-a:bet-1" \
  -H "X-Correlation-ID: example-1" \
  -d '{"providerId":"provider-a","externalTransactionId":"bet-1","playerId":"player-1","walletId":"WALLET_ID","roundId":"round-1","gameId":"game-1","kind":"BET","money":{"amount":"25.00","currency":"BRL"}}'
```

Rotas, respostas e erros estão em [API.md](API.md). Observabilidade está em
[OBSERVABILITY.md](OBSERVABILITY.md). Os endpoints públicos são:

- `GET /health/live`;
- `GET /health/ready`;
- `GET /metrics`.

## Testes

Suíte unitária e verificações estáticas:

```bash
go test ./...
go test -race ./...
go vet ./...
```

Para executar todas as integrações reais, mantenha PostgreSQL, Keycloak e LocalStack
ativos e configure:

```powershell
$env:TEST_DATABASE_URL="postgres://jungle:jungle@localhost:5433/jungle_wallet?sslmode=disable"
$env:TEST_OIDC_ISSUER_URL="http://localhost:8081/realms/jungle"
$env:TEST_SQS_ENDPOINT="http://localhost:4566"
go test ./test/integration -count=1
go test -race ./test/integration -count=1
```

Uma variável `TEST_*` ausente faz somente os testes dependentes daquele serviço serem
ignorados. A matriz de cenários obrigatórios está em [VERIFICATION.md](VERIFICATION.md).

## Arquitetura e decisões

- [ARCHITECTURE.md](ARCHITECTURE.md): dinheiro, locks, transações e idempotência;
- [API.md](API.md): contrato HTTP e autorização;
- [MESSAGING.md](MESSAGING.md): inbox, retry, DLQ e shutdown;
- [EVENTS.md](EVENTS.md): transactional outbox e eventos;
- [OBSERVABILITY.md](OBSERVABILITY.md): logs, métricas e health checks.

## Limitação conhecida

Tracing distribuído e dashboards não foram implementados, pois são diferenciais
opcionais. A execução de testes reais do SQS depende de o ambiente conseguir baixar e
iniciar a imagem do LocalStack; os testes permanecem disponíveis e não são substituídos
por mocks.
