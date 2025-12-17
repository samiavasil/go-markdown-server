# Test PlantUML Integration

This is a test document with PlantUML diagrams.

## Simple Sequence Diagram

```plantuml
@startuml
Alice -> Bob: Authentication Request
Bob --> Alice: Authentication Response

Alice -> Bob: Another authentication Request
Alice <-- Bob: Another authentication Response
@enduml
```

## Class Diagram

```plantuml
@startuml
class User {
  +String name
  +String email
  +login()
  +logout()
}

class Admin {
  +String role
  +manageUsers()
}

User <|-- Admin
@enduml
```

## Component Diagram

```plantuml
@startuml
package "Web Application" {
  [Frontend] --> [Backend API]
  [Backend API] --> [Database]
}

[Frontend] --> [PlantUML Server]
@enduml
```

## Regular Markdown

This is regular markdown text that should render normally.

- List item 1
- List item 2
- List item 3

**Bold text** and *italic text*.
