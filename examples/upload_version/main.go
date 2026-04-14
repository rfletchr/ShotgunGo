package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	sg "github.com/rfletchr/ShotgunGo"
)

func main() {
	projectID := flag.Int("project", 0, "Shotgun project ID")
	moviePath := flag.String("movie", "", "Path to the .mov file to upload")
	flag.Parse()

	if *projectID == 0 {
		log.Fatal("-project is required")
	}
	if *moviePath == "" {
		log.Fatal("-movie is required")
	}

	sgURL := os.Getenv("SG_URL")
	scriptName := os.Getenv("SG_SCRIPT_NAME")
	scriptKey := os.Getenv("SG_SCRIPT_KEY")

	if sgURL == "" || scriptName == "" || scriptKey == "" {
		log.Fatal("SG_URL, SG_SCRIPT_NAME and SG_SCRIPT_KEY must be set")
	}

	client := sg.NewClient(sgURL, scriptName, scriptKey)
	ctx := context.Background()

	filename := filepath.Base(*moviePath)

	// Create the Version record.
	log.Printf("creating version %q on project %d", filename, *projectID)
	version, err := client.Create(ctx, "versions", map[string]any{
		"project":          map[string]any{"type": "Project", "id": *projectID},
		"code":             filename,
		"sg_path_to_movie": *moviePath,
	})
	if err != nil {
		log.Fatalf("failed to create version: %v", err)
	}
	log.Printf("created version id=%d", version.ID)

	// Upload the movie file to the version.
	log.Printf("uploading %s", *moviePath)
	err = client.UploadFile(ctx, "versions", version.ID, "sg_uploaded_movie", *moviePath, "video/quicktime")
	if err != nil {
		log.Fatalf("failed to upload movie: %v", err)
	}

	fmt.Printf("done — version id=%d\n", version.ID)
}
