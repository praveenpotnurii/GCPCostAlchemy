package main

import (
    "context"
    "fmt"

    recommender "cloud.google.com/go/recommender/apiv1"
    recommenderpb "google.golang.org/genproto/googleapis/cloud/recommender/v1"
    "google.golang.org/api/iterator"
    "google.golang.org/api/option"
)

type Recommender struct {
    client           *recommender.Client
    recommenderID    string
    ignoreDescriptions []string
}

func NewRecommender(recommenderID string, ignoreDescriptions []string, credsJSON []byte) (*Recommender, error) {
    ctx := context.Background()
    client, err := recommender.NewClient(ctx, option.WithCredentialsJSON(credsJSON))
    if err != nil {
        return nil, fmt.Errorf("failed to create recommender client: %v", err)
    }

    return &Recommender{
        client:           client,
        recommenderID:    recommenderID,
        ignoreDescriptions: ignoreDescriptions,
    }, nil
}

func (r *Recommender) ListRecommendations(projectID string) ([]*recommenderpb.Recommendation, error) {
    ctx := context.Background()
    parent := fmt.Sprintf("projects/%s/locations/global/recommenders/%s", projectID, r.recommenderID)
    req := &recommenderpb.ListRecommendationsRequest{
        Parent: parent,
    }

    var recommendations []*recommenderpb.Recommendation
    it := r.client.ListRecommendations(ctx, req)
    for {
        recommendation, err := it.Next()
        if err == iterator.Done {
            break
        }
        if err != nil {
            return nil, fmt.Errorf("failed to iterate over recommendations: %v", err)
        }
        if !r.shouldIgnore(recommendation.Description) {
            recommendations = append(recommendations, recommendation)
        }
    }

    return recommendations, nil
}

func (r *Recommender) shouldIgnore(description string) bool {
    for _, ignoreDesc := range r.ignoreDescriptions {
        if description == ignoreDesc {
            return true
        }
    }
    return false
}

func (r *Recommender) Close() {
    r.client.Close()
}