# epub-translator

Envie um EPUB, acompanhe a tradução **capítulo a capítulo** e baixe o arquivo traduzido — em [Amarra](https://github.com/puppe1990/amarra-cais) (HTML + Drive + SSE + SQLite).

- `GET /` — envio (multipart) + lista de livros com progresso
- `GET /books/{id}` — progresso ao vivo (**SSE** morfa `#book-progress` e `#chapter-list`), tabela de capítulos com status/tentativas/erro
- `POST /books/{id}/retry` — roda a fila (pendentes + falhos)
- `POST /books/{id}/pause` — cancela a chamada atual e pausa o livro, preservando os capítulos concluídos
- `POST /books/{id}/chapters/{cid}/retry` — devolve um capítulo falho à fila e roda
- `GET /books/{id}/download` — reconstrói o EPUB com os capítulos já traduzidos (o que faltar sai no idioma original)
- `GET|POST /settings` — endpoint OpenAI-compatível (base URL, modelo e chave); `POST /settings/test` valida na hora

## Como funciona

1. **Upload** → `internal/epub.Parse` lê o zip: metadados (título/autor/idioma), documentos XHTML do spine (+ nav) e o miolo de `<body>` de cada um. Cada documento vira um **capítulo** em `chapters` (unidade de chamada e de retry).
2. **Tradução** → `internal/pipeline.Runner` processa a fila (pendentes + falhos, em ordem) em um goroutine por livro; cada capítulo é gravado como `running → done|failed` (com `attempts` e a mensagem de erro). Restart do servidor devolve `running` para a fila; o retry manual retoma.
3. **Validação** → `internal/translate.ValidateBody` só aceita resposta com as mesmas tags (sequência normalizada), texto realmente traduzido, tamanho plausível e sem preâmbulo; falhou? o `Service` retenta com um lembrete (3 tentativas), depois marca o capítulo como `failed`.
4. **Download** → `internal/epub.Rebuild` copia o zip original trocando apenas o miolo de `<body>` dos capítulos traduzidos e o `dc:language` do OPF; o resto (CSS, imagens, nav) passa intacto. O arquivo é montado a partir do banco na hora do download.

Na página do livro, **Pausar** interrompe a requisição atual e salva o estado **Pausado**. O capítulo interrompido volta à fila sem contar como falha. **Retomar tradução** continua pelos pendentes e falhos; os concluídos permanecem intactos. A pausa sobrevive ao reinício do servidor, e os controles acompanham o estado ao vivo por SSE.

A lista mostra **tempo**, **provedor** e **modelo** da última execução de cada capítulo. A configuração é capturada antes da chamada, e a duração inclui as retentativas automáticas daquela execução. O tempo em andamento atualiza por SSE; sucessos, falhas e pausas preservam a duração final. Ao retentar, esses campos passam a representar a nova execução. Traduções anteriores ao registro mostram “—”; seus dados não são inferidos pela configuração atual.

## Provedores (perfis com chave)

A página **/settings** mantém um **perfil por provedor**, cada um com a própria chave salva (tabela `providers`, chave mascarada na tela):

| Perfil       | Base URL (fixa no código)                                 | Modelo padrão                  |
| ------------ | --------------------------------------------------------- | ------------------------------ |
| `openai`     | `https://api.openai.com/v1`                               | `gpt-4.1-mini`                 |
| `gemini`     | `https://generativelanguage.googleapis.com/v1beta/openai` | `gemini-3.5-flash-lite`        |
| `groq`       | `https://api.groq.com/openai/v1`                          | `openai/gpt-oss-120b`          |
| `nvidia`     | `https://integrate.api.nvidia.com/v1`                     | `minimaxai/minimax-m3.1-flash` |
| `openrouter` | `https://openrouter.ai/api/v1`                            | `google/gemini-2.5-flash`      |
| `custom`     | livre (formulário)                                        | livre                          |

- **Trocar de provedor** = escolher uma opção em **Um provedor por vez** e clicar em **Ativar provedor selecionado**. Apenas um fica ativo; os demais mantêm suas chaves salvas. A mudança vale no próximo capítulo, mesmo no meio de um livro.
- **NVIDIA NIM** oferece um seletor pesquisável de modelos em `/settings`. A lista curada prioriza tradução e preservação de XHTML (Riva Translate, Mistral Large 4, DeepSeek V4.1 Flash, Gemma 4 31B, GPT-OSS 20B, Nemotron 3.5 Lightning, GLM-5.3 Flash). O Riva usa o par de idiomas no `system` (`en-pt-br`), tem contexto de 8 mil tokens e fatia capítulos grandes (~6 mil caracteres por chamada) para a tradução e a saída caberem na janela. Escolha e clique em **Salvar**: a chave existente é preservada e o modelo passa a valer no próximo capítulo. A disponibilidade depende da NVIDIA; um modelo já salvo fora da lista continua selecionável.
- **Groq e OpenRouter** também oferecem seletores pesquisáveis: o servidor consulta seus catálogos de modelos de texto e guarda a lista por 15 minutos. Modelos de áudio, geração de imagem, moderação e variantes batch ficam fora. Se o catálogo falhar ou ainda não houver chave, a tela mantém opções locais e o modelo já salvo. Escolher um modelo não muda o provedor ativo.
- **Testar a chave ativa** faz uma tradução mínima pelo endpoint real.
- Resolução: com um perfil ativo, usa exclusivamente sua configuração e exige sua própria chave. Sem perfil selecionado, usa a configuração legada (`translate_*`)/ambiente e os padrões. Não há troca automática para outro provedor quando o selecionado falha. Env: `TRANSLATE_BASE_URL`, `TRANSLATE_MODEL`, `TRANSLATE_API_KEY` (ou `OPENAI_API_KEY`).
- Só o perfil `custom` aceita base URL livre; os presets fixam a base para a chave não ser apontada a um endpoint arbitrário sem intenção.

> As chaves ficam em texto puro no SQLite — é um app pessoal. Em produção as envs do Cleat servem de fallback e o painel do Cleat mascara segredos.

## Desenvolvimento

```bash
amarra-cais install   # npm install + go mod tidy
amarra-cais dev       # http://localhost:8080 (auto-pick de porta)
amarra-cais test      # go test ./...
make ci               # test + golangci-lint + prettier --check
```

Dados ficam em `data/` (SQLite `data/app.db`, EPUBs enviados em `data/books/`), fora do git.

## Deploy (Cleat)

- Host: **https://epub-translator.apps.gestaobem.com** — app `epub-translator` (id 227), runtime `golang`, porta 4217, server `gestaobem-cx43`.
- Repo `puppe1990/epub-translator`, branch `main`, `auto_deploy` ligado: push em `main` publica.
- Deploy manual: `cleat__deploy app=epub-translator ref=main` (acompanhe com `cleat__deploy_status`).
- Envs de produção: `ENV=production`, `APP_URL`, `PORT=:4217`, `DB_PATH=/opt/epub-translator/data/app.db`, `STATIC_DIR=/opt/epub-translator/current/web/static`, `TRUSTED_PROXIES=127.0.0.1`, `ADMIN_TOKEN`, `LOCALE=pt` e `TRANSLATE_BASE_URL`/`TRANSLATE_MODEL`/`TRANSLATE_API_KEY` (provedor inicial; a página /settings sobrescreve por requisição).
- Build no servidor: `npm install` + Tailwind + `go build ./cmd/server` (contrato em `.cleat_deploy/deploy.json`, `binaries: ["server"]`).
