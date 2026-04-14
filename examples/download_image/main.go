package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	sg "github.com/rfletchr/ShotgunGo"
)

func main() {
	entityType := flag.String("entity", "", "Entity type (e.g. shots, assets, versions)")
	entityID := flag.Int("id", 0, "Entity ID")
	thumbnail := flag.Bool("thumbnail", false, "Download thumbnail instead of original image")
	output := flag.String("output", "", "Output file path (default: <entity_type>_<id>.jpg)")
	flag.Parse()

	if *entityType == "" {
		log.Fatal("-entity is required")
	}
	if *entityID == 0 {
		log.Fatal("-id is required")
	}

	sgURL := os.Getenv("SG_URL")
	scriptName := os.Getenv("SG_SCRIPT_NAME")
	scriptKey := os.Getenv("SG_SCRIPT_KEY")

	if sgURL == "" || scriptName == "" || scriptKey == "" {
		log.Fatal("SG_URL, SG_SCRIPT_NAME and SG_SCRIPT_KEY must be set")
	}

	filePath := *output
	if filePath == "" {
		filePath = fmt.Sprintf("%s_%d.jpg", *entityType, *entityID)
	}

	client := sg.NewClient(sgURL, scriptName, scriptKey)
	ctx := context.Background()

	log.Printf("downloading image for %s/%d (thumbnail=%v) -> %s", *entityType, *entityID, *thumbnail, filePath)

	err := client.DownloadImageFile(ctx, *entityType, *entityID, *thumbnail, filePath)
	if err != nil {
		log.Fatalf("failed to download image: %v", err)
	}

	fmt.Printf("saved %s\n", filePath)
}
