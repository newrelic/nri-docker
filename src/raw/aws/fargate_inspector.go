package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/docker/docker/api/types"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/persist"
)

const fargateTaskMetadataCacheKey = "task-metadata-response"

// FargateInspector is responsible for listing containers and inspecting containers in Fargate.
// Both operations use the same data source and thus access it through a caching layer to avoid extra
// computations.
type FargateInspector struct {
	baseURL        *url.URL
	http           *http.Client
	containerStore persist.Storer
}

// NewFargateInspector creates a new FargateInspector
func NewFargateInspector(baseURL *url.URL) (*FargateInspector, error) {
	containerStore := persist.NewInMemoryStore()

	return &FargateInspector{
		baseURL:        baseURL,
		http:           fargateHTTPClient,
		containerStore: containerStore,
	}, nil
}

// ContainerList lists containers that the current Fargate container can see (only the container in the same
// task). It completely ignores any listing option for the moment.
func (i *FargateInspector) ContainerList(_ context.Context, _ types.ContainerListOptions) ([]types.Container, error) {
	var taskResponse TaskResponse
	err := i.taskResponseFromCacheOrNew(&taskResponse)
	if err != nil {
		return nil, err
	}

	containers := make([]types.Container, len(taskResponse.Containers))
	for index, container := range taskResponse.Containers {
		converted := containerResponseToDocker(container)
		containers[index] = converted
	}
	return containers, nil
}

// taskResponseFromCacheOrNew wraps the access to Fargate task metadata with a caching layer.
func (i *FargateInspector) taskResponseFromCacheOrNew(response *TaskResponse) error {
	defer func() {
		if err := i.containerStore.Save(); err != nil {
			log.Warn("error persisting Fargate task metadata: %s", err)
		}
	}()

	var err error
	_, err = i.containerStore.Get(fargateTaskMetadataCacheKey, response)
	if err == persist.ErrNotFound {
		err = i.fetchTaskResponse(response)
	}
	if err != nil {
		return fmt.Errorf("cannot fetch Fargate task metadata response: %s", err)
	}
	i.containerStore.Set(fargateTaskMetadataCacheKey, response)
	return nil
}

func (i *FargateInspector) fetchTaskResponse(taskResponse *TaskResponse) error {
	endpoint := TaskMetadataEndpoint(i.baseURL.String())

	response, err := metadataResponse(i.http, endpoint)
	if err != nil {
		return fmt.Errorf(
			"error when sending request to ECS task metadata endpoint (%s): %v",
			endpoint,
			err,
		)
	}
	log.Debug("task metadata response from endpoint %s: %s", endpoint, string(response))

	err = json.Unmarshal(response, taskResponse)
	if err != nil {
		return fmt.Errorf("error unmarshalling ECS task: %v", err)
	}
	return nil
}

// ContainerInspect returns metadata about a container given its container ID.
func (i *FargateInspector) ContainerInspect(_ context.Context, containerID string) (types.ContainerJSON, error) {
	var taskResponse TaskResponse
	err := i.taskResponseFromCacheOrNew(&taskResponse)
	if err != nil {
		return types.ContainerJSON{}, err
	}

	for _, container := range taskResponse.Containers {
		if container.ID == containerID {
			containerJSON := types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{ID: containerID},
			}
			return containerJSON, nil
		}
	}
	return types.ContainerJSON{}, errors.New("container not found")
}

func containerResponseToDocker(container ContainerResponse) types.Container {
	c := types.Container{
		ID:      container.ID,
		Names:   []string{container.Name},
		Image:   container.Image,
		ImageID: container.ImageID,
		Labels:  processFargateLabels(container.Labels),
		Status:  container.KnownStatus,
	}
	if created := container.CreatedAt; created != nil {
		c.Created = created.Unix()
	}
	return c
}

func processFargateLabels(labels map[string]string) map[string]string {
	for label, value := range labels {
		switch label {
		case "com.amazonaws.ecs.cluster":
			if name := clusterNameFromARN(value); name != "" {
				// The cluster name label has to be processed because in ECS with EC2 launch type it's only the cluster name.
				// Meanwhile, in ECS with Fargate launch type the same label has the full cluster ARN as value.
				labels[label] = name
				// Preserve the original arn in a synthetic label
				labels["com.newrelic.nri-docker.cluster-arn"] = value
			}

		case "com.amazonaws.ecs.task-arn":
			// Obtain aws region from task arn
			if region := regionFromTaskARN(value); region != "" {
				labels["com.newrelic.nri-docker.aws-region"] = region
			} else {
				// Log an error if task-arn is not an arn
				log.Error("could not process task arn: %s", label)
			}
		}
	}

	// Add label signaling fargate launch type
	labels["com.newrelic.nri-docker.launch-type"] = "fargate"

	return labels
}

// clusterNameFromARN extracts the cluster name from an ECS cluster ARN.
func clusterNameFromARN(ecsClusterARN string) string {
	a, err := arn.Parse(ecsClusterARN)
	if err != nil {
		return ""
	}

	resourceParts := strings.Split(a.Resource, "/")
	if len(resourceParts) < 2 || resourceParts[0] != "cluster" {
		return ""
	}

	return resourceParts[1]
}

// regionFromTaskARN returns the aws region from the task ARN.
// Example of task ARN: arn:aws:ecs:us-west-2:xxxxxxxx:task/ecs-local-cluster/37e873f6-37b4-42a7-af47-eac7275c6152
func regionFromTaskARN(taskARN string) string {
	a, err := arn.Parse(taskARN)
	if err != nil {
		return ""
	}

	return a.Region
}
