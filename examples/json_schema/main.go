package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gollem/llm/openai"
)

// createUserProfileSchema creates a schema for extracting user profile information
func createUserProfileSchema() *gollem.ResponseSchema {
	return &gollem.ResponseSchema{
		Name:        "UserProfile",
		Description: "Structured user profile information",
		Schema: &gollem.Parameter{
			Type: gollem.TypeObject,
			Properties: map[string]*gollem.Parameter{
				"name": {
					Type:        gollem.TypeString,
					Description: "Full name of the user",
				},
				"age": {
					Type:        gollem.TypeInteger,
					Description: "Age in years",
					Minimum:     Float64Ptr(0),
					Maximum:     Float64Ptr(150),
				},
				"email": {
					Type:        gollem.TypeString,
					Description: "Email address",
				},
				"interests": {
					Type: gollem.TypeArray,
					Items: &gollem.Parameter{
						Type: gollem.TypeString,
					},
					Description: "List of interests or hobbies",
				},
				"location": {
					Type: gollem.TypeObject,
					Properties: map[string]*gollem.Parameter{
						"city": {
							Type:        gollem.TypeString,
							Description: "City name",
						},
						"country": {
							Type:        gollem.TypeString,
							Description: "Country name",
						},
					},
					Description: "User's location",
				},
			},
			Required: []string{"name", "email"},
		},
	}
}

// Float64Ptr is a helper function to create a pointer to a float64
func Float64Ptr(v float64) *float64 {
	return &v
}

func runOpenAIExample() error {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY is not set")
	}

	ctx := context.Background()
	client, err := openai.New(ctx, apiKey)
	if err != nil {
		return fmt.Errorf("failed to create OpenAI client: %w", err)
	}

	schema := createUserProfileSchema()

	session, err := client.NewSession(ctx,
		gollem.WithSessionContentType(gollem.ContentTypeJSON),
		gollem.WithSessionResponseSchema(schema),
	)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	resp, err := session.GenerateContent(ctx,
		gollem.Text("Extract user information: Sarah Johnson is 28 years old, email: sarah.j@example.com, lives in Seattle, USA, and enjoys hiking, photography, and cooking."))
	if err != nil {
		return fmt.Errorf("failed to generate content: %w", err)
	}

	// Parse and pretty-print the JSON response
	var profile map[string]any
	jsonStr := resp.Texts[0]
	if err := json.Unmarshal([]byte(jsonStr), &profile); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	prettyJSON, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	fmt.Println("=== OpenAI Result ===")
	fmt.Println(string(prettyJSON))
	fmt.Println()

	return nil
}

func runClaudeExample() error {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY is not set")
	}

	ctx := context.Background()
	client, err := claude.New(ctx, apiKey)
	if err != nil {
		return fmt.Errorf("failed to create Claude client: %w", err)
	}

	schema := createUserProfileSchema()

	session, err := client.NewSession(ctx,
		gollem.WithSessionContentType(gollem.ContentTypeJSON),
		gollem.WithSessionResponseSchema(schema),
	)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	resp, err := session.GenerateContent(ctx,
		gollem.Text("Extract user information: Michael Chen is 35 years old, email: m.chen@tech.com, lives in San Francisco, USA, and enjoys programming, gaming, and traveling."))
	if err != nil {
		return fmt.Errorf("failed to generate content: %w", err)
	}

	// Parse and pretty-print the JSON response
	var profile map[string]any
	jsonStr := resp.Texts[0]
	if err := json.Unmarshal([]byte(jsonStr), &profile); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	prettyJSON, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	fmt.Println("=== Claude Result ===")
	fmt.Println(string(prettyJSON))
	fmt.Println()

	return nil
}

func runGeminiExample() error {
	projectID := os.Getenv("GEMINI_PROJECT_ID")
	location := os.Getenv("GEMINI_LOCATION")
	if projectID == "" || location == "" {
		return fmt.Errorf("GEMINI_PROJECT_ID or GEMINI_LOCATION is not set")
	}

	ctx := context.Background()
	client, err := gemini.New(ctx, projectID, location)
	if err != nil {
		return fmt.Errorf("failed to create Gemini client: %w", err)
	}

	schema := createUserProfileSchema()

	session, err := client.NewSession(ctx,
		gollem.WithSessionContentType(gollem.ContentTypeJSON),
		gollem.WithSessionResponseSchema(schema),
	)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	resp, err := session.GenerateContent(ctx,
		gollem.Text("Extract user information: Emily Davis is 31 years old, email: emily.d@design.com, lives in Portland, USA, and enjoys painting, yoga, and reading."))
	if err != nil {
		return fmt.Errorf("failed to generate content: %w", err)
	}

	// Parse and pretty-print the JSON response
	var profile map[string]any
	jsonStr := resp.Texts[0]
	if err := json.Unmarshal([]byte(jsonStr), &profile); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	prettyJSON, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	fmt.Println("=== Gemini Result ===")
	fmt.Println(string(prettyJSON))
	fmt.Println()

	return nil
}

func main() {
	fmt.Println("JSON Schema Example")
	fmt.Println("===================")
	fmt.Println()

	// Try each provider
	if err := runOpenAIExample(); err != nil {
		log.Printf("OpenAI example skipped: %v", err)
	}

	if err := runClaudeExample(); err != nil {
		log.Printf("Claude example skipped: %v", err)
	}

	if err := runGeminiExample(); err != nil {
		log.Printf("Gemini example skipped: %v", err)
	}
}
