# ShotgunGo
A Shotgun REST API client for Go. This API is based on https://developers.shotgridsoftware.com/rest-api/ and was written with the assistance of Claude Code.

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
    Content string `json:"content"`
    Status  string `json:"sg_status_list"`
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

    // Build a query — this does not hit the network.
    q := client.Find("tasks",
        sg.Fields(
            "content",
            "sg_status_list",
            "project.Project.tank_name",
            "entity.Shot.code",
        ),
        sg.And(
            sg.Filter("project.Project.archived", sg.Is, false),
            sg.Filter("project.Project.is_demo", sg.Is, false),
            sg.Or(
                sg.Filter("sg_status_list", sg.Is, "ip"),
                sg.Filter("sg_status_list", sg.Is, "rdy"),
            ),
        ),
    )

    // Iterate over all results, fetching pages on demand.
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

The REST API uses **plural snake_case** for entity types, which differs from the Python API's singular PascalCase:

| Python API    | REST API       |
|---------------|----------------|
| `Task`        | `tasks`        |
| `Project`     | `projects`     |
| `HumanUser`   | `human_users`  |
| `CustomEntity01` | `custom_entity_01s` |

### Client

```go
client := sg.NewClient(baseURL, scriptName, scriptKey)
```

Authenticates using Shotgun script credentials. Tokens are refreshed automatically.

### Find

```go
q := client.Find(entityType, options...)
```

Returns an immutable `*Query`. No network call is made at this point.

**Options:**

| Function                            | Description                                                  |
|-------------------------------------|--------------------------------------------------------------|
| `sg.Fields("f1", "f2", ...)`        | Fields to return, including deep links (`entity.Shot.code`)  |
| `sg.Filter(field, relation, value)` | Single filter condition; `relation` is a `sg.FilterRelation` constant |
| `sg.And(conditions...)`             | All conditions must match                                    |
| `sg.Or(conditions...)`              | Any condition must match                                     |
| `sg.PageSize(n)`                    | Override the default page size (500)                         |

### Query methods

```go
entity, err := q.One(ctx)             // first result, or nil
entities, err := q.All(ctx)           // all results, walking pages
page, err := q.Page(ctx, 1)           // fetch a specific page (1-indexed)

for entity, err := range q.Iter(ctx) { // range-over iterator, pages on demand
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

### NewEntityRef

```go
ref := sg.NewEntityRef(entityType, id)
```

Returns a relationship reference (`map[string]any{"type": entityType, "id": id}`) for use as a field value in `Create`, `Update`, and `Batch` requests.

> **Note:** `entityType` must be the singular PascalCase name used by the REST API for relationship objects (e.g. `"Project"`, `"Task"`, `"HumanUser"`) — **not** the plural snake_case name used for endpoint paths (e.g. `"projects"`, `"tasks"`).

### Create

```go
entity, err := client.Create(ctx, entityType, fields)
```

Creates a new record. `fields` is a flat `map[string]any` of field names to values.
Relationship fields are set using `NewEntityRef`:

```go
entity, err := client.Create(ctx, "tasks", map[string]any{
    "content":        "Animation",
    "sg_status_list": "rdy",
    "project":        sg.NewEntityRef("Project", 123),
})
```

Returns the created `Entity`. Call `Decode` on it to get a typed struct.

### Update

```go
entity, err := client.Update(ctx, entityType, id, fields)
```

Updates an existing record. Only the fields present in the map are changed — unspecified fields are left untouched.

```go
entity, err := client.Update(ctx, "tasks", 456, map[string]any{
    "sg_status_list": "ip",
})
```

Returns the updated `Entity`.

### Delete

```go
err := client.Delete(ctx, entityType, id)
```

Deletes a record. Returns `nil` on success, an error otherwise.

```go
err := client.Delete(ctx, "tasks", 456)
```

### Batch

```go
results, err := client.Batch(ctx, requests...)
```

Executes multiple create, update, and delete operations in a single request.
Results are returned in the same order as the requests. Delete operations produce a zero-valued `Entity` in the result slice.

```go
results, err := client.Batch(ctx,
    sg.NewCreateRequest("tasks", map[string]any{
        "content": "Animation",
        "project": sg.NewEntityRef("Project", 123),
    }),
    sg.NewUpdateRequest("tasks", 456, map[string]any{
        "sg_status_list": "ip",
    }),
    sg.NewDeleteRequest("tasks", 789),
)
```

### Upload

```go
// Upload from a file path — multipart is used automatically for files > 5MB.
err := client.UploadFile(ctx, entityType, id, field, filePath, contentType)

// Upload from any io.Reader when you already have the data in memory or a stream.
err := client.Upload(ctx, entityType, id, field, filename, contentType, r, size)

// Upload as a linked Attachment rather than into a specific field.
err := client.UploadFile(ctx, "versions", 123, "", "/path/to/notes.pdf", "application/pdf")
```

**Creating a Version and uploading a movie:**

```go
version, err := client.Create(ctx, "versions", map[string]any{
    "project":          sg.NewEntityRef("Project", 123),
    "code":             "sc010_sh020_anim_v001.mov",
    "sg_path_to_movie": "/path/to/sc010_sh020_anim_v001.mov",
})
if err != nil {
    log.Fatal(err)
}

err = client.UploadFile(ctx, "versions", version.ID, "sg_uploaded_movie",
    "/path/to/sc010_sh020_anim_v001.mov", "video/quicktime")
```

### Download

```go
// Download a file field to disk.
err := client.DownloadFile(ctx, entityType, id, field, filePath)

// Stream a file field to any io.Writer.
err := client.Download(ctx, entityType, id, field, w)

// Download the image/thumbnail field to disk.
err := client.DownloadImageFile(ctx, entityType, id, thumbnail, filePath)

// Stream the image/thumbnail field to any io.Writer.
err := client.DownloadImage(ctx, entityType, id, thumbnail, w)
```

```go
// Download a version's uploaded movie.
err := client.DownloadFile(ctx, "versions", 123, "sg_uploaded_movie", "./cut.mov")

// Download an asset's thumbnail.
err := client.DownloadImageFile(ctx, "assets", 456, true, "./thumb.jpg")

// Download the original image.
err := client.DownloadImageFile(ctx, "assets", 456, false, "./image.jpg")
```

## Unmarshalling into typed structs

`Entity.Decode` unmarshals the original response JSON into any struct using standard `json` tags.
Define a struct that mirrors the entity envelope:

- `id` and `type` sit at the top level
- requested fields are nested under `attributes` — deep-linked fields (e.g. `project.Project.tank_name`) come back as flat keys using the same dot-notation string
- related entity references are nested under `relationships`, each with a `data` array

```go
type EntityRef struct {
    ID   int    `json:"id"`
    Type string `json:"type"`
}

type TaskAttributes struct {
    Content      string `json:"content"`
    Status       string `json:"sg_status_list"`
    UpdatedAt    string `json:"updated_at"`
    // Deep-linked fields use the same dot-notation passed to sg.Fields(...)
    ProjectName  string `json:"project.Project.tank_name"`
    ShotCode     string `json:"entity.Shot.code"`
    SequenceCode string `json:"entity.Shot.sg_sequence.Sequence.code"`
}

type TaskAssignees struct {
    Data []EntityRef `json:"data"`
}

type TaskRelationships struct {
    TaskAssignees TaskAssignees `json:"task_assignees"`
}

type Task struct {
    ID            int               `json:"id"`
    Type          string            `json:"type"`
    Attributes    TaskAttributes    `json:"attributes"`
    Relationships TaskRelationships `json:"relationships"`
}
```

Then decode inline:

```go
for entity, err := range q.Iter(ctx) {
    if err != nil {
        log.Fatal(err)
    }
    var task Task
    if err := entity.Decode(&task); err != nil {
        log.Fatal(err)
    }
    fmt.Println(task.ID, task.Attributes.Content, task.Attributes.Status)
    for _, assignee := range task.Relationships.TaskAssignees.Data {
        fmt.Println("  assignee id:", assignee.ID)
    }
}
```
