# Diagrams as Code

## Mermaid

```mermaid
graph TD
A[Client] --> B[Load Balancer]
B --> C[Server01]
B --> D[Server02]
```

## Plant UML

```puml
@startuml
title CP Theme
'skinparam handwritten true
skinparam {
    ArrowColor Black
    NoteColor Black
    NoteBackgroundColor White
    LifeLineBorderColor Black
    LifeLineColor Black
    ParticipantBorderColor Black
    ParticipantBackgroundColor Black
    ParticipantFontColor White
    defaultFontStyle Bold
}

== 1. title ==

"Dev Machine"->Github: commit and push
Github->Jenkins: call webhook,\ntrigger build

Jenkins->"Build Slave": automated trigger:\ncommit

== 2a. image scan ==

Jenkins->"Build Slave": automated trigger:\nimage scan
@enduml
```

## C4 Container Diagram

See [Mermaid's C4 Syntax](https://mermaid.js.org/syntax/c4c.html) and
[C4 Plant UML](https://github.com/plantuml-stdlib/C4-PlantUML/blob/master/README.md).

```puml
@startuml C4_Elements
!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Container.puml

Person(personAlias, "Label", "Optional Description")
Container(containerAlias, "Label", "Technology", "Optional Description")
System(systemAlias, "Label", "Optional Description")

Rel(personAlias, containerAlias, "Label", "Optional Technology")
@enduml
```
