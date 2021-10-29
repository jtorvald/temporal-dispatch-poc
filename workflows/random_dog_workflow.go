package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jtorvald/temporal-dispatch-poc/schema"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
	"net/http"
	"path"
	"time"
)

func init(){
	if availableWorkflows == nil {
		availableWorkflows = make(map[string]interface{})
	}
	availableWorkflows["random_dog"] = RandomDogWorkflow
}

// RandomDogWorkflow is a Hello World workflow definition.
func RandomDogWorkflow(ctx workflow.Context, params map[string]interface{}) (*schema.WorkflowInstanceUpdate, error) {
/*
	params:
	  {
       "key": "value",
       "incident_id": 32,
       "incident_name": "dispatch-singlesproject-default-32",
       "instance_id": 25
     }
 */
	result :=  &schema.WorkflowInstanceUpdate{
		CreatedAt:    workflow.Now(ctx).UTC().Format(format),
		Artifacts: []*schema.DocumentCreate{},
		Parameters:   nil,
		ResourceId:   "",
		ResourceType: "",
		RunReason:    "",
		Status:       "Failed",
		UpdatedAt:    workflow.Now(ctx).UTC().Format(format),
		Weblink:      "https://dog.ceo/",
	}

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	logger := workflow.GetLogger(ctx)
	logger.Info("workflow started", "name", params["workflow_instance_id"])

	// setup query handler for query type "state"
	err := workflow.SetQueryHandler(ctx, "state", func() (*schema.WorkflowInstanceUpdate, error) {
		return result, nil
	})
	if err != nil {
		logger.Info("SetQueryHandler failed: " + err.Error())
		return nil, err
	}

	// we have setup everything, now we go into a running state
	result.Status = "Running"
	result.UpdatedAt = workflow.Now(ctx).UTC().Format(format)

	// to simulate workflow been blocked on something, in reality, workflow could wait on anything like activity, signal or timer
	_ = workflow.NewTimer(ctx, time.Second*15).Get(ctx, nil)
	logger.Info("Timer fired")

	var artifact1 *schema.DocumentCreate
	err = workflow.ExecuteActivity(ctx, FetchRandomDogActivity, params).Get(ctx, &artifact1)
	if err != nil {
		result.Status = "Failed"
		logger.Error("FetchRandomDogActivity failed.", "Error", err)
		return nil, err
	}
	artifact1.Description = "Dog 1"
	result.Artifacts = append(result.Artifacts, artifact1)
	result.UpdatedAt = workflow.Now(ctx).UTC().Format(format)

	// to simulate workflow been blocked on something, in reality, workflow could wait on anything like activity, signal or timer
	_ = workflow.NewTimer(ctx, time.Second*15).Get(ctx, nil)

	var artifact2 *schema.DocumentCreate
	err = workflow.ExecuteActivity(ctx, FetchRandomDogActivity, params).Get(ctx, &artifact2)
	if err != nil {
		result.Status = "Failed"
		logger.Error("FetchRandomDogActivity failed.", "Error", err)
		return nil, err
	}
	artifact2.Description = "Dog 2"
	result.Artifacts = append(result.Artifacts, artifact2)

	result.UpdatedAt = workflow.Now(ctx).UTC().Format(format)
	result.Status = "Completed"
	logger.Info("Random Dog workflow completed.", "result", result)
	return result, nil
}

//FetchRandomDogActivity calls the dog.ceo api for a random dog picture
func FetchRandomDogActivity(ctx context.Context, params map[string]interface{}) (*schema.DocumentCreate, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("FetchRandomDogActivity", "name", params["workflow_instance_id"])

	c := http.Client{Timeout: time.Duration(1) * time.Second}
	resp, err := c.Get("https://dog.ceo/api/breeds/image/random")
	if err != nil {
		fmt.Printf("Error %s", err)
		return nil, err
	}
	defer resp.Body.Close()
	randomDog := &dogResponse{}
	err = json.NewDecoder(resp.Body).Decode(randomDog)
	if err != nil {
		return nil, err
	}

	if randomDog.Status != "success" {
		return nil, errors.New(randomDog.Message)
	}

	artifact := &schema.DocumentCreate{
		CreatedAt:                 time.Now().UTC().Format(format),
		Description:               "Random dog",
		Evergreen:                 false,
		Name:                      path.Base(randomDog.Message),
		ResourceId:                path.Base(randomDog.Message),
		ResourceType:              "",
		Filters: []*schema.SearchFilterRead{},
		UpdatedAt:                 time.Now().UTC().Format(format),
		Weblink:                   randomDog.Message,
	}

	return artifact, nil
}

type dogResponse struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}