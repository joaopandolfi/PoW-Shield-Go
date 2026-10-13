# Contrato — headers e payloads

Referência de tudo que trafega entre o front e o WAF no fluxo do contrato criptografado. O protocolo, a configuração e as decisões de design estão em [sprint1.md](sprint1.md).

## Headers

| Header | Direção | Obrigatório | Configurável por | Conteúdo |
|---|---|---|---|---|
| `X-Request-Context` | front → WAF | sim, em toda request ao proxy | `CONTRACT_HEADER` | `base64(gpg_encrypt(payload))` |
| `Retry-After` | WAF → front | só no 429 | — | segundos até liberar o rate limit (`RATE_LIMIT_WINDOW_SECONDS`) |

- O nome default é propositalmente genérico: nada de "pgp", "pow" ou "challenge" no nome.
- Se `CONTRACT_HEADER` for alterado, front e WAF precisam usar o mesmo nome.
- Existe um único header de request: o challenge resolvido vai dentro do mesmo payload criptografado.
- O WAF não remove o header antes de repassar a request ao backend protegido.

## Payload da request (antes de criptografar)

Texto ASCII, campos separados por `:`, sem espaços nem quebra de linha.

```
<version>:<uuid>:<unix_ts>                     # sem challenge resolvido
<version>:<uuid>:<unix_ts>:<prefix>:<nonce>    # com challenge resolvido
```

| Campo | Formato | Regra |
|---|---|---|
| `version` | `1` | Única versão aceita hoje |
| `uuid` | UUID (`xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`) | Id do cliente. Normalizado para minúsculas no servidor |
| `unix_ts` | inteiro, segundos desde epoch | Diferença para o relógio do servidor no máximo `CONTRACT_MAX_AGE_SECONDS` (default 30), para trás ou para frente |
| `prefix` | hex, 32 caracteres | Exatamente o `prefix` recebido no 401 |
| `nonce` | hex | Solução do PoW para esse `prefix` |

- Tamanho máximo depois de decriptar: 512 bytes.
- O servidor só aceita 3 ou 5 campos.

Exemplos:

```
1:3f2b8c1e-9d4a-4f6b-8a2e-1c5d7e9f0a3b:1760371200
1:3f2b8c1e-9d4a-4f6b-8a2e-1c5d7e9f0a3b:1760371205:5c3b23d589cbd72cb7ced1e748716726:00000000000000000000000000001a2f
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

async function contractHeader(uuid, solution /* "prefix:nonceHex" ou "" */) {
  let payload = `1:${uuid}:${Math.floor(Date.now() / 1000)}`;
  if (solution) payload += `:${solution}`;

  const encrypted = await openpgp.encrypt({
    message: await openpgp.createMessage({ text: payload }),
    encryptionKeys: wafKey,
    format: 'binary',
  });
  return btoa(String.fromCharCode(...encrypted));
}

fetch('/api/x', { headers: { 'X-Request-Context': await contractHeader(uuid, solution) } });
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
| `406` (`BLOCK_RESPONSE_STATUS`) | `BLOCK_RESPONSE_CONTENT_TYPE` (`text/plain; charset=utf-8`) | `BLOCK_RESPONSE_BODY` (`blocked: x_x`) | Header ausente ou inválido, criptografado para outra chave, versão desconhecida, uuid inválido, timestamp fora da janela, cliente bloqueado |
| `429` | `application/json` | `{"code":13,"message":"rate limit exceeded"}` + header `Retry-After` | Rate limit por IP |
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

Fluxo típico do front:

```
1. GET /api/x   X-Request-Context: enc("1:<uuid>:<ts>")                     -> 401 {prefix, difficulty}
2. resolve o PoW
3. GET /api/x   X-Request-Context: enc("1:<uuid>:<ts>:<prefix>:<nonce>")    -> 200 (backend)
4. repete o passo 3 (ts novo, criptografia nova) até receber 401
5. no 401, resolve o novo challenge e volta ao passo 3
```
