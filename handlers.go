package main

import (
    "path/filepath"
    "context"
    "encoding/base64"
    "fmt"
    "html/template"
    "io/ioutil"
    "net/http"
    "strings"
    "sync"
    "sort"
    recommender "cloud.google.com/go/recommender/apiv1"
    recommenderpb "google.golang.org/genproto/googleapis/cloud/recommender/v1"
    "google.golang.org/api/iterator"
    "google.golang.org/api/option"
    "google.golang.org/api/cloudresourcemanager/v1"
    compute "google.golang.org/api/compute/v1"
)

var serviceAccountKeyPath string

func landingHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodPost {
        file, _, err := r.FormFile("service_account_key")
        if err != nil {
            http.Error(w, "Failed to get uploaded file: "+err.Error(), http.StatusBadRequest)
            return
        }
        defer file.Close()

        // Read the file content
        content, err := ioutil.ReadAll(file)
        if err != nil {
            http.Error(w, "Failed to read uploaded file: "+err.Error(), http.StatusInternalServerError)
            return
        }

        // Encode the content to base64
        encodedContent := base64.StdEncoding.EncodeToString(content)

        // Save the encoded content to a file
        serviceAccountKeyPath = filepath.Join(".", "service_account_key.json")
        err = ioutil.WriteFile(serviceAccountKeyPath, []byte(encodedContent), 0644)
        if err != nil {
            http.Error(w, "Failed to save service account key: "+err.Error(), http.StatusInternalServerError)
            return
        }

        http.Redirect(w, r, "/home", http.StatusSeeOther)
        return
    }

    renderTemplate(w, "landing.html", nil)
}


func homeHandler(w http.ResponseWriter, r *http.Request) {
    if serviceAccountKeyPath == "" {
        http.Redirect(w, r, "/", http.StatusSeeOther)
        return
    }

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
    if serviceAccountKeyPath == "" {
        http.Redirect(w, r, "/", http.StatusSeeOther)
        return
    }
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

    recommenderSummaries, err := listCostRecommendations(projectID, creds)
    if err != nil {
        logger.Printf("Error listing cost recommendations: %v", err)
        http.Error(w, "Failed to list cost recommendations: "+err.Error(), http.StatusInternalServerError)
        return
    }

    // Convert map to slice for sorting
    summaries := make([]RecommenderSummary, 0, len(recommenderSummaries))
    for _, summary := range recommenderSummaries {
        summaries = append(summaries, summary)
    }

    // Sort summaries by total savings (descending order)
    sort.Slice(summaries, func(i, j int) bool {
        return summaries[i].TotalSavings > summaries[j].TotalSavings
    })

    totalSavings := calculateTotalSavings(summaries)

    data := struct {
        ProjectID            string
        RecommenderSummaries []RecommenderSummary
        TotalSavings         float64
    }{
        ProjectID:            projectID,
        RecommenderSummaries: summaries,
        TotalSavings:         totalSavings,
    }

    renderTemplate(w, "recommendations.html", data)
}

func loadCredentials() ([]byte, error) {
    if serviceAccountKeyPath == "" {
        return nil, fmt.Errorf("SERVICE_ACCOUNT_KEY not set")
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

func calculateTotalSavings(summaries []RecommenderSummary) float64 {
    total := 0.0
    for _, summary := range summaries {
        total += summary.TotalSavings
    }
    return total
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

func listCostRecommendations(projectID string, creds []byte) (map[string]RecommenderSummary, error) {
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

    recommenderSummaries := make(map[string]RecommenderSummary)
    var mutex sync.Mutex
    var wg sync.WaitGroup

    recommenderIDs := []string{
        "google.compute.instance.MachineTypeRecommender",
        "google.compute.disk.IdleResourceRecommender",
        "google.compute.commitment.UsageCommitmentRecommender",
        "google.compute.instance.IdleResourceRecommender",
    }

    semaphore := make(chan struct{}, 20) // Limit concurrent goroutines

    for _, location := range locations {
        if !strings.HasPrefix(location, "us-") {
            continue
        }
        for _, recommenderID := range recommenderIDs {
            wg.Add(1)
            semaphore <- struct{}{} // Acquire semaphore
            go func(loc, recID string) {
                defer wg.Done()
                defer func() { <-semaphore }() // Release semaphore

                parent := fmt.Sprintf("projects/%s/locations/%s/recommenders/%s", projectID, loc, recID)
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
                        logger.Printf("Error fetching recommendation for location %s and recommender %s: %v", loc, recID, err)
                        continue
                    }
                    
                    costSavings := extractCostSavings(recommendation)
                    
                    mutex.Lock()
                    summary, ok := recommenderSummaries[recID]
                    if !ok {
                        summary = RecommenderSummary{
                            Name:                recID,
                            TotalSavings:        0,
                            RecommendationCount: 0,
                        }
                    }
                    summary.TotalSavings += costSavings
                    summary.RecommendationCount++
                    recommenderSummaries[recID] = summary
                    mutex.Unlock()
                }
            }(location, recommenderID)
        }
    }

    wg.Wait()

    logger.Printf("Found recommendations for %d recommenders in project %s", len(recommenderSummaries), projectID)
    return recommenderSummaries, nil
}


func extractCostSavings(recommendation *recommenderpb.Recommendation) float64 {
    if recommendation.PrimaryImpact == nil {
        return 0
    }

    switch impact := recommendation.PrimaryImpact.GetCategory(); impact {
    case recommenderpb.Impact_COST:
        costImpact := recommendation.PrimaryImpact.GetCostProjection()
        if costImpact != nil && costImpact.Cost != nil {
            // The Cost field is a *money.Money type
            costAmount := costImpact.Cost
            if costAmount.CurrencyCode == "USD" {
                // Convert int64 units to float64
                dollars := float64(costAmount.Units)
                // Convert nanos to a fraction of a dollar
                nanos := float64(costAmount.Nanos) / 1e9
                // The cost in the recommendation is typically negative (a saving)
                // so we return the negation to represent it as a positive saving
                return -(dollars + nanos)
            }
        }
    }

    return 0
}

type RecommenderSummary struct {
    Name                string
    TotalSavings        float64
    RecommendationCount int
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