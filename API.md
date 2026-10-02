# Contrato HTTP

Todas as respostas usam `application/json`. Erros seguem o formato:

```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "invalid wager transaction request"
  }
}
```

## Autenticação

Os endpoints de negócio exigem `Authorization: Bearer <token>` validado pelo IdP
OIDC. Operações de carteira exigem uma identidade interna. Operações e consultas de
apostas exigem uma identidade de provedor, e o `providerId` do caminho ou corpo deve
ser o mesmo da identidade autenticada.

Os endpoints `/health/live` e `/health/ready` são públicos.

## Rotas

| Método e rota | Autorização | Sucesso |
| --- | --- | --- |
| `POST /wallets` | interna | `201 Created` |
| `GET /wallets/{walletId}` | interna | `200 OK` |
| `GET /wallets/{walletId}/ledger` | interna | `200 OK` |
| `POST /wallets/{walletId}/reconciliation` | interna | `200 OK` |
| `POST /wagering/transactions` | provedor | conforme o resultado abaixo |
| `GET /wagering/transactions/{transactionId}` | provedor proprietário | `200 OK` |
| `GET /providers/{providerId}/wagering/transactions/{externalTransactionId}` | provedor proprietário | `200 OK` |
| `GET /health/live` | pública | `200 OK` |
| `GET /health/ready` | pública | `200 OK` ou `503 Service Unavailable` |
| `GET /metrics` | pública | `200 OK` |

O ledger aceita `limit` entre 1 e 100, com padrão 50, e `cursor` opaco. A ordenação
é estável por `(createdAt, id)`. A resposta omite `nextCursor` quando não existe outra
página.

`POST /wagering/transactions` exige `Content-Type: application/json` e o header
`Idempotency-Key`. O resultado usa:

| Estado | HTTP |
| --- | --- |
| `PROCESSED` | `200 OK` |
| `PENDING` ou `PENDING_REFERENCE` | `202 Accepted` |
| `REJECTED` | `422 Unprocessable Entity` |
| `FAILED` | `500 Internal Server Error` |

Um replay idempotente devolve o mesmo `transactionId`, estado e saldo observado na
operação original, com `idempotentReplay: true`.

## Erros principais

| HTTP | Código |
| --- | --- |
| `400` | `INVALID_REQUEST`, `INVALID_QUERY`, `INVALID_CURSOR`, `IDEMPOTENCY_KEY_REQUIRED` |
| `401` | `UNAUTHORIZED` |
| `403` | `FORBIDDEN`, `PROVIDER_MISMATCH` |
| `404` | `WALLET_NOT_FOUND`, `WAGER_TRANSACTION_NOT_FOUND` |
| `409` | `WALLET_ALREADY_EXISTS`, `IDEMPOTENCY_KEY_CONFLICT`, `EXTERNAL_TRANSACTION_CONFLICT` |
| `415` | `UNSUPPORTED_MEDIA_TYPE` |
| `503` | `TEMPORARILY_UNAVAILABLE` |
| `500` | `INTERNAL_ERROR` |

Consultas de uma transação por um provedor diferente retornam `404`, evitando revelar
a existência de dados de outro provedor. Quando o próprio `providerId` da URL ou do
corpo diverge da identidade, a API retorna `403`.
