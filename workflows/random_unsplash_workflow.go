package workflows

import (
	"context"
	"fmt"
	"github.com/jtorvald/temporal-dispatch-poc/schema"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
	"net/http"
	"net/url"
	"path"
	"time"
)

func init() {
	if availableWorkflows == nil {
		availableWorkflows = make(map[string]interface{})
	}
	availableWorkflows["random_unsplash"] = RandomUnsplashWorkflow
}

// RandomUnsplashWorkflow is a Hello World workflow definition.
func RandomUnsplashWorkflow(ctx workflow.Context, params map[string]interface{}) (*schema.WorkflowInstanceUpdate, error) {
	/*
	   	params:
	   	  {
	          "term": "nature",
	          "incident_id": 32,
	          "incident_name": "dispatch-singlesproject-default-32",
	          "instance_id": 25
	        }
	*/
	result := &schema.WorkflowInstanceUpdate{
		CreatedAt:    workflow.Now(ctx).UTC().Format(format),
		Artifacts:    []*schema.DocumentCreate{},
		Parameters:   nil,
		ResourceId:   "",
		ResourceType: "",
		RunReason:    "",
		Status:       "Created",
		UpdatedAt:    time.Now().UTC().Format(format),
		Weblink:      "https://unsplash.com/",
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
		result.Status = "Failed"
		logger.Info("SetQueryHandler failed: " + err.Error())
		return result, err
	}
	err = workflow.ExecuteActivity(ctx, initActivity, params).Get(ctx, &result.Status)
	if err != nil {
		result.Status = "Failed"
		logger.Error("InitActivity failed.", "Error", err)
		return result, err
	}
	result.UpdatedAt = workflow.Now(ctx).UTC().Format(format)

	// to simulate workflow been blocked on something, in reality, workflow could wait on anything like activity, signal or timer
	_ = workflow.NewTimer(ctx, time.Second*15).Get(ctx, nil)

	var artifact1 *schema.DocumentCreate
	err = workflow.ExecuteActivity(ctx, FetchRandomUnsplashActivity, params).Get(ctx, &artifact1)
	if err != nil {
		result.Status = "Failed"
		logger.Error("FetchRandomUnsplashActivity failed.", "Error", err)
		return nil, err
	}
	artifact1.Description = "Photo 1"
	result.Artifacts = append(result.Artifacts, artifact1)
	result.UpdatedAt = workflow.Now(ctx).UTC().Format(format)

	// to simulate workflow been blocked on something, in reality, workflow could wait on anything like activity, signal or timer
	_ = workflow.NewTimer(ctx, time.Second*15).Get(ctx, nil)

	var artifact2 *schema.DocumentCreate
	err = workflow.ExecuteActivity(ctx, FetchRandomUnsplashActivity, params).Get(ctx, &artifact2)
	if err != nil {
		result.Status = "Failed"
		logger.Error("FetchRandomUnsplashActivity failed.", "Error", err)
		return nil, err
	}
	artifact2.Description = "Photo 2"
	result.Artifacts = append(result.Artifacts, artifact2)
	result.UpdatedAt = workflow.Now(ctx).UTC().Format(format)

	result.Status = "Completed"
	logger.Info("Random Unsplash workflow completed.", "result", result)

	return result, nil
}

// initActivity
func initActivity(ctx context.Context, params map[string]interface{}) (string, error) {
	return "Running", nil
}

// FetchRandomUnsplashActivity calls unsplash to get a random photo
func FetchRandomUnsplashActivity(ctx context.Context, params map[string]interface{}) (*schema.DocumentCreate, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("FetchRandomUnsplashActivity", "name", params["workflow_instance_id"])

	requestURL := "https://source.unsplash.com/random/400x320?"
	var term string
	if foo, ok := params["term"]; ok {
		if term, ok = foo.(string); ok {
			requestURL += url.QueryEscape(term)
		}
	}

	c := http.Client{Timeout: time.Duration(1) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}}

	resp, err := c.Head(requestURL)
	if err != nil {
		fmt.Printf("Error %s", err)
		return nil, err
	}
	defer resp.Body.Close()

	newLocation := resp.Header.Get("Location")

	var filename string
	if parts, err := url.Parse(newLocation); err != nil {
		filename = path.Base(newLocation)
	} else {
		filename = path.Base(parts.Path)
	}

	artifact := &schema.DocumentCreate{
		CreatedAt:    time.Now().UTC().Format(format),
		Description:  "Random photo from Unsplash",
		Evergreen:    false,
		Name:         filename,
		ResourceId:   filename,
		ResourceType: "",
		Filters:      []*schema.SearchFilterRead{},
		UpdatedAt:    time.Now().UTC().Format(format),
		Weblink:      newLocation,
	}

	return artifact, nil
}
