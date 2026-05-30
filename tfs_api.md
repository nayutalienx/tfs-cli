REST API для работы с бэклогом (TFS/Azure DevOps Server), API version = 6.0 — документация (Markdown)

Ниже приведены только endpoint’ы и структуры, которые описаны в официальной документации Microsoft (Microsoft Learn).
В примерах URL и параметров Microsoft часто показывает api-version=7.1; в вашем окружении используйте api-version=6.0 (как вы указали), сохраняя те же пути и параметры. Формат ...?_apis/...?...&api-version={version} и различия URL для TFS on-prem описаны у Microsoft.

1) Базовые соглашения
1.1. Базовый URL (Azure DevOps Services vs TFS on-prem)

Microsoft задаёт общую форму запроса:

VERB https://{instance}[/{team-project}]/_apis[/{area}]/{resource}?api-version={version}


и отдельно уточняет instance для TFS:

TFS: {server:port}/tfs/{collection}
(обычно порт 8080, collection часто DefaultCollection)


Примеры ниже даны в “облачном” формате, как у Microsoft (dev.azure.com/...). Для TFS on-prem меняется только {instance}.

2) Получение настроек команды, влияющих на бэклог
2.1. Получить Team Settings

Назначение: получить настройки команды, включая backlogIteration, bugsBehavior, видимость уровней бэклога и т. п.

HTTP

GET https://dev.azure.com/{organization}/{project}/{team}/_apis/work/teamsettings?api-version=6.0


Path/query параметры (как в Microsoft Learn):

{organization} (path, required) — организация

{project} (path, required) — проект (ID или имя)

{team} (path, optional/required по контексту URL) — команда (ID или имя)

api-version (query, required) — версия API

Request body: отсутствует.

Response: 200 OK — объект Team Setting

Ключевые поля (структура и описания из Microsoft Learn):

_links — ссылки на связанные ресурсы (Reference Links)

backlogIteration — “backlog iteration” (Team Settings Iteration)

backlogVisibilities — информация о категориях (уровнях), видимых на бэклоге (object)

bugsBehavior — поведение багов (Bugs Behavior: off|asRequirements|asTasks)

defaultIteration — итерация по умолчанию (Team Settings Iteration)

defaultIterationMacro — макрос итерации по умолчанию (string)

workingDays — рабочие дни (DayOfWeek[])

url — полный URL ресурса

Пример ответа у Microsoft включает поля backlogIteration, bugsBehavior, workingDays, backlogVisibilities и др.

3) Конфигурация бэклога (типы, уровни, поля ранжирования)
3.1. Получить Backlog Configuration

Назначение: получить конфигурацию бэклога команды/проекта: уровни (portfolio/requirement/task), типы WIT, колонки, а также отображение “типовых” полей поведения (например, поле сортировки/порядка).

HTTP

GET https://dev.azure.com/{organization}/{project}/{team}/_apis/work/backlogconfiguration?api-version=6.0


Response: 200 OK — объект Backlog Configuration

3.1.1. Backlog Configuration (Object)

Поля (официальные описания):

backlogFields (Backlog Fields) — маппинг “Field Type” → “Field Reference Name” (например Order, Activity)

bugsBehavior (Bugs Behavior) — поведение багов

hiddenBacklogs (string[]) — скрытые бэклоги

isBugsBehaviorConfigured (boolean) — настроено ли bugsBehavior в процессе

portfolioBacklogs (Backlog Level Configuration[]) — уровни portfolio

requirementBacklog (Backlog Level Configuration) — requirement backlog

taskBacklog (Backlog Level Configuration) — task backlog

workItemTypeMappedStates (Work Item Type State Info[]) — маппинг состояний типов WIT в категории состояний

url (string) — URL ресурса

3.1.2. Backlog Fields (Object)

typeFields (object) — маппинг “Field Type” → “Field Reference Name”

В примере Microsoft видно, что Order может быть сопоставлен с Microsoft.VSTS.Common.StackRank (а также встречаются Effort, RemainingWork, Activity).

3.1.3. Backlog Level Configuration (Object)

Поля:

id (string) — Backlog Id

name (string) — имя уровня бэклога

rank (int32) — ранг уровня (в т. ч. task backlog может иметь ранг 0)

type (Backlog Type: portfolio|requirement|task)

workItemCountLimit (int32) — максимум элементов, показываемых на бэклоге

isHidden (boolean) — скрыт ли уровень

color (string) — цвет уровня

workItemTypes (Work Item Type Reference[]) — типы work item, участвующие в этом уровне

defaultWorkItemType (Work Item Type Reference) — тип по умолчанию

addPanelFields (Work Item Field Reference[]) — поля в панели “Add”

columnFields (Backlog Column[]) — колонки по умолчанию

3.1.4. Backlog Column (Object)

columnFieldReference (Work Item Field Reference) — ссылка на поле

width (int32) — ширина колонки

3.1.5. Work Item Field Reference (Object)

referenceName (string) — reference name поля

name (string) — “friendly name” поля

url (string) — REST URL ресурса поля

3.1.6. Work Item Type Reference (Object)

name (string) — имя типа WIT

url (string) — REST URL типа

4) Уровни бэклога (Backlog Levels)
4.1. Список уровней бэклога (List all backlog levels)

HTTP

GET https://dev.azure.com/{organization}/{project}/{team}/_apis/work/backlogs?api-version=6.0


Response: 200 OK

В примере Microsoft ответ имеет обёртку:

{
  "count": 4,
  "value": [ /* Backlog Level Configuration[] */ ]
}


и каждый элемент value[] соответствует Backlog Level Configuration (поля id, name, rank, workItemCountLimit, addPanelFields, columnFields, workItemTypes, defaultWorkItemType, color, isHidden, type).

5) Получение work items внутри уровня бэклога
5.1. Получить work items для backlog level

Назначение: получить список элементов (work items) в рамках конкретного backlog level (например, Features / Stories / Epics и т. п.).

HTTP

GET https://dev.azure.com/{organization}/{project}/{team}/_apis/work/backlogs/{backlogId}/workItems?api-version=6.0


Path/query параметры:

{backlogId} (path, required, string) — ID уровня бэклога

остальные параметры {organization}, {project}, {team}, api-version — стандартные

Response: 200 OK — Backlog Level Work Items

Структура:

workItems (Work Item Link[]) — список элементов в backlog level

Work Item Link (Object)

rel (string) — тип ссылки

source (Work Item Reference) — исходный work item

target (Work Item Reference) — целевой work item

Work Item Reference (Object)

id (int32) — ID work item

url (string) — REST URL work item ресурса

Важно: этот endpoint возвращает ссылочные структуры/ID, а детали (поля, заголовок, состояние и т. п.) вы обычно догружаете через Work Item Tracking API (см. раздел 7).

6) Получение work items итерации (Iteration backlog)
6.1. Получить work items для конкретной итерации (Get iteration work items)

HTTP

GET https://dev.azure.com/{organization}/{project}/{team}/_apis/work/teamsettings/iterations/{iterationId}/workitems?api-version=6.0


Path/query параметры:

{iterationId} (path, required, uuid) — ID итерации

{organization}, {project}, {team}, api-version — стандартные

Response: 200 OK — Iteration Work Items

Поля:

_links (Reference Links) — ссылки

url (string) — URL ресурса

workItemRelations (Work Item Link[]) — связи/элементы итерации

Work Item Link / Work Item Reference — такие же по структуре (rel/source/target и id/url).

7) Изменение порядка элементов бэклога (ранжирование)
7.1. Reorder Product Backlog / Boards Work Items

Назначение: изменить порядок work items на Product Backlog/Boards.

HTTP

PATCH https://dev.azure.com/{organization}/{project}/{team}/_apis/work/workitemsorder?api-version=6.0


Request body — Reorder Operation (Object):

ids (int32[]) — IDs reorder’имых work items

previousId (int32) — ID элемента, который должен быть перед вставляемыми; 0 = начало списка

nextId (int32) — ID элемента, который должен быть после вставляемых; 0 = конец списка

parentId (int32) — Parent ID для всех элементов операции; 0 = без родителя

iterationPath (string) — используется только при reorder из Iteration Backlog

Пример запроса (из Microsoft):

{
  "parentId": 0,
  "previousId": 4,
  "nextId": 5,
  "ids": [1, 2, 3]
}


Response: 200 OK

В примере Microsoft ответ имеет обёртку:

{
  "count": 3,
  "value": [
    { "id": 1, "order": 1000102770 },
    { "id": 2, "order": 1000110675 },
    { "id": 3, "order": 1000118580 }
  ]
}


Reorder Result:

id (int32) — ID reorder’нутого work item

order (double) — обновлённое значение порядка

8) Work Item Tracking API: создавать/читать/обновлять элементы бэклога

Элементы бэклога — это work items (User Story / Product Backlog Item / Bug / Feature / Epic и т. п. — зависит от процесса).
Ниже — базовые endpoint’ы, которые Microsoft описывает для создания/чтения/обновления work items.

8.1. Создать Work Item (Create)

HTTP

POST https://dev.azure.com/{organization}/{project}/_apis/wit/workitems/${type}?api-version=6.0


Ключевое: Content-Type

application/json-patch+json

Request body — JSON Patch Document (модель JSON Patch Operations)

Каждая операция содержит:

op — тип операции (add|remove|replace|move|copy|test)

path — путь (в т. ч. поддержка индексов массивов и - для вставки в конец)

from — путь “копировать из” для move/copy

value — значение операции (primitive или “token”)

Response: 200 OK — Work Item

Work Item (как описано у Microsoft):

_links (Reference Links) — ссылки на связанные ресурсы

commentVersionRef — ссылка на конкретную версию комментария в этой ревизии

fields (object) — “Map of field and values for the work item.”

id (int32) — ID work item

relations (array) — “Relations of the work item.”

rev (int32) — revision number

url (string) — URL ресурса

8.2. Обновить Work Item (Update)

HTTP

PATCH https://dev.azure.com/{organization}/{project}/_apis/wit/workitems/{id}?api-version=6.0


Request body: такой же JSON Patch (application/json-patch+json) и те же поля операций (op/path/from/value).

Response: возвращается Work Item (та же структура, что и при Create).

8.3. Получить один Work Item (Get work item)

HTTP

GET https://dev.azure.com/{organization}/{project}/_apis/wit/workitems/{id}?api-version=6.0


Опциональные query параметры (как у Microsoft):

fields — список запрашиваемых полей

asOf — “AsOf UTC date time string”

$expand — варианты раскрытия (None|Relations|Fields|Links|All)

Response: Work Item

8.4. Получить список Work Items по ID (List, максимум 200)

HTTP

GET https://dev.azure.com/{organization}/{project}/_apis/wit/workitems?ids={ids}&api-version=6.0


Ключевые query параметры:

ids — список ID через запятую (максимум 200)

fields, asOf, $expand, errorPolicy — опционально (описаны у Microsoft)

Response: Work Item[]

8.5. Получить Work Items батчем (Get Work Items Batch, максимум 200)

HTTP

POST https://dev.azure.com/{organization}/{project}/_apis/wit/workitemsbatch?api-version=6.0


Request body (описан у Microsoft):

ids (int32[]) — список ID

fields (string[]) — запрашиваемые поля

$expand — параметры раскрытия

asOf — дата-время UTC

errorPolicy — политика ошибок (например, Fail/Omit)

Response: массив Work Item[]

9) Типовой “workflow” работы с бэклогом через REST

Получить конфигурацию бэклога и узнать, какие уровни/типы WIT используются, а также какое поле отвечает за Order (ранжирование)

Получить список backlog levels (/work/backlogs)

Получить work item ID внутри нужного backlog level (/work/backlogs/{backlogId}/workItems)

Догрузить детали по ID через /wit/workitems или /wit/workitemsbatch

Изменять порядок в бэклоге через /work/workitemsorder

Создавать/обновлять элементы через /wit/workitems/${type} (JSON Patch)