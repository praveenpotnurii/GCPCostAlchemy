package main

import (
    "context"
    "encoding/base64"
    "fmt"
    "html/template"
    "io/ioutil"
    "net/http"
    "os"
    "strings"

    "github.com/joho/godotenv"
    recommender "cloud.google.com/go/recommender/apiv1"
    recommenderpb "google.golang.org/genproto/googleapis/cloud/recommender/v1"
    "google.golang.org/api/iterator"
    "google.golang.org/api/option"
    "google.golang.org/api/cloudresourcemanager/v1"
    compute "google.golang.org/api/compute/v1"
)

func homeHandler(w http.ResponseWriter, r *http.Request) {
    logger.Println("Received request for home page")
    projects, err := listProjects()
    if err != nil {
        logger.Printf("Error listing projects: %v", err)
        http.Error(w, "Failed to list projects: "+err.Error(), http.StatusInternalServerError)
        return
    }

    renderTemplate(w, "home.html", projects)
}

func costRecommendationsHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    projectID := r.FormValue("project")
    if projectID == "" {
        http.Error(w, "Project ID is required", http.StatusBadRequest)
        return
    }

    creds, err := loadCredentials()
    if err != nil {
        logger.Printf("Error loading credentials: %v", err)
        http.Error(w, "Failed to load credentials: "+err.Error(), http.StatusInternalServerError)
        return
    }

    recommendations, err := listCostRecommendations(projectID, creds)
    if err != nil {
        logger.Printf("Error listing cost recommendations: %v", err)
        http.Error(w, "Failed to list cost recommendations: "+err.Error(), http.StatusInternalServerError)
        return
    }

    data := struct {
        ProjectID       string
        Recommendations []string
    }{
        ProjectID:       projectID,
        Recommendations: recommendations,
    }

    renderTemplate(w, "recommendations.html", data)
}

func listProjects() ([]string, error) {
    logger.Println("Starting listProjects function")
    
    creds, err := loadCredentials()
    if err != nil {
        return nil, fmt.Errorf("failed to load credentials: %v", err)
    }

    ctx := context.Background()
    rmService, err := cloudresourcemanager.NewService(ctx, option.WithCredentialsJSON(creds))
    if err != nil {
        logger.Printf("Failed to create Resource Manager service: %v", err)
        return nil, fmt.Errorf("failed to create Resource Manager service: %v", err)
    }

    var projects []string
    pageToken := ""
    for {
        resp, err := rmService.Projects.List().PageToken(pageToken).Do()
        if err != nil {
            logger.Printf("Failed to list projects: %v", err)
            return nil, fmt.Errorf("failed to list projects: %v", err)
        }

        for _, project := range resp.Projects {
            projects = append(projects, project.ProjectId)
        }

        pageToken = resp.NextPageToken
        if pageToken == "" {
            break
        }
    }

    logger.Printf("Found %d projects", len(projects))
    return projects, nil
}

func listCostRecommendations(projectID string, creds []byte) ([]string, error) {
    ctx := context.Background()
    client, err := recommender.NewClient(ctx, option.WithCredentialsJSON(creds))
    if err != nil {
        return nil, fmt.Errorf("failed to create recommender client: %v", err)
    }
    defer client.Close()

    locations, err := listRegions(projectID, creds)
    if err != nil {
        return nil, fmt.Errorf("failed to list regions and zones: %v", err)
    }

    var allRecommendations []string

    recommenderIDs := []string{
        "google.compute.instance.MachineTypeRecommender",
        "google.compute.disk.IdleResourceRecommender",
        "google.compute.commitment.UsageCommitmentRecommender",
        "google.compute.instance.IdleResourceRecommender",
    }

    for _, location := range locations {
        if !strings.HasPrefix(location, "us-") {
            continue
        }
        for _, recommenderID := range recommenderIDs {
            parent := fmt.Sprintf("projects/%s/locations/%s/recommenders/%s", projectID, location, recommenderID)
            req := &recommenderpb.ListRecommendationsRequest{
                Parent: parent,
            }

            it := client.ListRecommendations(ctx, req)
            for {
                recommendation, err := it.Next()
                if err == iterator.Done {
                    break
                }
                if err != nil {
                    logger.Printf("Error fetching recommendation for location %s and recommender %s: %v", location, recommenderID, err)
                    continue
                }
                allRecommendations = append(allRecommendations, fmt.Sprintf("Location: %s, Recommender: %s, Description: %s, Priority: %s", location, recommenderID, recommendation.Description, recommendation.Priority))
            }
        }
    }

    logger.Printf("Found %d recommendations in total for project %s", len(allRecommendations), projectID)
    return allRecommendations, nil
}

func listRegions(projectID string, creds []byte) ([]string, error) {
    ctx := context.Background()
    computeService, err := compute.NewService(ctx, option.WithCredentialsJSON(creds))
    if err != nil {
        return nil, fmt.Errorf("failed to create compute service: %v", err)
    }

    var locations []string

    // List all zones
    zonesListCall := computeService.Zones.List(projectID)
    err = zonesListCall.Pages(ctx, func(page *compute.ZoneList) error {
        for _, zone := range page.Items {
            locations = append(locations, zone.Name)
        }
        return nil
    })
    if err != nil {
        return nil, fmt.Errorf("failed to list zones: %v", err)
    }

    // Extract unique regions from zones
    regionSet := make(map[string]bool)
    for _, zone := range locations {
        parts := strings.Split(zone, "-")
        if len(parts) > 2 {
            region := strings.Join(parts[:len(parts)-1], "-")
            regionSet[region] = true
        }
    }

    // Add regions to locations
    for region := range regionSet {
        locations = append(locations, region)
    }

    // Add global location
    locations = append(locations, "global")

    return locations, nil
}

func loadCredentials() ([]byte, error) {
    err := godotenv.Load()
    if err != nil {
        return nil, fmt.Errorf("error loading .env file: %v", err)
    }

    serviceAccountKeyPath := os.Getenv("SERVICE_ACCOUNT_KEY_PATH")
    if serviceAccountKeyPath == "" {
        return nil, fmt.Errorf("SERVICE_ACCOUNT_KEY_PATH not set")
    }

    encodedCreds, err := ioutil.ReadFile(serviceAccountKeyPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read service account key file: %v", err)
    }

    creds, err := base64.StdEncoding.DecodeString(string(encodedCreds))
    if err != nil {
        return nil, fmt.Errorf("failed to decode credentials: %v", err)
    }

    return creds, nil
}

func renderTemplate(w http.ResponseWriter, tmplFile string, data interface{}) {
    tmpl, err := template.ParseFiles(tmplFile)
    if err != nil {
        logger.Printf("Error parsing template: %v", err)
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }

    err = tmpl.Execute(w, data)
    if err != nil {
        logger.Printf("Error executing template: %v", err)
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }
}