# Arquitetura

## Concorrência financeira

A coordenação financeira é feita no PostgreSQL, por carteira, sem locks globais ou
estado de sincronização em memória. Cada operação roda em uma transação `READ COMMITTED`.
O adapter carrega a operação com `SELECT ... FOR UPDATE` e, em seguida, bloqueia somente
a linha da carteira correspondente com `SELECT ... FOR UPDATE`. Operações da mesma
carteira são serializadas; carteiras diferentes continuam avançando em paralelo.

A atualização da carteira também compara a versão anterior no `UPDATE`. Esse controle
otimista é uma defesa adicional contra atualização perdida caso algum novo fluxo deixe
de adquirir o lock pessimista. Saldo, versão, estado da operação, ledger e outbox são
confirmados no mesmo commit.

Idempotência é persistente por `(provider_id, idempotency_key)` e por
`(provider_id, external_transaction_id)`. Violações concorrentes das chaves únicas,
inclusive da chave primária interna, provocam uma releitura do registro vencedor. Se
outra instância concluir uma operação entre o aceite e o processamento, a instância
perdedora devolve o resultado terminal persistido sem reaplicar a movimentação.

Os testes de integração usam três pools PostgreSQL independentes, cada um com sua
própria unidade de trabalho e seus próprios casos de uso. Eles cobrem:

- cinquenta entregas simultâneas da mesma aposta, resultando em um único débito;
- duas apostas distintas de 80.00 BRL sobre 100.00 BRL, resultando em saldo 20.00,
  uma operação processada e outra rejeitada;
- processamento de uma carteira enquanto a linha de outra permanece bloqueada;
- comparação do saldo armazenado com a soma dos créditos e débitos do ledger.
