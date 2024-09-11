package main

import (
    "context"
    "encoding/base64"
    "fmt"
    "html/template"
    "io/ioutil"
    "net/http"
    "os"

    "github.com/joho/godotenv"
    recommender "cloud.google.com/go/recommender/apiv1"
    recommenderpb "google.golang.org/genproto/googleapis/cloud/recommender/v1"
    "google.golang.org/api/iterator"
    "google.golang.org/api/option"
    "google.golang.org/api/cloudresourcemanager/v1"
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

    recommendations, err := listCostRecommendations(projectID)
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
    
    err := godotenv.Load()
    if err != nil {
        logger.Printf("Error loading .env file: %v", err)
        return nil, fmt.Errorf("error loading .env file: %v", err)
    }

    serviceAccountKeyPath := os.Getenv("SERVICE_ACCOUNT_KEY_PATH")
    if serviceAccountKeyPath == "" {
        logger.Println("SERVICE_ACCOUNT_KEY_PATH not set")
        return nil, fmt.Errorf("SERVICE_ACCOUNT_KEY_PATH not set")
    }

    encodedCreds, err := ioutil.ReadFile(serviceAccountKeyPath)
    if err != nil {
        logger.Printf("Failed to read service account key file: %v", err)
        return nil, fmt.Errorf("failed to read service account key file: %v", err)
    }

    creds, err := base64.StdEncoding.DecodeString(string(encodedCreds))
    if err != nil {
        logger.Printf("Failed to decode credentials: %v", err)
        return nil, fmt.Errorf("failed to decode credentials: %v", err)
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
            if project.LifecycleState == "ACTIVE" {
                projects = append(projects, project.ProjectId)
            }
        }

        pageToken = resp.NextPageToken
        if pageToken == "" {
            break
        }
    }

    logger.Printf("Found %d projects", len(projects))
    return projects, nil
}

func listCostRecommendations(projectID string) ([]string, error) {
    logger.Println("Starting listCostRecommendations function")
    
    err := godotenv.Load()
    if err != nil {
        logger.Printf("Error loading .env file: %v", err)
        return nil, fmt.Errorf("error loading .env file: %v", err)
    }

    serviceAccountKeyPath := os.Getenv("SERVICE_ACCOUNT_KEY_PATH")
    if serviceAccountKeyPath == "" {
        logger.Println("SERVICE_ACCOUNT_KEY_PATH not set")
        return nil, fmt.Errorf("SERVICE_ACCOUNT_KEY_PATH not set")
    }

    encodedCreds, err := ioutil.ReadFile(serviceAccountKeyPath)
    if err != nil {
        logger.Printf("Failed to read service account key file: %v", err)
        return nil, fmt.Errorf("failed to read service account key file: %v", err)
    }

    creds, err := base64.StdEncoding.DecodeString(string(encodedCreds))
    if err != nil {
        logger.Printf("Failed to decode credentials: %v", err)
        return nil, fmt.Errorf("failed to decode credentials: %v", err)
    }

    ctx := context.Background()
    client, err := recommender.NewClient(ctx, option.WithCredentialsJSON(creds))
    if err != nil {
        logger.Printf("Failed to create recommender client: %v", err)
        return nil, fmt.Errorf("failed to create recommender client: %v", err)
    }
    defer client.Close()

    var allRecommendations []string

    recommenderIDs := []string{
        "google.compute.instance.MachineTypeRecommender",
        "google.compute.disk.IdleResourceRecommender",
        "google.compute.commitment.UsageCommitmentRecommender",
    }

    for _, recommenderID := range recommenderIDs {
        parent := fmt.Sprintf("projects/%s/locations/global/recommenders/%s", projectID, recommenderID)
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
                logger.Printf("Error fetching recommendation: %v", err)
                continue
            }
            allRecommendations = append(allRecommendations, fmt.Sprintf("Recommender: %s, Description: %s, Priority: %s", recommenderID, recommendation.Description, recommendation.Priority))
        }
    }

    logger.Printf("Found %d recommendations in total for project %s", len(allRecommendations), projectID)
    return allRecommendations, nil
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