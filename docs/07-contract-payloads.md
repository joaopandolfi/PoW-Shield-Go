# Contrato — headers e payloads

Referência de tudo que trafega entre o front e o WAF no fluxo do contrato criptografado. O protocolo, a configuração e as decisões de design estão em [sprint1.md](sprint1.md).

## Handshake (com `ATTEST_ACTIVE=true`)

Antes de usar o contrato, o app obtém a chave e o uuid:

```
POST /v1/handshake
X-Firebase-AppCheck: <token do Firebase App Check>

200 {"key":"-----BEGIN PGP PUBLIC KEY BLOCK-----\n...","id":"3f2b8c1e-...","token":"eyJhbGciOiJFZERTQSIs...","exp":1760457600}
```

| Campo | Uso |
|---|---|
| `key` | Chave pública armored do WAF. Cifrar o contrato com ela e manter só em memória |
| `id` | O uuid do cliente (o `sub` do token). Só para referência: **não** vai no payload |
| `token` | Id token (JWT EdDSA assinado pelo WAF). Vai no campo de id do payload no lugar do uuid |
| `exp` | Unix em segundos. Refazer o handshake antes disso; depois, o contrato responde 406 |

Repetir o mesmo token do App Check devolve o mesmo `token`. Token inválido recebe 406. Detalhes em [sprint3.md](sprint3.md).

## Headers

| Header | Direção | Obrigatório | Configurável por | Conteúdo |
|---|---|---|---|---|
| `X-Request-Context` | front → WAF | sim, em toda request ao proxy | `CONTRACT_HEADER` | `base64(gpg_encrypt(payload))` |
| `X-Firebase-AppCheck` | front → WAF | só no handshake | `ATTEST_HEADER` | token do Firebase App Check |
| `Retry-After` | WAF → front | só no 429 | — | segundos até liberar o rate limit: `CONTRACT_RATE_LIMIT_WINDOW_SECONDS` (por uuid) ou `RATE_LIMIT_WINDOW_SECONDS` (por IP) |

- O nome default é propositalmente genérico: nada de "pgp", "pow" ou "challenge" no nome.
- Se `CONTRACT_HEADER` for alterado, front e WAF precisam usar o mesmo nome.
- Existe um único header de request: o challenge resolvido vai dentro do mesmo payload criptografado.
- O WAF não remove o header antes de repassar a request ao backend protegido.

## Payload da request (antes de criptografar)

Texto ASCII, campos separados por `:`, sem espaços nem quebra de linha.

```
1:<uuid>:<unix_ts>                                    # v1, sem challenge resolvido
1:<uuid>:<unix_ts>:<prefix>:<nonce>                   # v1, com challenge resolvido
2:<uuid>:<unix_ts>:<request_hash>                     # v2 (binding), sem challenge resolvido
2:<uuid>:<unix_ts>:<request_hash>:<prefix>:<nonce>    # v2 (binding), com challenge resolvido
```

- **v1:** só identifica o cliente.
- **v2:** amarra o header a uma request específica (request binding). É obrigatória com `CONTRACT_REQUIRE_BINDING=true`, e aí a v1 passa a ser rejeitada.

| Campo | Formato | Regra |
|---|---|---|
| `version` | `1` ou `2` | v2 inclui `request_hash` |
| `uuid` | UUID (`xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`), ou o id token com `ATTEST_ACTIVE=true` | Sem atestação: uuid escolhido pelo cliente. Com atestação: o `token` do handshake, validado pela assinatura e pelo `exp`, sem consulta; o id passa a ser o `sub` |
| `unix_ts` | inteiro, segundos desde epoch | Diferença para o relógio do servidor no máximo `CONTRACT_MAX_AGE_SECONDS` (default 30), para trás ou para frente |
| `request_hash` | hex, 64 caracteres (só v2) | `sha256` da request; definição abaixo |
| `prefix` | hex, 32 caracteres | Exatamente o `prefix` recebido no 401 |
| `nonce` | hex | Solução do PoW para esse `prefix` |

- Tamanho máximo depois de decriptar: 1024 bytes (o id token tem cerca de 320).
- O servidor aceita 3 ou 5 campos na v1 e 4 ou 6 na v2.

### Request hash (v2)

```
request_hash = hex( sha256( METHOD + "\n" + REQUEST_URI + "\n" + BODY ) )
```

| Parte | Exemplo | Regra |
|---|---|---|
| `METHOD` | `POST` | Em maiúsculas, exatamente como enviado |
| `REQUEST_URI` | `/api/orders?page=2` | Path + query como trafegam na linha da request (já com percent-encoding), sem host. Sem query: só o path |
| `BODY` | `{"item":42}` | Bytes exatos do body. Vazio em GET |

Se qualquer parte não bater com a request recebida, a resposta é 406. Um header capturado não serve para outra rota, outro método ou outro body.

Exemplos:

```
1:3f2b8c1e-9d4a-4f6b-8a2e-1c5d7e9f0a3b:1760371200
1:3f2b8c1e-9d4a-4f6b-8a2e-1c5d7e9f0a3b:1760371205:5c3b23d589cbd72cb7ced1e748716726:00000000000000000000000000001a2f
2:3f2b8c1e-9d4a-4f6b-8a2e-1c5d7e9f0a3b:1760371210:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08:5c3b23d589cbd72cb7ced1e748716726:00000000000000000000000000001a2f
```

## Codificação

```
X-Request-Context = base64( OpenPGP_encrypt( payload, chave_publica_do_WAF ) )
```

- **Criptografia:** OpenPGP de chave pública, para a chave do WAF. Mensagem criptografada por senha (simétrica) é rejeitada.
- **Assinatura:** não é exigida e é ignorada se vier.
- **Formato da mensagem:** binário (recomendado, header menor) ou armored (`-----BEGIN PGP MESSAGE-----`). O servidor detecta qual é.
- **Base64:** padrão (RFC 4648, com `+`, `/` e `=`), em uma linha só.
- **Aleatoriedade:** a criptografia PGP é aleatória, então o mesmo payload gera um header diferente a cada envio. Header repetido é tratado como replay.
- **Tamanho:** cerca de 300 a 600 bytes, conforme o tipo de chave e o formato.

### Front (openpgp.js)

```js
const wafKey = await openpgp.readKey({ armoredKey: WAF_PUBLIC_KEY });

async function requestHash(method, uri, body = '') {
  const data = new TextEncoder().encode(`${method}\n${uri}\n${body}`);
  const digest = await crypto.subtle.digest('SHA-256', data);
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, '0')).join('');
}

// v2: header amarrado à request (method, uri, body)
async function contractHeader(uuid, method, uri, body, solution /* "prefix:nonceHex" ou "" */) {
  const ts = Math.floor(Date.now() / 1000);
  let payload = `2:${uuid}:${ts}:${await requestHash(method, uri, body)}`;
  if (solution) payload += `:${solution}`;

  const encrypted = await openpgp.encrypt({
    message: await openpgp.createMessage({ text: payload }),
    encryptionKeys: wafKey,
    format: 'binary',
  });
  return btoa(String.fromCharCode(...encrypted));
}

const body = JSON.stringify({ item: 42 });
fetch('/api/orders?page=2', {
  method: 'POST',
  body,
  headers: { 'X-Request-Context': await contractHeader(uuid, 'POST', '/api/orders?page=2', body, solution) },
});
```

### Shell (gpg)

```sh
PAYLOAD="1:$CLIENT_UUID:$(date +%s)"
curl -H "X-Request-Context: $(printf '%s' "$PAYLOAD" | gpg --batch --encrypt -r "$WAF_KEY" | base64 | tr -d '\n')" https://waf/api/x
```

## Respostas do WAF

| Status | Content-Type | Body | Quando |
|---|---|---|---|
| repassado do backend | repassado | repassado | Contrato válido e token com usos disponíveis |
| `401` | `application/json` | `{"prefix":"<hex32>","difficulty":<int>}` | Payload sem challenge, prefix desconhecido ou expirado, solução errada, token esgotado ou replay |
| `406` (`BLOCK_RESPONSE_STATUS`) | `BLOCK_RESPONSE_CONTENT_TYPE` (`text/plain; charset=utf-8`) | `BLOCK_RESPONSE_BODY` (`blocked: x_x`) | Header ausente ou inválido, criptografado para outra chave, versão desconhecida, uuid inválido, timestamp fora da janela, `request_hash` inválido ou diferente da request, v1 com binding obrigatório, cliente bloqueado |
| `429` | `application/json` | `{"code":13,"message":"rate limit exceeded"}` + header `Retry-After` | Rate limit por uuid (`CONTRACT_RATE_LIMIT_*`) ou por IP (`RATE_LIMIT_*`) |
| `500` | `application/json` | `{"code":10,"message":"internal error"}` | Erro de cache (Redis fora). Falha fechada: a request não passa |

Os 406 não dizem o motivo da rejeição. O motivo aparece só no log do WAF (`Contract rejected`, campo `reason`).

### Corpo do 401 (challenge)

```json
{"prefix":"5c3b23d589cbd72cb7ced1e748716726","difficulty":12}
```

| Campo | Tipo | Significado |
|---|---|---|
| `prefix` | string hex, 32 caracteres (16 bytes) | Uso único. Vale por `CONTRACT_CHALLENGE_TTL_SECONDS` (default 60) |
| `difficulty` | int | Número de bits zero exigidos no começo do hash |

Quando a resposta é um challenge mais difícil (solução errada, token esgotado ou replay), `difficulty` já vem aumentado em `PUNISHMENT`.

## Resolvendo o challenge

Achar bytes `nonce` tais que:

```
sha256( hex_decode(prefix) || nonce )  começa com <difficulty> bits zero
```

- `nonce` vai no payload em hex. Tamanho livre; o solver atual usa 16 bytes.
- É o mesmo algoritmo do solver JS existente: `new Solver().solve(difficulty, prefix)` em `client/solver/solver.js`, que devolve um `Buffer`. Usar `nonce.toString('hex')`.
- Depois de resolver, o front manda `...:<prefix>:<nonce>` nas próximas requests, sempre com um timestamp novo e uma nova criptografia.

## Ciclo do token

| Situação | Resposta |
|---|---|
| Primeira request com a solução correta | Repassa ao backend (uso 1) |
| Requests seguintes com o mesmo `prefix:nonce` | Repassa (uso 2, 3, …) até `CONTRACT_MAX_USES` (default 100) |
| Uso `CONTRACT_MAX_USES + 1` | 401 com challenge mais difícil |
| Depois de `CONTRACT_TOKEN_TTL_SECONDS` (default 600) desde a solução | 401 com challenge na dificuldade atual |
| Mesmo header (mesmo base64) reenviado | 401 com challenge mais difícil, e o token é queimado |
| Dificuldade passa de `CONTRACT_MAX_DIFFICULTY` | 406 para esse uuid por `CONTRACT_PENALTY_SECONDS` |
| Mais de `CONTRACT_RATE_LIMIT_REQUESTS` em `CONTRACT_RATE_LIMIT_WINDOW_SECONDS` para o mesmo uuid | 429 com `Retry-After`, sem mexer na dificuldade |

Os limites são configuráveis:

| Quero configurar | Env |
|---|---|
| Quantas requests o cliente faz com um challenge resolvido antes de receber outro mais difícil | `CONTRACT_MAX_USES` |
| Por quanto tempo um challenge resolvido vale | `CONTRACT_TOKEN_TTL_SECONDS` |
| Quanto tempo o cliente tem para resolver um challenge emitido | `CONTRACT_CHALLENGE_TTL_SECONDS` |
| Dificuldade inicial e quanto ela sobe por punição | `DEFAULT_PREFIX_SIZE`, `PUNISHMENT` |
| Até onde a dificuldade sobe antes de bloquear | `CONTRACT_MAX_DIFFICULTY` |
| Requests por uuid em uma janela | `CONTRACT_RATE_LIMIT_REQUESTS`, `CONTRACT_RATE_LIMIT_WINDOW_SECONDS` |
| Dificuldade subindo a cada X requests na janela | `CONTRACT_DIFFICULTY_STEP` (bits por degrau), `CONTRACT_DIFFICULTY_EVERY` (X), janela `CONTRACT_RATE_LIMIT_WINDOW_SECONDS` |

Fluxo típico do front:

```
1. GET /api/x   X-Request-Context: enc("2:<uuid>:<ts>:<hash>")                     -> 401 {prefix, difficulty}
2. resolve o PoW
3. GET /api/x   X-Request-Context: enc("2:<uuid>:<ts>:<hash>:<prefix>:<nonce>")    -> 200 (backend)
4. repete o passo 3 (ts novo, criptografia nova) até receber 401
5. no 401, resolve o novo challenge e volta ao passo 3
```
