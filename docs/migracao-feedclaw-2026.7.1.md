# Plano de Migração e Correções — FeedClaw

> **Repositório:** `marceloalmeidadev/feedclaw` — `main`, 42 commits, Fases 1–7 completas.
> **Alvo:** OpenClaw v2026.7.1.
> **Base:** revisão do `cmd/feedclaw/*` e do `skill/SKILL.md` reais (não do README).
> **Substitui:** `adendo-feedclaw-openclaw-2026.7.1.md` e o plano preliminar baseado só no README.

---

## Sumário executivo

A migração para as funcionalidades da v2026.7.1 (`on-exit`, `utilityModel`) é direta e de baixo risco. **Mas a revisão do código encontrou uma falha de SSRF que precede e supera a migração em prioridade**, além de uma dúvida crítica sobre a ativação do guard e três bugs colaterais.

**Ordem correta: segurança primeiro, migração depois.**

| # | PR | Tipo | Prioridade |
|---|---|---|---|
| **0** | Verificar `SecurityMode` zero-value | 🔴 Segurança / investigação | **Bloqueante** |
| **1** | SSRF no `import --opml` | 🔴 Segurança | **Bloqueante** |
| **2** | Verificação da integração v2026.7.1 | Investigação | Bloqueante da migração |
| **3** | `fetch`: exit codes, `--report`, lockfile | Migração (breaking) | Alta |
| **4** | SKILL: separar fluxo fetch/digest | Migração | Alta |
| **5** | Cron `on-exit` + sessão dedicada | Migração | Alta |
| **6** | Tiering de `utilityModel` | Migração | Média |
| **7** | Bugs: UTF-8, timezone, help do `--db` | Correção | Média |
| **8** | Licença, release, `install` pinado | Chore / bloqueante de publicação | Média |

### O que já está correto — não abrir

Verificado no código, contrariando o plano preliminar:

- ✅ **Gating por binário já existe:** `metadata.openclaw.requires.bins: ["feedclaw"]` no frontmatter.
- ✅ **`description` bem formada:** usa bloco `>-`, sem o footgun de YAML que quebra o parse silenciosamente.
- ✅ Instrução de dado não confiável no SKILL.md (Seção 4) — boa, explícita e com exemplos.
- ✅ `serve` binda exclusivamente em `127.0.0.1`, com `ReadHeaderTimeout`.
- ✅ Tema residual "Outros" e validação de `article_ids` no `digest save`.
- ✅ O SKILL.md **não** contém instruções de instalação de dependência via shell — importante dado o histórico de skills maliciosos no ClawHub usando markdown como instalador.

---

# PARTE I — SEGURANÇA (antes de qualquer migração)

## PR 0 — 🔴 INVESTIGAR: `fetch.Config{}` zero-value desliga o guard?

**Bloqueante. Primeira coisa a fazer.**

Três caminhos passam um `Config` vazio:

```go
// cmd_content.go — extractFull()
client, _ := fetch.Client(fetch.Config{})

// cmd_serve.go — runServe()
Handler: api.New(st, fetch.Config{}).Handler()

// cmd_fetch.go — runFetch()
f := fetch.New(st, fetch.Config{Workers: workers})   // SecurityMode também vazio
```

Se o zero-value de `Config.SecurityMode` for `""` e o guard só ativar comparando com `"restricted"`, **o SSRF guard está desligado nos três caminhos** — incluindo o fetcher, o `full` e toda a API.

Isso é a diferença entre "seguro" e "acha que é seguro". O README afirma que o guard é *always on*; o código só confirma isso se `internal/fetch` normalizar o zero-value.

**Ação:**
1. Ler `internal/fetch/` e determinar o comportamento de `SecurityMode == ""`.
2. Se **não** normalizar para `restricted`: corrigir imediatamente. A normalização deve morar **dentro** de `fetch.Config` (num `func (c Config) normalize() Config` ou equivalente), nunca depender de cada chamador lembrar de preencher.
3. Adicionar teste: `fetch.Client(fetch.Config{})` deve **recusar** `http://127.0.0.1/`, `http://169.254.169.254/` e `http://10.0.0.1/`.
4. **Fail-safe por construção:** considere inverter a semântica para que o modo mais restritivo seja o zero-value. Um `Config` vazio jamais deve ser o menos seguro.

**Bônus no mesmo PR — erro descartado:**

```go
client, _ := fetch.Client(fetch.Config{})   // ← erro ignorado
html, err := readability.Extract(ctx, client, url, 0)
```

Se `fetch.Client` puder retornar `(nil, err)`, o `Extract` recebe um client nil e entra em pânico. Tratar o erro.

---

## PR 1 — 🔴 CORRIGIR: `import --opml` fura o SSRF guard

**Local:** `cmd/feedclaw/cmd_feeds.go`, função `loadOPML`.

```go
func loadOPML(src string) ([]opml.Feed, error) {
	if u, err := url.Parse(src); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		client := &http.Client{Timeout: 30 * time.Second}   // ← http.Client CRU
		resp, err := client.Get(src)
		...
		return opml.Parse(resp.Body)                        // ← sem limite de bytes
	}
	...
}
```

### O problema

Este é um `http.Client` padrão. **Não é o `fetch.Client` com o guard.** Todo o trabalho de anti-DNS-rebinding, bloqueio de RFC1918/loopback/link-local e revalidação por hop de redirect — que existe no fetcher e no readability — **não se aplica a este caminho de saída de rede**.

Consequências concretas:

- `feedclaw import --opml http://169.254.169.254/latest/meta-data/` busca metadados de instância cloud sem qualquer bloqueio.
- `--opml http://127.0.0.1:8484/api/...` alcança serviços locais, inclusive a própria API do FeedClaw.
- Redirects são seguidos por default (até 10, pelo `http.Client`), **sem revalidação de hop** — um OPML remoto legítimo pode redirecionar para um alvo interno.
- **Sem cap de bytes:** `resp.Body` vai direto para `opml.Parse`. Uma resposta hostil de 5 GB é lida sem limite. O README anuncia `max_feed_bytes` (2 MiB) — não vale aqui.

O README declara o SSRF guard como transversal ("enforced from Phase 1"). Ele é — em dois dos três caminhos de saída. Este é o terceiro, e passou despercebido. É exatamente a classe de bug que uma auditoria pega: a defesa existe, está bem construída, e um caminho não a usa.

### A correção

```go
func loadOPML(src string) ([]opml.Feed, error) {
	if u, err := url.Parse(src); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		client, err := fetch.Client(fetch.Config{})   // guard + redirect hops + timeouts
		if err != nil {
			return nil, err
		}
		resp, err := client.Get(src)
		if err != nil {
			return nil, fmt.Errorf("fetch opml: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch opml: status %d", resp.StatusCode)
		}
		return opml.Parse(io.LimitReader(resp.Body, maxOPMLBytes))
	}
	...
}
```

Requisitos:

1. **Usar `fetch.Client`** — nunca `http.Client` cru. Depende do PR 0 estar resolvido, ou o guard virá desligado de qualquer forma.
2. **`io.LimitReader`** com `maxOPMLBytes` (sugestão: 4 MiB — um OPML do Feedly com centenas de feeds fica na casa das dezenas de KB).
3. **Auditar o resto do repositório** procurando por outros `http.Client` crus, `http.Get`, `http.Post`, ou `http.DefaultClient`. Este pode não ser o único. Grep sugerido:
   ```sh
   grep -rn 'http\.Client{\|http\.Get(\|http\.Post(\|http\.DefaultClient' --include='*.go' .
   ```
4. **Verificar o endpoint `POST /api/feeds/import`** da API: o README diz que ele aceita um documento OPML *uploaded* (sem rede), mas confirmar que não há um caminho por URL ali também.
5. **Regra estrutural:** proibir `http.Client` fora de `internal/fetch`. Adicionar ao `.golangci.yml` (via `forbidigo` ou `depguard`) para que o CI barre reincidência. Uma defesa que depende de disciplina humana volta a falhar.

### Testes

- `import --opml http://127.0.0.1:9/` → recusado.
- `import --opml http://169.254.169.254/` → recusado.
- `import --opml http://10.0.0.1/` → recusado.
- OPML remoto legítimo que redireciona para IP privado → recusado no hop.
- Corpo acima do limite → truncado/rejeitado, sem OOM.

---

# PARTE II — MIGRAÇÃO PARA A v2026.7.1

## PR 2 — Verificação da integração (bloqueante da migração)

Registrar em `docs/openclaw-integration.md`:

1. **Sintaxe do schedule kind `on-exit`.** Nome exato da flag, como declarar o comando observado, como o agente recebe o resultado.
2. **Filtro por exit code.** O `on-exit` dispara em qualquer saída, ou permite filtrar? **Determina o desenho do PR 4.**
   - Se **filtra:** acordar o agente só em `0` e `20`. O LLM nunca é invocado num dia sem novidade.
   - Se **não filtra:** o skill precisa abortar na primeira instrução ao ler o relatório. Funcional, mas mais frágil.
3. **Runs direcionados a sessão** com detach limpo — como vincular a uma sessão fixa e nomeada.
4. **Configuração de `utilityModel`** (default e por agente) em `openclaw.json`.

**Gate:** documento aprovado.

---

## PR 3 — `feat(fetch)!: exit codes semânticos, relatório JSON e lockfile`

Transforma o `fetch` no contrato do pipeline. **Breaking** — usar `feat!:`.

### Estado atual

`main()` faz `os.Exit(1)` para qualquer erro. `runFetch` retorna `nil` mesmo quando feeds falharam — **sucesso parcial hoje é exit 0**, indistinguível de sucesso total. E "nada novo" também é exit 0. O agente não tem como saber se vale a pena acordar.

A boa notícia: `fetch.Result` já carrega tudo que é preciso (`FeedURL`, `Status`, `NotModified`, `NewArticles`, `Err`). Falta agregar e propagar.

### 3.1 Exit codes

| Code | Significado | Consequência |
|---|---|---|
| `0` | Sucesso, **há artigos novos não lidos** | Agente prossegue |
| `10` | Sucesso, **nada novo** | Agente **não** é invocado |
| `20` | Parcial: alguns feeds falharam, mas há artigos novos | Prossegue; menciona os feeds com erro |
| `30` | Falha total de rede / nenhum feed alcançável | Notifica erro; sem LLM |
| `40` | Erro de config / banco inacessível | Notifica; sugere `feedclaw doctor` |
| `50` | Fetch concorrente já em execução (lock) | Aborta sem efeito colateral |

**Implementação.** Como o Cobra converte o retorno de `RunE` num `os.Exit(1)` genérico via `main()`, defina um tipo de erro que carregue o código:

```go
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }
```

E em `main()`:

```go
if err := rootCmd().Execute(); err != nil {
	var ee *exitError
	if errors.As(err, &ee) {
		if ee.err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "error:", ee.err)
		}
		os.Exit(ee.code)
	}
	_, _ = fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
```

Códigos de sucesso (`10`) não devem imprimir "error:" no stderr — trate `ee.err == nil` como saída limpa com código não-zero.

> ⚠️ **Auditar todos os chamadores neste mesmo PR.** Exit `10` em "sucesso sem novidade" quebra tudo que trate não-zero como falha: `set -e`, `skill/scripts/feedclaw.sh`, targets do Makefile, o CI, o SonarQube. O wrapper precisa **propagar** o código — não engoli-lo nem convertê-lo em `1`.

### 3.2 `--report <path>`

Sempre escrito, inclusive em falha. Default: `$XDG_CONFIG_HOME/feedclaw/last_run.json` — coerente com `dbPath()`, que usa `os.UserConfigDir()`.

```json
{
  "schema_version": 1,
  "run_id": "01J...",
  "started_at": "2026-07-15T07:00:00-03:00",
  "finished_at": "2026-07-15T07:00:42-03:00",
  "exit_code": 0,
  "feeds_total": 47,
  "feeds_ok": 45,
  "feeds_304": 31,
  "feeds_failed": 2,
  "failed_feeds": [
    {"url": "https://…", "title": "…", "error": "timeout", "error_count": 3}
  ],
  "articles_new": 23,
  "unread_total": 61
}
```

`unread_total` exige uma chamada a `st.CountUnread()` (já existe — usada pelo `doctor`).

### 3.3 Lockfile

Não existe hoje. Criar em `$XDG_CONFIG_HOME/feedclaw/fetch.lock` com PID e timestamp. Segundo `fetch` concorrente sai com `50` sem tocar no banco. Lock stale (processo morto) é reclamado automaticamente.

Relevante porque o `on-exit` pode coincidir com um `fetch` manual, ou com o `POST /api/fetch` disparado pela UI.

### 3.4 Testes

Um cenário por código: sucesso com novidade; sucesso sem novidade; parcial (um feed 500); falha total; banco inacessível; lock ativo. O relatório JSON deve ser válido nos seis.

---

## PR 4 — `feat(skill): separar fluxo de fetch e fluxo de digest`

Hoje o `SKILL.md` é monolítico: a Seção 1 manda o agente rodar `fetch` → `unread` → clusterizar → `save` numa tacada. O LLM é acordado para esperar rede, e é acordado mesmo quando não há nada.

### Estrutura: dois skills

O OpenClaw descobre um skill sempre que um `SKILL.md` aparece sob um root configurado (até 6 níveis), e o nome vem do frontmatter, não da pasta:

```
skill/
├── feedclaw/SKILL.md           # comandos conversacionais (Seções 2 e 4 atuais)
├── feedclaw-digest/SKILL.md    # Fluxo B — acordado por on-exit
└── scripts/feedclaw.sh
```

O **Fluxo A** (fetch) não precisa de skill: é o cron chamando o binário direto, sem agente.

**`feedclaw-digest`** — primeira instrução, antes de qualquer outra:

> Leia o exit code do fetch (ou o campo `exit_code` do relatório em `$FEEDCLAW_REPORT`). Se for `10`, `30`, `40` ou `50`, **aborte imediatamente** — não chame `unread`, não clusterize. Em `30` e `40`, notifique o usuário do erro.

Só em `0` ou `20`: `unread --since 24h --json` → clusterizar em 4–8 temas → `digest save` → notificar. Em `20`, mencionar os feeds que falharam (de `failed_feeds`).

Incluir a **tabela de exit codes** dentro do próprio SKILL.md.

As diretrizes de clusterização atuais (afinidade com o stack, `category` como sinal, nomes específicos, resumo de 2–4 frases em pt-BR) estão boas — **preservar na íntegra**. Idem os mapeamentos conversacionais e a Seção 4 de segurança.

### Correção de caminho

O SKILL.md atual diz "chame o CLI pelo wrapper `scripts/feedclaw.sh`" — caminho relativo. O correto é `{baseDir}/scripts/feedclaw.sh`; o agente resolve `{baseDir}` contra o diretório do próprio skill, sem hardcode.

---

## PR 5 — `feat(skill): cron on-exit e sessão dedicada`

Substituir, no `SKILL.md` (Seção 3), no README e em `docs/INSTALL.md`:

```sh
# ANTES — remover
openclaw cron add --name feedclaw-digest --schedule "0 7 * * *" \
  --task "Execute o fluxo de digest diário do FeedClaw conforme o SKILL.md"
```

pelo par **cron por horário (executa só o binário) + `on-exit` (acorda o agente)**, com a sintaxe confirmada no PR 2.

Vincular o Fluxo B a uma sessão fixa e nomeada ("FeedClaw — Digest diário"). A Control UI da v2026.7.1 permite fixar, agrupar, renomear e marcar sessões como lidas — o digest se beneficia de morar sempre no mesmo lugar, e o título de sessão gerado automaticamente deixa de poluir.

**Nota operacional:** o watcher detecta mudanças em `SKILL.md` e atualiza o snapshot no próximo turno. Se o watcher estiver desabilitado ou a sessão for antiga, iniciar sessão nova ou `openclaw gateway restart`.

---

## PR 6 — `feat(skill): tiering de utilityModel`

- **Clusterização de títulos + resumo dos temas → `utilityModel`.** Agrupar 30–60 títulos e escrever 4–8 resumos curtos é classificação, não raciocínio profundo.
- **Drill-down, `full`, busca → modelo principal.** É onde a qualidade importa.

O plugin **não hardcoda modelo algum** — respeita o tiering configurado no OpenClaw. O campo `model_note` do `digest save` (já existe) serve para registrar qual modelo gerou cada digest: útil justamente para comparar qualidade entre tiers.

**Risco a validar antes de fechar:** a qualidade do agrupamento é o que justifica largar o Feedly. Se os temas do `utilityModel` saírem genéricos ou mal agrupados, promover a clusterização ao modelo principal e registrar a decisão. Não economizar centavos ao custo da curadoria.

---

# PARTE III — BUGS E CHORES

## PR 7 — `fix: UTF-8, timezone e help do --db`

### 7.1 🟠 `truncate()` corrompe UTF-8

`cmd/feedclaw/cmd_feeds.go`:

```go
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n-1]) + "…"   // ← fatia BYTES, não runas
}
```

`len()` conta bytes e `s[:n-1]` fatia bytes. Num leitor de RSS **em português**, títulos com acento serão cortados no meio de um caractere multibyte, produzindo UTF-8 inválido no terminal. Usado em `printFeedTable` e `printArticleTable` — ou seja, em toda saída de tabela.

```go
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n-1])) + "…"
}
```

Teste com título real: `"Símbolo de operação não é ação — atenção"`.

### 7.2 🟠 `today()` em UTC com usuário em UTC−3

`cmd/feedclaw/cmd_digest.go`:

```go
func today() string { return time.Now().UTC().Format("2006-01-02") }
```

Entre **21:00 e meia-noite no horário de Brasília**, `today()` já retorna a data de amanhã. Consequências:

- `digest save` sem `--date` grava o digest na data errada.
- `digest show` sem `--date` procura uma data futura, não encontra, cai no fallback `LatestDigestDate()` e **acerta por acidente**.
- O `header` do `printDigest` compara `d.Date == today()` e, à noite, deixa de dizer "Notícias para hoje" — dizendo "Notícias de 2026-07-15" enquanto ainda é dia 14 para o usuário.

O SKILL.md agrava: instrui `--date "$(date -u +%F)"`, forçando UTC também no shell.

Como a leitura noturna é o cenário provável, usar timezone local:

```go
func today() string { return time.Now().Local().Format("2006-01-02") }
```

Ou tornar configurável (`FEEDCLAW_TZ`). Decidir e **documentar a escolha** — o importante é que engine, SKILL.md e UI concordem. Ajustar o SKILL.md para `date +%F` (sem `-u`) se optar por local.

### 7.3 🔵 Help do `--db` contradiz o código

`main.go` diz `"default: XDG data dir"`, mas `dbPath()` chama `os.UserConfigDir()` — que é o XDG **config** dir (`~/.config`), não o data dir (`~/.local/share`). Um dos dois está errado. O README diz `$XDG_CONFIG_HOME/feedclaw/feedclaw.db`, então o **help** é que está errado. Corrigir o texto (ou mover para `os.UserCacheDir()`/data dir e ajustar tudo — mas isso quebraria bancos existentes; preferir corrigir o help).

---

## PR 8 — `chore: licença, release e install pinado`

1. **Definir a licença.** O README diz `TBD`. Repositório público sem licença é, por default, "todos os direitos reservados" — não é open source e não deve ser publicável no ClawHub. MIT é o default razoável. O frontmatter de skill aceita um campo `license` — preencher também.

2. **`install` aponta para `@latest`.** No frontmatter:
   ```yaml
   install:
     - id: go
       module: github.com/marceloalmeidadev/feedclaw/cmd/feedclaw@latest
   ```
   Para uso pessoal, tudo bem. **Se publicar no ClawHub, um `@latest` não fixado é cheiro de supply chain:** quem instala recebe o que estiver no `main` naquele instante — sem revisão, sem checksum, sem imutabilidade. Fixar numa tag assim que houver release.

3. **Publicar a primeira release.** `make package` já produz bundle + `SHA256SUMS` + tarball; falta a tag e o release no GitHub. SemVer.

4. **Corrigir o link do README:** "OpenClaw", no primeiro parágrafo, aponta para `marceloalmeidadev/feedclaw` em vez de `openclaw/openclaw`.

---

## Ordem de execução

```
PR 0 (guard zero-value)  ──►  PR 1 (SSRF no import)     ── segurança, primeiro
                                    │
PR 2 (verificação)  ────────────────┼──►  PR 3 (fetch!: exit codes/report/lock)
                                    │           │
                                    │           └──►  PR 4 (skill: dois fluxos)
                                    │                     ├──►  PR 5 (cron on-exit)
                                    │                     └──►  PR 6 (utilityModel)
                                    │
PR 7 (bugs)  ───────────────────────┴─  independente, pode ir a qualquer momento
PR 8 (chores) ──────────────────────────  independente
```

### A janela perigosa

**Entre o PR 3 e o PR 4, o cron antigo pode receber um exit `10` e interpretá-lo como falha.** Duas saídas:

- **Preferida:** PR 3 → PR 4 → PR 5 em sequência rápida, na mesma branch de trabalho, trocando o cron só no final.
- **Alternativa:** manter um flag `--legacy-exit` no `fetch` durante a transição, removido no PR 5.

---

## Fora de escopo (registrado para não perder)

- **Migração para code-plugin** usando o contrato durável de session-transcript do SDK (PR #95030 do OpenClaw). Reabrir só se a invocação via CLI se mostrar frágil na prática. Hoje ela funciona.
- **Entrega do digest por canal** (Telegram / WhatsApp).
- **`openclaw attach`, terminais de workspace, ClawRouter:** úteis para desenvolver, irrelevantes para o produto.
- **`security.installPolicy` do OpenClaw:** não é código do FeedClaw, mas vale ligar no seu ambiente — é um comando de política local que roda antes de qualquer instalação de skill (ClawHub, Git, upload, update) e **falha fechado**. Defesa estrutural contra skills maliciosos distribuídos como markdown.
