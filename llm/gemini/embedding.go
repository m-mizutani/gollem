package gemini

import (
	"context"
	"fmt"

	aiplatform "cloud.google.com/go/aiplatform/apiv1"
	"cloud.google.com/go/aiplatform/apiv1/aiplatformpb"
	"github.com/m-mizutani/goerr/v2"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/types/known/structpb"
)

// GenerateEmbedding generates embeddings for the given input text.
func (x *Client) GenerateEmbedding(ctx context.Context, dimension int, input []string) ([][]float64, error) {
	apiEndpoint := fmt.Sprintf("%s-aiplatform.googleapis.com:443", x.location)

	client, err := aiplatform.NewPredictionClient(ctx, option.WithEndpoint(apiEndpoint))
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create aiplatform client")
	}
	defer client.Close()

	endpoint := fmt.Sprintf("projects/%s/locations/%s/publishers/google/models/%s", x.projectID, x.location, x.embeddingModel)
	instances := make([]*structpb.Value, len(input))

	for i, v := range input {
		instances[i] = structpb.NewStructValue(&structpb.Struct{
			Fields: map[string]*structpb.Value{
				"content":   structpb.NewStringValue(v),
				"task_type": structpb.NewStringValue("QUESTION_ANSWERING"),
			},
		})
	}

	params := structpb.NewStructValue(&structpb.Struct{
		Fields: map[string]*structpb.Value{
			"outputDimensionality": structpb.NewNumberValue(float64(dimension)),
		},
	})

	req := &aiplatformpb.PredictRequest{
		Endpoint:   endpoint,
		Instances:  instances,
		Parameters: params,
	}
	resp, err := client.Predict(ctx, req)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to predict",
			goerr.V("endpoint", endpoint),
			goerr.V("dimensionality", dimension),
		)
	}

	if len(resp.Predictions) == 0 {
		return nil, goerr.New("no predictions returned")
	}

	embeddings := make([][]float64, len(resp.Predictions))
	for i, prediction := range resp.Predictions {
		values := prediction.GetStructValue().Fields["embeddings"].GetStructValue().Fields["values"].GetListValue().Values
		embedding := make([]float64, len(values))
		for j, value := range values {
			embedding[j] = float64(value.GetNumberValue())
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}
