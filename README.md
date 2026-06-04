# ShotgunGo

A ShotGrid REST API client for Go, based on the [Flow Production Tracking REST API](https://developers.shotgridsoftware.com/rest-api/). Written with the assistance of Claude Code.

## Installation

```sh
go get github.com/rfletchr/ShotgunGo
```

## Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    sg "github.com/rfletchr/ShotgunGo"
)

type TaskAttributes struct {
    Content     string `json:"content"`
    Status      string `json:"sg_status_list"`
    ProjectName string `json:"project.Project.tank_name"`
    ShotCode    string `json:"entity.Shot.code"`
}

type EntityRef struct {
    ID   int    `json:"id"`
    Type string `json:"type"`
}

type TaskRelationships struct {
    TaskAssignees struct {
        Data []EntityRef `json:"data"`
    } `json:"task_assignees"`
}

type Task struct {
    ID            int               `json:"id"`
    Attributes    TaskAttributes    `json:"attributes"`
    Relationships TaskRelationships `json:"relationships"`
}

func main() {
    client := sg.NewClient(
        "https://your-studio.shotgunstudio.com",
        "your_script_name",
        "your_script_key",
    )

    ctx := context.Background()

    q := client.Find("Task",
        sg.Fields("content", "sg_status_list", "project.Project.tank_name", "entity.Shot.code"),
        sg.And(
            sg.Filter("project.Project.archived", sg.Is, false),
            sg.Or(
                sg.Filter("sg_status_list", sg.Is, "ip"),
                sg.Filter("sg_status_list", sg.Is, "rdy"),
            ),
        ),
        sg.Order(sg.OrderField{Field: "created_at", Direction: sg.Desc}),
    )

    for entity, err := range q.Iter(ctx) {
        if err != nil {
            log.Fatal(err)
        }
        var task Task
        if err := entity.Decode(&task); err != nil {
            log.Fatal(err)
        }
        fmt.Println(task.ID, task.Attributes.Content, task.Attributes.Status)
    }
}
```

## API

### Entity type names

Entity types can be passed in either PascalCase or plural snake_case — the API accepts both. PascalCase is recommended as it is consistent with schema responses and relationship objects.

| PascalCase (recommended) | Plural snake_case     |
|--------------------------|-----------------------|
| `Task`                   | `tasks`               |
| `Project`                | `projects`            |
| `HumanUser`              | `human_users`         |
| `CustomEntity01`         | `custom_entity_01s`   |

---

### Client

```go
client := sg.NewClient(baseURL, scriptName, scriptKey)
```

Authenticates using ShotGrid script credentials. Tokens are refreshed automatically.

---

### Find

```go
q := client.Find(entityType, options...)
```

Returns an immutable `*Query`. No network call is made at this point.

**Options:**

| Function | Description |
|----------|-------------|
| `sg.Fields("f1", "f2", ...)` | Fields to return, including deep links (`entity.Shot.code`) |
| `sg.Filter(field, relation, value)` | Single filter condition |
| `sg.And(conditions...)` | All conditions must match |
| `sg.Or(conditions...)` | Any condition must match |
| `sg.PageSize(n)` | Override the default page size (500) |
| `sg.Order(fields...)` | Sort by one or more fields |

**Filter relations** are typed constants on `sg.FilterRelation`:
`Is`, `IsNot`, `LessThan`, `GreaterThan`, `Contains`, `NotContains`, `StartsWith`,
`EndsWith`, `Between`, `NotBetween`, `In`, `NotIn`, `InLast`, `NotInLast`,
`InNext`, `NotInNext`, `InCalendarDay`, `InCalendarWeek`, `InCalendarMonth`,
`InCalendarYear`, `TypeIs`, `TypeIsNot`, `NameContains`, `NameNotContains`,
`NameStartsWith`, `NameEndsWith`.

**Ordering:**

```go
sg.Order(
    sg.OrderField{Field: "created_at", Direction: sg.Desc},
    sg.OrderField{Field: "code",       Direction: sg.Asc},
)
```

`sg.Asc` and `sg.Desc` are the only valid `OrderDirection` values.

### Query methods

```go
entity, err := q.One(ctx)                  // first result, or nil
entities, err := q.All(ctx)                // all results, walking pages
page, err := q.Page(ctx, 1)               // fetch a specific page (1-indexed)

for entity, err := range q.Iter(ctx) {    // range-over iterator, pages on demand
    ...
}
```

### Page

```go
page.Entities   // []Entity for this page
page.HasNext()  // true if another page follows
page.HasPrev()  // true if a previous page exists
page.Next(ctx)  // fetch the next Page (nil, nil if none)
page.Prev(ctx)  // fetch the previous Page (nil, nil if none)
```

### Entity

```go
entity.ID         // int
entity.Type       // e.g. "Task"
entity.Decode(&v) // unmarshal the full record into a typed struct
```

---

### Schema

```go
// All entity types visible in the instance.
types, err := client.EntityTypes(ctx)

// Entity types in the context of a specific project.
types, err := client.EntityTypes(ctx, projectID)
```

Returns `map[string]EntityType` keyed by PascalCase type name.

```go
type EntityType struct {
    Name    string
    Label   string
    Visible bool
}
```

```go
// All fields for an entity type.
fields, err := client.Fields(ctx, "Shot")

// Fields in the context of a specific project — required for accurate
// status values and other project-configured field properties.
fields, err := client.Fields(ctx, "Shot", projectID)
```

Returns `map[string]SchemaField` keyed by field name.

```go
type SchemaField struct {
    Name        string
    Label       string
    Description string
    DataType    string
    Editable    bool
    Mandatory   bool
    Visible     bool
    ValidValues []string // populated for list and status_list fields
    ValidTypes  []string // populated for entity and multi_entity fields
}
```

---

### Create

```go
entity, err := client.Create(ctx, entityType, fields)
```

`fields` is a flat `map[string]any`. Relationship fields use `NewEntityRef`:

```go
entity, err := client.Create(ctx, "Task", map[string]any{
    "content":        "Animation",
    "sg_status_list": "rdy",
    "project":        sg.NewEntityRef("Project", 123),
})
```

### Update

```go
entity, err := client.Update(ctx, entityType, id, fields)
```

Only fields present in the map are changed.

```go
entity, err := client.Update(ctx, "Task", 456, map[string]any{
    "sg_status_list": "ip",
})
```

### Delete

```go
err := client.Delete(ctx, entityType, id)
```

### NewEntityRef

```go
ref := sg.NewEntityRef("Project", 123)
// returns map[string]any{"type": "Project", "id": 123}
```

Use as a field value in `Create`, `Update`, and `Batch` requests.

---

### Batch

```go
results, err := client.Batch(ctx, requests...)
```

Executes multiple operations in a single request. Results are returned in the same order as the requests. Delete operations produce a zero-valued `Entity` in the result slice.

```go
results, err := client.Batch(ctx,
    sg.NewCreateRequest("Task", map[string]any{
        "content": "Animation",
        "project": sg.NewEntityRef("Project", 123),
    }),
    sg.NewUpdateRequest("Task", 456, map[string]any{
        "sg_status_list": "ip",
    }),
    sg.NewDeleteRequest("Task", 789),
)
```

---

### Upload

```go
// Upload from a file path — multipart used automatically for files > 5MB.
err := client.UploadFile(ctx, entityType, id, field, filePath, contentType)

// Upload from any io.Reader.
err := client.Upload(ctx, entityType, id, field, filename, contentType, r, size)
```

**Creating a Version and uploading a movie:**

```go
version, err := client.Create(ctx, "Version", map[string]any{
    "project": sg.NewEntityRef("Project", 123),
    "code":    "sc010_sh020_anim_v001",
})
err = client.UploadFile(ctx, "Version", version.ID, "sg_uploaded_movie",
    "/path/to/sc010_sh020_anim_v001.mov", "video/quicktime")
```

### Download

```go
err := client.DownloadFile(ctx, entityType, id, field, filePath)
err := client.Download(ctx, entityType, id, field, w)

// thumbnail: true for thumbnail, false for original image
err := client.DownloadImageFile(ctx, entityType, id, thumbnail, filePath)
err := client.DownloadImage(ctx, entityType, id, thumbnail, w)
```

---

## Unmarshalling into typed structs

`Entity.Decode` unmarshals the response JSON into any struct using standard `json` tags.
Fields are nested under `attributes`; deep-linked fields come back as flat keys using
the same dot-notation string passed to `sg.Fields`.

```go
type TaskAttributes struct {
    Content     string `json:"content"`
    Status      string `json:"sg_status_list"`
    ProjectName string `json:"project.Project.tank_name"`
    ShotCode    string `json:"entity.Shot.code"`
}

type TaskAssignees struct {
    Data []struct {
        ID   int    `json:"id"`
        Type string `json:"type"`
    } `json:"data"`
}

type Task struct {
    ID         int            `json:"id"`
    Type       string         `json:"type"`
    Attributes TaskAttributes `json:"attributes"`
    Relationships struct {
        TaskAssignees TaskAssignees `json:"task_assignees"`
    } `json:"relationships"`
}
```
