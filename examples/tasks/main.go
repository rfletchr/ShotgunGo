package main

import (
	"context"
	"fmt"
	"log"
	"os"

	sg "github.com/rfletchr/ShotgunGo"
)

type TaskAttributes struct {
	Content      string `json:"content"`
	Status       string `json:"sg_status_list"`
	UpdatedAt    string `json:"updated_at"`
	ProjectName  string `json:"project.Project.tank_name"`
	ShotCode     string `json:"entity.Shot.code"`
	SequenceCode string `json:"entity.Shot.sg_sequence.Sequence.code"`
	EpisodeCode  string `json:"entity.Shot.sg_sequence.Sequence.episode.Episode.code"`
	AssetName    string `json:"entity.Asset.code"`
	AssetType    string `json:"entity.Asset.sg_asset_type"`
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
	Type          string            `json:"type"`
	Attributes    TaskAttributes    `json:"attributes"`
	Relationships TaskRelationships `json:"relationships"`
}

func main() {
	url := os.Getenv("SG_URL")
	scriptName := os.Getenv("SG_SCRIPT_NAME")
	scriptKey := os.Getenv("SG_SCRIPT_KEY")

	if url == "" || scriptName == "" || scriptKey == "" {
		log.Fatal("SG_URL, SG_SCRIPT_NAME and SG_SCRIPT_KEY must be set")
	}

	client := sg.NewClient(url, scriptName, scriptKey)
	ctx := context.Background()

	q := client.Find("tasks",
		sg.Fields(
			"content",
			"sg_status_list",
			"updated_at",
			"task_assignees",
			"project.Project.tank_name",
			"entity.Shot.code",
			"entity.Shot.sg_sequence.Sequence.code",
			"entity.Shot.sg_sequence.Sequence.episode.Episode.code",
			"entity.Asset.code",
			"entity.Asset.sg_asset_type",
		),
		sg.And(
			sg.Filter("content", "is_not", nil),
			sg.Filter("content", "is_not", ""),
			sg.Filter("project", "is_not", nil),
			sg.Filter("project.Project.tank_name", "is_not", nil),
			sg.Filter("project.Project.is_demo", "is", false),
			sg.Filter("project.Project.is_template", "is", false),
			sg.Filter("project.Project.archived", "is", false),
		),
	)

	count := 0
	for entity, err := range q.Iter(ctx) {
		if err != nil {
			log.Fatal(err)
		}
		var task Task
		if err := entity.Decode(&task); err != nil {
			log.Fatal(err)
		}
		count++
		fmt.Printf("%d\t%-30s\t%-10s\t%s\n",
			task.ID,
			task.Attributes.Content,
			task.Attributes.Status,
			task.Attributes.ProjectName,
		)
	}

	fmt.Printf("\n%d tasks\n", count)
}
