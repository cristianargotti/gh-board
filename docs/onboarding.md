# Onboarding de um time no gh board

Este checklist leva um time do zero a um board funcionando, com o agente respondendo `gh board context`. Quinze minutos é a meta a medir, não uma promessa: preencha a coluna "Tempo medido" em cada passo e registre o total no fim. O que for medido alimenta a próxima versão do kit.

Antes de começar, tenha em mãos: o `owner/número` do template (o board de referência ou o template da organização), o repositório onde o time cria issues e a lista de pessoas do time com seus logins no GitHub.

## Checklist

| # | Passo | Como fazer | Quem | Tempo medido |
| --- | --- | --- | --- | --- |
| 1 | Copiar o template | `gh board init --from <owner>/<número> --repository <owner>/<repo> --forms` grava um plano; `gh board apply <id>` executa: copia o projeto (views, campos, workflows menos o auto-add, insights), cria as etiquetas do `board.yml` de referência, vincula o repositório do time e escreve o rascunho de `board.yml`. Itens, colaboradores e vínculos de repositório não são copiados pelo GitHub. | PM | |
| 2 | Mapear o `board.yml` | Partindo do rascunho e de `templates/board.yml`, preencha `repository`, distribua as opções de status entre `backlog`, `ready`, `active` e `done`, declare épico, tarefa, frente, sprint, estimativa, datas, impedimento, entrada e laboratório, e os membros do digest. Rode `gh board doctor` até não sobrar divergência. Comite o arquivo no repositório do time. | PM e uma pessoa de engenharia | |
| 3 | Revisar os workflows ativos com o PM | No board copiado, abra Workflows e decida com o PM quais ficam ligados. O kit trata "mover para DONE" como fechar a issue porque um workflow faz isso; um workflow desligado ou diferente muda esse efeito. Ligue o auto-add se o time usa. | PM | |
| 4 | PR dos formulários | Os formulários renderizados em `.github/ISSUE_TEMPLATE/` vão em um pull request. Se `.github/` tem CODEOWNERS no repositório, o PR precisa da aprovação dos donos. Confira as etiquetas declaradas em `template.labels`, criadas pelo plano de init, e os tipos de issue disponíveis na organização. | Engenharia | |
| 5 | Pedidos de acesso | Cada pessoa do time precisa do papel `writer` no projeto e de permissão de escrita no repositório; o token do `gh` precisa dos escopos `repo` e `project` (`gh auth refresh -s project`). | PM | |
| 6 | Instalar o kit e os agentes | Cada pessoa: `gh extension install cristianargotti/gh-board`, depois `gh board agent install --agent all` (ou só o agente que usa) e `gh board doctor`. Quem usa Codex confirma o hook uma vez em `/hooks`; quem usa Claude Code executa `claude plugin marketplace add cristianargotti/gh-board` e `claude plugin install gh-board@gh-board`. | Cada pessoa | |
| 7 | Primeiro `context` | `gh board context` no terminal e depois no agente. Confira `completeness`: `complete` significa que tudo coube no orçamento; `partial` lista o que foi omitido por seção. | Cada pessoa | |
| | **Total** | | | |

## Registro da medição

| Campo | Valor |
| --- | --- |
| Time | |
| Data | |
| Quem mediu | |
| Tempo total | |
| Passo mais demorado e por quê | |
| O que faltou no kit | |

## Depois do onboarding

- `gh board watch --install --project <owner>/<número>` em pelo menos uma máquina do time liga os alertas na área de notificação (um poller por máquina, sem servidor nem bot). O projeto é obrigatório; também pode vir do `board.yml`.
- `gh board digest print` na sexta imprime a semana; `gh board digest post` grava o plano que publica o status update e uma pessoa executa com `gh board apply <id>`.
- `gh board tidy` antes da weekly grava o plano da arrumação; o mesmo `apply` executa.
- Leia `docs/security.md` com o time: o que o kit garante, o que fica fora e o que o modo estrito acrescenta.
