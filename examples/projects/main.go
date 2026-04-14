package main

import (
	"context"
	"fmt"
	"log"
	"os"

	sg "github.com/rfletchr/ShotgunGo"
)

type ProjectAttributes struct {
	Name     string `json:"name"`
	TankName string `json:"tank_name"`
	Status   string `json:"sg_status"`
	Archived bool   `json:"archived"`
}

type Project struct {
	ID         int               `json:"id"`
	Type       string            `json:"type"`
	Attributes ProjectAttributes `json:"attributes"`
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

	q := client.Find("projects",
		sg.Fields("name", "tank_name", "sg_status", "archived"),
		sg.And(
			sg.Filter("archived", "is", false),
			sg.Filter("is_demo", "is", false),
			sg.Filter("is_template", "is", false),
		),
	)

	for entity, err := range q.Iter(ctx) {
		if err != nil {
			log.Fatal(err)
		}
		var p Project
		if err := entity.Decode(&p); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%d\t%-20s\t%s\t%s\n", p.ID, p.Attributes.TankName, p.Attributes.Name, p.Attributes.Status)
	}
}
