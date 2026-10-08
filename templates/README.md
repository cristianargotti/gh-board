# Board do time

Este é o template de board do GitHub Projects que o kit `gh board` entende. Ele vem com os campos, as views, os workflows, os gráficos de insights e os formulários de issue que um time usa para planejar, executar e prestar contas do trabalho, com agentes de IA (Claude Code, Cursor e Codex) operando o board em nome de cada pessoa, sem token compartilhado e sem caminho destrutivo.

A referência deste template é um board de exemplo (`acme/7`, repositório `acme/app`). O arquivo `board.yml` ao lado descreve, chave por chave, o que cada campo significa; os formulários em `forms/` nascem prontos para o repositório do time.

## Como o board é organizado

| Peça | No board | O que é |
| --- | --- | --- |
| Épico | tipo de issue `Feature`, campo `Épico`, datas de início e objetivo | Um compromisso de entrega. Uma issue por entrega, com PRD e ADR quando houver. |
| Tarefa | tipo de issue `Task`, sub-issue do épico, `Estimativa (dias)` até 3 | O que fica pronto em até três dias, com responsável, datas e critério de pronto. |
| Frente | campo `Frente` | A linha de trabalho. A tarefa herda a frente do épico quando ninguém preenche. |
| Sprint | campo de iteração `Sprint` | A janela de execução. O kit calcula dias restantes, abertos e concluídos. |
| Marco | milestones do repositório | Datas de compromisso externo, mostradas no roadmap junto com os épicos. |
| Datas | `Start date` e `Target date`, campos de issue da organização | Alimentam o roadmap, o alerta de atraso e o digest. Uma mudança aparece em todo projeto que mostra a issue. |
| Entrada | etiqueta `entrada`, campo `Decisão`, SLA de 2 dias úteis, etiqueta `urgente` | A fila de triagem. Tudo que chega de fora entra aqui e recebe uma decisão: Resolver, Ensinar, Construir capacidade, Lab, Encaminhar, Recusar ou Adiar. |
| Laboratório | etiqueta `lab`, campos `Porta` e `Resultado` | Experimentos com hipótese, métricas e decisão escritas antes de rodar. |
| Impedimento | campo `Situação` | Marca o item parado e por quê. Dispara o alerta de bloqueio. |

Os campos `Origem`, `Squad`, `Tema`, `Causa`, `Área`, `Delivery Jira` e `Epic Jira` também vêm na cópia e aparecem como estão; o kit não lhes dá papel nenhum.

## Status e fluxo

| Classe | Opções | O que significa para o kit |
| --- | --- | --- |
| backlog | `BACKLOG` | Ainda não priorizado. |
| ready | `READY TO DEV` | Pronto para começar. |
| active | `IN PROGRESS`, `TEST / VALIDATION` | Em execução; conta no limite de WIP (6 e 5). |
| done | `DONE` | Entregue. Mover para cá fecha a issue pelo workflow do board, por isso vira plano e só executa com `gh board apply`. |

Transições permitidas: `READY TO DEV` para `IN PROGRESS`; `IN PROGRESS` para `TEST / VALIDATION` ou de volta a `READY TO DEV`; `TEST / VALIDATION` para `DONE` ou de volta a `IN PROGRESS`. As opções `DISCOVERY` e `PRD` existem no board e aparecem como estão, sem classe.

## Views, roadmap e insights

As views (tabela, quadro por status, roadmap com iterações, marcos e datas como marcadores) e os gráficos de insights chegam com a cópia do template e continuam funcionando porque o kit mantém os dados que elas leem: épicos, sub-issues, frentes, sprints, marcos e datas. O kit não edita views, workflows nem gráficos; o GitHub não oferece API para ler insights ou criar gráficos, então as medições do kit (`status`, `epics`, `roadmap`, `attention`, digest) saem dos itens.

Os workflows chegam ligados como estavam no template, menos o de auto-add, que o GitHub não copia. Revise com o PM quais ficam ativos antes do time usar o board: o kit trata "mover para DONE" como fechar a issue justamente por causa do workflow que faz isso.

## Formulários de issue

| Formulário | Quando usar | Tipo e etiquetas |
| --- | --- | --- |
| Épico | Um compromisso de entrega novo. | `Feature` |
| Tarefa | Um pedaço de até três dias de um épico. | `Task` |
| Oportunidade | Uma área quer usar IA para alguma coisa. | `Task`, `entrada`, `oportunidade` |
| Erro | Um sistema do time deu resultado errado. | `Bug`, `entrada`, `erro` |
| Regra | Mudar o que um sistema sabe ou faz. | `Task`, `entrada`, `regra` |
| Sinal de operação | O time observou algo que vai quebrar ou encarecer. | `Task`, `entrada`, `sinal` |
| Experimento | Um teste controlado no laboratório. | `Task`, `lab` |

Os campos `type` exigem os tipos de issue da organização (`Feature`, `Task`, `Bug`). Todas as etiquetas dos formulários, mais `urgente`, estão em `template.labels` do `board.yml` e são criadas pelo plano de `gh board init` quando faltam. `template.milestones` fica vazio para cada time escolher seus marcos. Sem tipos de issue na organização de destino, a renderização remove o campo `type` dos formulários.

## Rotina com o kit

| Momento | Comando | O que acontece |
| --- | --- | --- |
| Começo do dia, ou do agente | `gh board context` | Regras do time, sprint, meus itens, atenção, fila de entrada, épicos e entregas da semana, com `completeness` e o que foi omitido. |
| Daily | `gh board standup`, `gh board attention` | Por pessoa: moveu ontem, ativo hoje, bloqueado. Atrasados, bloqueados, entrada fora do SLA, WIP estourado. |
| Criar trabalho | `gh board new task --title "..." --parent "#17"` | Cria a issue no repositório com o tipo certo, coloca no board, preenche os campos e liga ao épico. `new epic` e `new entry` fazem o mesmo para épicos e itens de entrada. |
| Mover | `gh board move "#17" "IN PROGRESS"` | Valida a transição e o WIP e escreve. Para `DONE` o comando grava um plano; uma pessoa executa com `gh board apply <id>`. |
| Fechar | `gh board close "#17"` | Grava um plano; `gh board apply <id>` fecha. |
| Semana | `gh board tidy`, `gh board digest print`, `gh board digest post` | `tidy` grava o plano da arrumação (frente e épico herdados, sprint pela data objetivo, data de início, status do épico). `digest` imprime a semana; `digest post` grava o plano que publica o status update. |
| Alertas | `gh board watch --install --project <owner>/<número>` | Avisos na área de notificação do sistema a cada dez minutos, sem servidor nem bot. |

O agente lê o board e faz as escritas reversíveis em seu nome; o que é consequente vira plano e você executa no terminal. Toda escrita fica atribuída a você no GitHub e registrada localmente.

## Como copiar para o seu time

1. `gh board init --from <owner>/<número> --repository <owner>/<repo> --forms` grava um plano que copia o template, cria as etiquetas do `board.yml`, vincula o repositório do time e escreve o rascunho do `board.yml`; depois do apply, `--forms` deixa os formulários em `.github/ISSUE_TEMPLATE/` ao lado do rascunho escolhido por `--output` para um pull request. `gh board apply <id>` executa.
2. Edite o `board.yml` a partir deste modelo e rode `gh board doctor` até não sobrar divergência.
3. Siga o checklist de `docs/onboarding.md`, que mede o tempo de cada passo.

## O que o kit troca ao renderizar os formulários

Os formulários são escritos com os nomes de referência deste `board.yml`. Ao renderizar para outro time, `gh board init --forms` depois do apply substitui cada nome de referência pelo valor da mesma chave no `board.yml` do time:

| No formulário | Chave em `board.yml` | Valor de referência |
| --- | --- | --- |
| rótulo e campo `Frente` | `capabilities.lane.field` | `Frente` |
| campo `Épico` | `capabilities.epic.field` | `Épico` |
| `type: Feature` | `capabilities.epic.issue_type` | `Feature` |
| `type: Task` | `capabilities.task.issue_type` | `Task` |
| campo `Estimativa (dias)` | `capabilities.estimate.field` | `Estimativa (dias)` |
| `Start date` e `Target date` | `capabilities.dates.start` e `capabilities.dates.target` | `Start date` e `Target date` |
| etiqueta `entrada` | `capabilities.triage.label` | `entrada` |
| etiqueta `urgente` | `capabilities.triage.urgent_label` | `urgente` |
| campo `Decisão` | `capabilities.triage.decision_field` | `Decisão` |
| etiqueta `lab` | `capabilities.lab.label` | `lab` |
| campos `Porta` e `Resultado` | `capabilities.lab.gate_field` e `capabilities.lab.result_field` | `Porta` e `Resultado` |
| URL do board em `config.yml` | `project` | `acme/7` |

Nomes de sistemas, frentes e épicos patrocinadores são texto livre nos formulários, para que o mesmo arquivo sirva a qualquer time.
