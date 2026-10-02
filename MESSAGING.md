# Mensageria SQS

## Filas

O ambiente local provisiona três filas:

- `wager-transactions.fifo`: entrada de operações financeiras;
- `wager-transactions-dlq.fifo`: mensagens que esgotaram as tentativas;
- `integration-events.fifo`: publicação dos eventos da outbox.

A fila de entrada usa long polling de 10 segundos, visibility timeout de 30 segundos,
lotes de até 10 mensagens e redrive para a DLQ depois de 5 recebimentos sem conclusão.
O worker aplica backoff de 1, 2, 4, 8, 16 e no máximo 32 segundos alterando a
visibilidade da mensagem.

## Contrato de entrada

O corpo esperado é um envelope `WagerTransactionRequested`:

```json
{
  "messageId": "msg-123",
  "type": "WagerTransactionRequested",
  "occurredAt": "2026-09-08T12:00:00Z",
  "data": {
    "providerId": "provider-a",
    "externalTransactionId": "transaction-123",
    "idempotencyKey": "provider-a:transaction-123",
    "playerId": "player-1",
    "walletId": "wallet-1",
    "roundId": "round-1",
    "gameId": "game-1",
    "kind": "BET",
    "money": { "amount": "25.00", "currency": "BRL" }
  }
}
```

O produtor deve usar:

- `MessageGroupId`: o `walletId`, preservando a ordem das operações da carteira;
- `MessageDeduplicationId`: o `messageId`.

As garantias financeiras não dependem somente do FIFO: locks, constraints,
idempotência e inbox continuam sendo aplicados no PostgreSQL. Grupos diferentes são
processados em paralelo e mensagens do mesmo grupo são processadas em sequência.

## Inbox, retry e DLQ

O par `(consumerName, messageId)` identifica a entrega na inbox. O hash SHA-256 do
corpo bruto detecta a reutilização do mesmo ID com outro conteúdo. Uma mensagem só é
apagada da fila quando seu resultado durável está concluído.

Saldo, ledger, transação, outbox e conclusão da inbox compartilham a mesma transação
SQL. Se uma instância cair depois desse commit e antes do `DeleteMessage`, a reentrega
encontra a inbox concluída e é removida sem reaplicar a movimentação. Se cair enquanto
detém o lease, outra instância assume depois da expiração.

Rejeições de negócio persistidas são terminais e permitem apagar a mensagem. Falhas
transitórias liberam o lease da inbox e recebem backoff. Envelope inválido, conflito de
payload ou falhas repetidas não são apagados e chegam à DLQ pela política de redrive.

No shutdown, o worker para de buscar novas mensagens e aguarda as operações em curso.
Se o prazo de encerramento for esgotado, o processamento é cancelado e a visibilidade
da mensagem é alterada para zero com um contexto independente e limitado, permitindo
reentrega imediata por outra instância.
