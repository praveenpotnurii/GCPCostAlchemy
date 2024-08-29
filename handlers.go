package main

import (
    "net/http"
    "context"
    "fmt"
    "os"
    "encoding/base64"
    "io/ioutil"
    "google.golang.org/api/compute/v1"
    "google.golang.org/api/option"
    "github.com/joho/godotenv"

)


func homeHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "<h1>GCP VM Instances</h1><p><a href='/list-vms'>List all VM Instances</a></p>")
}

func listVMsHandler(w http.ResponseWriter, r *http.Request) {
    vms, err := listVMInstances()
    if err != nil {
        http.Error(w, "Failed to list VM instances: "+err.Error(), http.StatusInternalServerError)
        return
    }

    renderInstances(w, vms)
}


func listVMInstances() ([]string, error) {
	// Load the .env file
	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("error loading .env file: %v", err)
	}

	// Get the path to the Base64 encoded service account key
	serviceAccountKeyPath := os.Getenv("SERVICE_ACCOUNT_KEY_PATH")
	if serviceAccountKeyPath == "" {
		return nil, fmt.Errorf("SERVICE_ACCOUNT_KEY_PATH not set")
	}

	// Read the Base64 encoded credentials from the file
	encodedCreds, err := ioutil.ReadFile(serviceAccountKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read service account key file: %v", err)
	}

	// Decode the Base64 string
	creds, err := base64.StdEncoding.DecodeString(string(encodedCreds))
	if err != nil {
		return nil, fmt.Errorf("failed to decode credentials: %v", err)
	}

	// Use the decoded credentials to create a compute service
	ctx := context.Background()
	computeService, err := compute.NewService(ctx, option.WithCredentialsJSON(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to create compute service: %v", err)
	}

	// Replace "your-project-id" with your actual GCP project ID
	projectID := "ops-inf1"

	// List VM instances across all zones
	instancesService := compute.NewInstancesService(computeService)
	instanceList := instancesService.AggregatedList(projectID)

	var instances []string
	if err := instanceList.Pages(ctx, func(page *compute.InstanceAggregatedList) error {
		for _, instancesScopedList := range page.Items {
			for _, instance := range instancesScopedList.Instances {
				instances = append(instances, fmt.Sprintf("Name: %s, Zone: %s", instance.Name, instance.Zone))
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return instances, nil
}


func renderInstances(w http.ResponseWriter, instances []string) {
    fmt.Fprintf(w, "<h1>VM Instances</h1>")
    if len(instances) == 0 {
        fmt.Fprintf(w, "<p>No VM instances found.</p>")
        return
    }
    fmt.Fprintf(w, "<ul>")
    for _, instance := range instances {
        fmt.Fprintf(w, "<li>%s</li>", instance)
    }
    fmt.Fprintf(w, "</ul>")
}
