package workflows

import (
	"context"
	"database/sql"
	"github.com/jtorvald/temporal-dispatch-poc/schema"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"log"
	"strconv"
	"time"
)

const format = "2006-01-02 15:04:05"

var availableWorkflows map[string]interface{}

//WorkflowClient holds the temporal client and queue
type WorkflowClient struct {
	hostPort string
	queue    string
}

// NewWorkflowStarter returns a new workflow starter with a temporal client
func NewWorkflowStarter(hostPort, queue string) (*WorkflowClient, error) {
	s := &WorkflowClient{}

	if queue == "" {
		queue = "dispatch"
	}
	s.queue = queue

	if hostPort == "" {
		hostPort = "localhost:7233"
	}

	s.hostPort = hostPort

	return s, nil
}

func (ws *WorkflowClient) getClient() (client.Client, error) {
	c, err := client.NewClient(client.Options{
		HostPort: ws.hostPort,
	})

	if err != nil {
		log.Println("Unable to create client", err)
		return nil, err
	}
	return c, nil
}

// StartWorkflowWorker connects to temporal and listens for workflows
func (ws *WorkflowClient) StartWorkflowWorker(ctx context.Context) {
	c, err := ws.getClient()
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
	defer c.Close()

	w := worker.New(c, ws.queue, worker.Options{})

	// dogs
	w.RegisterWorkflow(RandomDogWorkflow)
	w.RegisterActivity(FetchRandomDogActivity)

	// unsplash
	w.RegisterWorkflow(RandomUnsplashWorkflow)
	w.RegisterActivity(InitActivity)
	w.RegisterActivity(FetchRandomUnsplashActivity)

	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}

}

// Start kicks of a workflow
func (ws *WorkflowClient) Start(workflowID string, params map[string]interface{}) *schema.WorkflowInstanceUpdate {
	/*
		{
	   	"workflow_id": "random_unsplash",
	   	"params": {
	   		"incident_id": 32,
	   		"incident_name": "dispatch-default-default-32",
	   		"instance_id": 43,
	   		"term": "love"
	   	}
	   }

	*/
	var startWorkflow interface{}
	var workflowExists bool
	if startWorkflow, workflowExists = availableWorkflows[workflowID]; !workflowExists {
		return &schema.WorkflowInstanceUpdate{
			Artifacts:    []*schema.DocumentCreate{},
			CreatedAt:    time.Now().UTC().Format(format),
			Parameters:   nil,
			ResourceId:   "",
			ResourceType: "",
			RunReason:    "",
			Status:       "Failed",
			UpdatedAt:    time.Now().UTC().Format(format),
			Weblink:      "",
		}
	}

	log.Println("Start workflow params:", params)
	// add instance ID to workflow ID to make sure we get a more unique ID
	combinedID := workflowID
	if v, ok := params["instance_id"]; ok {
		if instanceID, isFloat64 := v.(float64); isFloat64 {
			combinedID += "-" + strconv.Itoa(int(instanceID))
		} else if instanceID, isInt := v.(int); isInt {
			combinedID += "-" + strconv.Itoa(instanceID)
		} else {
			log.Printf("instance_id %T\n", params["instance_id"])
		}
	} else {
		log.Println("Instance_ID not set", params)
	}
	log.Println("Starting workflow ID: ", combinedID)
	workflowOptions := client.StartWorkflowOptions{
		ID:        combinedID,
		TaskQueue: ws.queue,
	}

	c, err := ws.getClient()
	if err != nil {
		return nil
	}
	defer c.Close()

	we, err := c.ExecuteWorkflow(context.Background(), workflowOptions, startWorkflow, params)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	log.Println("Started workflow", "WorkflowID", we.GetID(), "RunID", we.GetRunID())

	weblink := "https://google.com/"
	switch workflowID {
	case "random_dog":
		weblink = "https://dog.ceo/"
		break
	case "random_unsplash":
		weblink = "https://unsplash.com/"
		break
	}

	return &schema.WorkflowInstanceUpdate{
		Artifacts:    []*schema.DocumentCreate{},
		CreatedAt:    time.Now().UTC().Format(format),
		Parameters:   nil,
		ResourceId:   "",
		ResourceType: "",
		RunReason:    "",
		Status:       "Created",
		UpdatedAt:    time.Now().UTC().Format(format),
		Weblink:      weblink,
	}

}

// Query a workflow status
func (ws *WorkflowClient) Query(workflowID, instanceID string) *schema.WorkflowInstanceUpdate {
	queryType := "state"

	if instanceID != "" {
		workflowID += "-" + instanceID
	}
	c, err := ws.getClient()
	if err != nil {
		log.Println("Unable to get client", err)
		return nil
	}
	log.Println("Querying workflow ID: ", workflowID)
	resp, err := c.QueryWorkflow(context.Background(), workflowID, "", queryType)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		log.Println("Unable to query workflow", err)
		return nil
	}
	result := &schema.WorkflowInstanceUpdate{}
	if err := resp.Get(result); err != nil {
		log.Println("Unable to decode query result", err)
		return nil
	}

	if result == nil {
		artifacts := make([]*schema.DocumentCreate, 0)
		result = &schema.WorkflowInstanceUpdate{
			Artifacts:    artifacts,
			CreatedAt:    time.Now().UTC().Format(format),
			Parameters:   nil,
			ResourceId:   "",
			ResourceType: "",
			RunReason:    "",
			Status:       "Created",
			UpdatedAt:    time.Now().UTC().Format(format),
			Weblink:      "https://google.com/",
		}
	}

	return result
}
