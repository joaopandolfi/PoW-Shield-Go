# Sprint 1 — Contrato PGP criptografado + challenge por cliente

Headers, payloads e respostas em formato de referência: [07-contract-payloads.md](07-contract-payloads.md).

Implementação do `specs_v2.md`: o front manda em todo request um header com `gpg_encrypt(<version>:<uuid>:<timestamp>)`, criptografado com a chave pública do WAF. Id, versão e timestamp nunca trafegam em texto aberto, o que dificulta a engenharia reversa do protocolo.

## Chaves

- O WAF tem um par de chaves OpenPGP.
- A chave **pública** vai embutida no front, que só criptografa.
- A chave **privada** fica só no WAF (`CONTRACT_PRIVATE_KEY_FILE`, com passphrase opcional em `CONTRACT_PRIVATE_KEY_PASSPHRASE`).
- Gerar: `gpg --quick-gen-key waf@empresa default default never` e depois `gpg --armor --export-secret-keys waf@empresa > waf.key.asc`. Testado com GnuPG 2.5 (cv25519, AEAD/OCB padrão).
- `*.key.asc` está no `.gitignore`.
- Se a chave não carregar, o processo não sobe (fail fast).

## Header

Um único header, nome configurável (`CONTRACT_HEADER`, default `X-Request-Context`, sem "pgp"/"pow" no nome):

```
X-Request-Context: base64(gpg_encrypt(payload))
```

- Aceita a mensagem OpenPGP binária ou armored (`-----BEGIN PGP MESSAGE-----`), sempre em base64 porque header não aceita quebra de linha.
- Payload:
  ```
  <version>:<uuid>:<unix_ts>                    # sem challenge resolvido
  <version>:<uuid>:<unix_ts>:<prefix>:<nonce>   # com challenge resolvido (nonce em hex)
  ```
- `version` hoje só aceita `1`.
- `uuid` precisa ser um UUID válido.
- `unix_ts` em segundos. Rejeita se a diferença para o relógio do servidor passar de `CONTRACT_MAX_AGE_SECONDS`, para trás ou para frente.
- Payload decriptado limitado a 512 bytes (protege contra mensagem comprimida inflando).
- Como o timestamp muda e a criptografia PGP é aleatória, o header é diferente a cada request.

Exemplo com gpg:

```sh
PAYLOAD="1:$CLIENT_UUID:$(date +%s)"            # ou "...:$PREFIX:$NONCE_HEX"
H=$(printf '%s' "$PAYLOAD" | gpg --batch --encrypt -r "$WAF_KEY" | base64 | tr -d '\n')
curl -H "X-Request-Context: $H" https://waf/...
```

No browser, o equivalente é `openpgp.encrypt` do openpgp.js com a chave pública do WAF, seguido de base64 do resultado.

PoW: achar `nonce` tal que `sha256(hex_decode(prefix) || nonce)` tenha `difficulty` bits zerados à esquerda. Mesmo algoritmo do solver JS atual (`client/solver`).

## Fluxo (middleware `Contract`)

```
1. pgp existe?    -> header presente, base64, mensagem OpenPGP
2. pgp saudável?  -> decripta com a chave do WAF (criptografia assimétrica, não por senha)
3. payload válido -> versão suportada, uuid válido, timestamp dentro da janela
4. id tem bloqueio?
5. replay?        -> hash do header já visto: queima o token do payload e devolve 401 com challenge mais difícil
6. tem challenge?
   - não                                   -> 401 {prefix, difficulty}
   - token já resolvido e com usos         -> OK (conta um uso)
   - token esgotou CONTRACT_MAX_USES       -> 401 com challenge mais difícil
   - challenge emitido e PoW válido        -> OK, a solução vira token
   - solução errada ou prefix desconhecido -> 401 com challenge (mais difícil se errou)
   - dificuldade > CONTRACT_MAX_DIFFICULTY -> bloqueia o id
```

Falhas de 1 a 4 respondem com o bloqueio padrão (`BLOCK_RESPONSE_STATUS`, 406), sem dizer o motivo ao cliente. O motivo vai para o log (`Contract rejected`).

## Token: PGP x JWT x solução como token

Pergunta do spec: usar o challenge resolvido como token ou devolver um JWT?

Decisão: **a própria solução do challenge é o token, com estado no cache** (`ct:t:<uuid>:<prefix>`).

- JWT é stateless, mas contar o uso do token já exige estado no servidor. Com o estado no cache, o JWT só adicionaria segredo, rotação e dependência, sem ganho.
- A solução só vale para o uuid que recebeu o challenge e viaja dentro do header criptografado.
- Token em cache é revogável na hora (bloqueio, exaustão) e não precisa de endpoint extra.

## Replay

- O WAF guarda o `sha256` do header (o texto criptografado) por `2 * CONTRACT_MAX_AGE_SECONDS`, que é a janela inteira em que o timestamp dele é aceito. Chave de cache: `ct:r:<hash>`.
- A criptografia PGP é aleatória: o mesmo payload criptografado duas vezes gera headers diferentes. Header repetido, portanto, só pode ser uma request capturada e reenviada. Requests legítimas paralelas no mesmo segundo não colidem.
- Ao detectar replay:
  - o token que veio no payload é queimado (está comprometido);
  - a resposta é 401 com um challenge com a dificuldade aumentada.
- O hash só é guardado depois que o header decripta e valida, então lixo não enche o cache.

## Dificuldade e bloqueio

- Base: `DEFAULT_PREFIX_SIZE` (bits). Incremento: `PUNISHMENT`. Reaproveitados da config de PoW existente.
- Sobe quando o token esgota `CONTRACT_MAX_USES`, quando a solução está errada ou em replay. A dificuldade de cada cliente fica guardada por `CONTRACT_PENALTY_SECONDS` e depois volta para a base.
- Passou de `CONTRACT_MAX_DIFFICULTY`: o id fica bloqueado por `CONTRACT_PENALTY_SECONDS`.
- Challenge é de uso único: uma tentativa errada queima o prefix.

## Redis

Com `REDIS_USE=true`, todo o estado do contrato fica no Redis e é compartilhado entre as instâncias do WAF. Chaves com TTL:

| Chave | Valor | TTL |
|---|---|---|
| `ct:c:<uuid>:<prefix>` | dificuldade do challenge emitido | `CONTRACT_CHALLENGE_TTL_SECONDS` |
| `ct:t:<uuid>:<prefix>` | usos do token | `CONTRACT_TOKEN_TTL_SECONDS` |
| `ct:d:<uuid>:` | dificuldade atual do cliente | `CONTRACT_PENALTY_SECONDS` |
| `ct:b:<uuid>:` | bloqueio | `CONTRACT_PENALTY_SECONDS` |
| `ct:r:<sha256>:` | header já visto (replay) | `2 * CONTRACT_MAX_AGE_SECONDS` |

- **Startup:** o WAF faz `PING` no Redis e não sobe se ele não responder.
- **Operações atômicas**, válidas também entre instâncias:
  - Replay: `SET NX` (`PutIfAbsent`). Dois reenvios simultâneos do mesmo header não passam os dois.
  - Usos do token: script Lua com `EXISTS` + `INCR` (`IncrIfExists`). Não perde incremento e não cria a chave se ela não existe; o TTL original é mantido. Nenhuma rajada passa de `CONTRACT_MAX_USES`.
  - Challenge: `GETDEL` (`GetDel`). Só uma request consome o prefix, então não há gasto duplo.
  - Na memória, as mesmas operações usam o lock do cache.
- **Versão mínima:** `GETDEL` exige Redis 6.2 ou mais novo.
- **Fail closed:** qualquer erro de cache durante a request responde 500. Nenhuma request passa sem checagem se o Redis cair.
- **Testes:**
  - `TestAtomicMemory` e `TestAtomicRedis` disparam 100 goroutines contra `IncrIfExists` e `GetDel`, com `-race`. Conferem que nenhum incremento se perde, que só uma goroutine leva o `GetDel` e que o TTL é mantido.
  - `TestContractFlowRedis` roda o fluxo completo com o client `go-redis` real contra o miniredis (Redis em processo, sem Docker). Também confere que as chaves foram para o Redis com TTL e que, com o Redis fora, a resposta é 500.
- **docker-compose:** sobe um `redis:7.4-alpine` e o WAF já com `REDIS_USE=true`.

## Configuração

| Env | Default | Descrição |
|---|---|---|
| `CONTRACT_ACTIVE` | `false` | Liga o middleware no proxy |
| `CONTRACT_PRIVATE_KEY_FILE` | `waf.key.asc` | Chave privada armored do WAF |
| `CONTRACT_PRIVATE_KEY_PASSPHRASE` | vazio | Passphrase da chave privada, se tiver |
| `CONTRACT_HEADER` | `X-Request-Context` | Nome do header |
| `CONTRACT_MAX_AGE_SECONDS` | `30` | Idade máxima do timestamp |
| `CONTRACT_MAX_USES` | `100` | Usos por challenge resolvido |
| `CONTRACT_TOKEN_TTL_SECONDS` | `600` | TTL do token, contado a partir da solução (não renova a cada uso) |
| `CONTRACT_CHALLENGE_TTL_SECONDS` | `60` | Tempo para resolver o challenge |
| `CONTRACT_MAX_DIFFICULTY` | `28` | Acima disso, bloqueia o id |
| `CONTRACT_PENALTY_SECONDS` | `3600` | Duração do bloqueio e da memória de dificuldade |

O middleware roda antes de PoW/WAF/RateLimit na cadeia do proxy. Com o contrato ligado, normalmente `POW_ACTIVE=false`, já que o fluxo de cookie/sessão do PoW antigo fica redundante.

## Teste e2e

```sh
./e2e/run.sh            # KEEP_UP=1 ./e2e/run.sh deixa a stack no ar
```

O script:

1. Gera um par de chaves descartável do WAF (curve25519, mesmo padrão do openpgp.js) em `e2e/.keys/`, que está no `.gitignore`.
2. Sobe via Docker duas instâncias do WAF (portas 5656 e 5657) com Redis compartilhado, na frente do backend http-echo. O arquivo usado é `e2e/docker-compose.e2e.yml` por cima do `docker-compose.yml`.
3. Roda `server/e2e/e2e_test.go` (build tag `e2e`), que se comporta como o front: `base64(gpg_encrypt(payload))` com a chave pública.
4. Derruba a stack.

O que o e2e cobre:

- Header ausente, em texto aberto, com timestamp velho ou com versão desconhecida: 406.
- Sem challenge: 401. A solução chega no backend (200, "protected backend").
- Replay numa instância diferente da que viu o header: 401, com a dificuldade subindo de 12 para 14.
- Token compartilhado entre as instâncias: 3 usos alternando a/b, e o 4º esgota (dificuldade 16).
- Solução errada sobe a dificuldade para 18. A seguinte passa de `CONTRACT_MAX_DIFFICULTY` e bloqueia o id, e o bloqueio vale na outra instância.
- Outro cliente não é afetado e continua na dificuldade base.
- Implícito: o header base64 passa pelas 120 regras do WAF sem falso positivo.

## Arquivos

- `server/services/contract/contract.go`: carrega a chave do WAF, decripta e valida o payload.
- `server/web/middleware/contract.go`: challenge, token, dificuldade e bloqueio.
- `server/web/middleware/contract_test.go`: teste do fluxo completo. Cobre header ausente, texto aberto, criptografado para outra chave, uuid inválido, timestamp velho, versão desconhecida, challenge, token, replay (token queimado e dificuldade aumentada), exaustão e bloqueio.
- `server/config/config.go`, `.env.example`: seção `CONTRACT_*`.
- `server/internal/cache/`: operações atômicas `PutIfAbsent`, `IncrIfExists` e `GetDel` (memória com lock, Redis com `SET NX`, Lua e `GETDEL`), `Ping` para checagem no startup, `MAX_BUFF_SIZE` de 150 para 100000. Testes em `atomic_test.go`.
- `server/main.go`: falha no startup se o cache não responder.
- `docker-compose.yml`: serviço `redis`.
- Dependência de teste: `github.com/alicebob/miniredis/v2`.
- `e2e/run.sh`, `e2e/docker-compose.e2e.yml`, `server/e2e/keygen/` (gera a chave descartável), `server/e2e/e2e_test.go`.
- Dependência nova: `github.com/ProtonMail/go-crypto` (OpenPGP mantido; `x/crypto/openpgp` está deprecated). `cloudflare/circl` fixado em v1.6.1 (versões anteriores têm vulnerabilidade conhecida).

## Limitações conhecidas

- **Criptografia não é autenticação.** A chave pública do WAF está no front, então quem extraí-la consegue montar headers válidos para qualquer uuid. O header dificulta a engenharia reversa, mas não prova quem enviou. O que segura abuso é o PoW e a dificuldade por uuid.
- **Troca de uuid.** Qualquer UUID válido é aceito, então um atacante pode gerar um uuid novo para recomeçar na dificuldade base. Ainda precisa resolver o PoW a cada uuid, e o rate limit por IP continua valendo. Se os uuids forem emitidos pelo backend, dá para validar contra uma lista cadastrada.
- **Várias instâncias:** com cache em memória, cada instância tem seu próprio estado (replay, tokens, bloqueio). Com várias instâncias, usar Redis.
- **Replay como assédio:** quem consegue capturar o tráfego (ou seja, já passou do TLS) pode reenviar um header para queimar o token do cliente e subir a dificuldade dele.
- Quando o token esgota, só a request que cruzou o limite sobe a dificuldade. Uma rajada paralela é punida uma vez só.
- A dificuldade por cliente ainda é get+put. Em eventos simultâneos (por exemplo, dois replays ao mesmo tempo), um aumento pode se perder. O erro é sempre para menos punição, nunca para menos checagem.
- Agora toda request grava uma entrada de replay no cache. O limite do cache em memória (`MAX_BUFF_SIZE`) subiu de 150 para 100000. Esse limite não remove entradas válidas: acima dele, cada gravação varre o mapa inteiro só para apagar as expiradas. Com 150, essa varredura acontecia em quase toda request. Para tráfego real, usar Redis.
- O uuid do cliente não é repassado ao backend protegido.
