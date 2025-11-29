# PlantUML интеграция в go-markdown-server

## Какво е добавено

Проектът вече поддържа рендериране на PlantUML диаграми директно в Markdown постовете.

## Как работи

1. **PlantUML Server** - Docker контейнер с PlantUML сървър (`plantuml/plantuml-server:jetty`)
2. **PlantUML модул** - Go код който намира PlantUML блокове в Markdown и ги заменя с изображения
3. **Автоматична обработка** - Всички постове се обработват преди да се покажат

## Използване

### Вграждане на PlantUML диаграми

В Markdown текста използвайте синтаксиса:

\`\`\`plantuml
Alice -> Bob: Hello
Bob -> Alice: Hi there!
\`\`\`

Това автоматично ще се рендерира като изображение на диаграмата.

### Примери

#### 1. Sequence Diagram

\`\`\`plantuml
@startuml
actor User
participant "Web Browser" as Browser
participant "Go Server" as Server
participant "MongoDB" as DB
participant "PlantUML Server" as PlantUML

User -> Browser: Visit blog post
Browser -> Server: GET /post/example
Server -> DB: Query post data
DB -> Server: Return post with PlantUML code
Server -> PlantUML: Send PlantUML code
PlantUML -> Server: Return PNG diagram
Server -> Browser: HTML with embedded diagram
Browser -> User: Display rendered post
@enduml
\`\`\`

#### 2. Class Diagram

\`\`\`plantuml
@startuml
class Post {
  +string Title
  +string Body
  +string URL
}

class Database {
  +GetPosts()
  +GetPostByName()
  +InsertPost()
}

class Router {
  +indexHandler()
  +mdNamedHandler()
  +addHandler()
}

Router --> Database : uses
Database --> Post : manages
@enduml
\`\`\`

#### 3. Component Diagram

\`\`\`plantuml
@startuml
package "Docker Compose" {
  [go-markdown-server] as Web
  [MongoDB] as DB
  [PlantUML Server] as UML
}

Web --> DB : stores posts
Web --> UML : renders diagrams

actor User
User --> Web : HTTP requests
@enduml
\`\`\`

#### 4. Activity Diagram

\`\`\`plantuml
@startuml
start
:User writes Markdown post;
:Includes PlantUML code block;
:Saves to database;
:User requests post;
:Server fetches from DB;
:Server finds PlantUML blocks;
if (PlantUML block found?) then (yes)
  :Send to PlantUML server;
  :Receive PNG image;
  :Replace block with image URL;
else (no)
  :Keep original Markdown;
endif
:Convert Markdown to HTML;
:Return to user;
stop
@enduml
\`\`\`

## Стартиране

### С Docker Compose (препоръчително)

\`\`\`bash
cd /home/vav3sf/Projects/samiavasil/go-markdown-server
docker-compose up -d
\`\`\`

Това ще стартира:
- Go сървър на http://localhost:8080
- MongoDB на localhost:27017
- PlantUML Server на http://localhost:8081

### Добавяне на пост с PlantUML

\`\`\`bash
curl -X GET "http://localhost:8080/add?title=PlantUML%20Demo&url=plantuml-demo&body=Check%20out%20this%20diagram%3A%0A%0A%60%60%60plantuml%0AAlice%20-%3E%20Bob%3A%20Hello%0ABob%20-%3E%20Alice%3A%20Hi%21%0A%60%60%60&key=124252"
\`\`\`

### Проверка

Отворете http://localhost:8080/post/plantuml-demo за да видите рендерираната диаграма!

## Технически детайли

### Архитектура

1. Markdown съдържа \`\`\`plantuml блок
2. `plantuml.ProcessPlantUMLSimple()` намира всички PlantUML блокове
3. PlantUML кодът се енкодва в base64
4. Генерира се URL към PlantUML сървъра: `http://plantuml:8080/png/{encoded}`
5. Блокът се заменя с Markdown синтаксис за изображение: `![PlantUML Diagram](url)`
6. Blackfriday конвертира целия Markdown в HTML
7. HTML се показва в браузъра с рендерираните диаграми

### Файлова структура

\`\`\`
go-markdown-server/
├── docker-compose.yml      # Добавен PlantUML контейнер
├── plantuml/
│   └── plantuml.go         # PlantUML обработка
├── routes.go               # Интегрирана PlantUML обработка
├── main.go
├── db/
│   └── datebase.go
└── PLANTUML_USAGE.md       # Тази документация
\`\`\`

## Troubleshooting

### PlantUML сървърът не работи

\`\`\`bash
# Проверка дали контейнерът работи
docker ps | grep plantuml

# Проверка на логовете
docker logs plantuml

# Рестарт на PlantUML контейнера
docker-compose restart plantuml
\`\`\`

### Диаграмите не се рендерират

1. Уверете се, че PlantUML блокът е правилно форматиран
2. Проверете дали PlantUML сървърът е достъпен: http://localhost:8081
3. Проверете environment променливата `PLANTUML_SERVER` в docker-compose.yml

## PlantUML референция

Пълна документация: https://plantuml.com/

Поддържани типове диаграми:
- Sequence diagrams
- Use case diagrams
- Class diagrams
- Activity diagrams
- Component diagrams
- State diagrams
- Object diagrams
- Deployment diagrams
- Timing diagrams
- И много други...
