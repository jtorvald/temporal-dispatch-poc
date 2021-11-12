package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jtorvald/temporal-dispatch-poc/schema"
	"github.com/jtorvald/temporal-dispatch-poc/workflows"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"
)

type workflowEndpoint struct {
	workflowClient *workflows.WorkflowClient
}

const format = "2006-01-02 15:04:05"

// ServeHTTP is satisfies the http.Handler interface to serve requests
func (h *workflowEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json")
	switch {
	case r.Method == http.MethodGet:
		h.GetWorkflowStatus(w, r)
		return
	case r.Method == http.MethodPost:
		h.PostRunWorkflow(w, r)
		return
	default:
		notFound(w, r)
		return
	}
}

// ListenAndServe starts a http server in the background that listens for api calls to start a workflow or request
// workflow status
func ListenAndServe(ctx context.Context, addr string, workflowStarter *workflows.WorkflowClient) {
	mux := http.NewServeMux()
	mux.Handle("/workflow/", &workflowEndpoint{
		workflowClient: workflowStarter,
	})

	srv := &http.Server{
		Addr:              addr,
		ReadTimeout:       4 * time.Second,
		WriteTimeout:      4 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		Handler:           mux,
	}
	srv.SetKeepAlivesEnabled(true)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	log.Print("Server Started and listening on ", addr)

	go func() {
		<-ctx.Done()
		log.Print("Gracefully shutting down..")

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer func() {
			// extra handling here

			log.Print("Server Stopped")

			cancel()
		}()

		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("Server Shutdown Failed:%+v", err)
		}
	}()
}

// GetWorkflowStatus is one of the request/response handlers and is responsible for
// returning a User given its ID
// it will parse the user id from within the URL Path in the request
func (h *workflowEndpoint) GetWorkflowStatus(w http.ResponseWriter, r *http.Request) {

	workflowID := r.URL.Query().Get("workflow_id")
	instanceID := r.URL.Query().Get("workflow_instance_id")

	log.Println("Query workflow params:", r.URL.Query())

	// Query params:  workflow_id, workflow_instance_id, incident_id and incident_name.
	result := h.workflowClient.Query(workflowID, instanceID)
	if result == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	m := workflowInstanceUpdateToMap(result)

	bts, err := json.Marshal(m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(bts)
}

//PostRunWorkflow handles to post request to the api to start a workflow
func (h *workflowEndpoint) PostRunWorkflow(w http.ResponseWriter, r *http.Request) {
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
	req := &workflowRunRequest{}

	// Try to decode the request body into the struct. If there is an error,
	// respond to the client with the error message and a 400 status code.
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := h.workflowClient.Start(req.WorkflowID, req.Params)
	if result == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	m := workflowInstanceUpdateToMap(result)
	log.Println("mapped start result ", m)

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(m)
	if err != nil {
		log.Println(err)
	}
}

func notFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("not found"))
}

//workflowRunRequest contains the data that needs to start a workflow
type workflowRunRequest struct {
	WorkflowID string                 `json:"workflow_id"`
	Params     map[string]interface{} `json:"params"`
}

// workflowInstanceUpdateToMap returns a map from an workflow instance. This is a little hack to
// workaround the json schema validation in Dispatch that can't handle empty email values in case there is no
// evergreen holder.
func workflowInstanceUpdateToMap(update *schema.WorkflowInstanceUpdate) map[string]interface{} {
	m := map[string]interface{}{}
	//long and bored code
	t := reflect.TypeOf(*update)
	if t.Kind() == reflect.Struct {
		for i := 0; i < t.NumField(); i++ {
			var tag = t.Field(i).Tag.Get("json")
			v := strings.Split(tag, ",")[0] // use split to ignore tag "options" like omitempty, etc.

			r := reflect.ValueOf(update)
			f := reflect.Indirect(r).FieldByName(t.Field(i).Name)

			m[v] = f.Interface()
		}

		if artifacts, ok := m["artifacts"].([]*schema.DocumentCreate); ok {
			artifactsAsMap := make([]interface{}, len(artifacts))
			for i, artifact := range artifacts {
				artifactsAsMap[i] = artifactToMap(artifact)
			}
			m["artifacts"] = artifactsAsMap
		}

	} else {
		fmt.Println("not a stuct ", t.Kind())
	}

	return m
}

// artifactToMap converts a schema.DocumentCreate object to a map of string interface and removes evergreen fields
// that have empty values and are not allowed by the JSON schema validation in Dispatch
func artifactToMap(artifact *schema.DocumentCreate) map[string]interface{} {
	m := map[string]interface{}{}
	//long and bored code
	t := reflect.TypeOf(*artifact)
	if t.Kind() == reflect.Struct {
		for i := 0; i < t.NumField(); i++ {
			var tag = t.Field(i).Tag.Get("json")
			v := strings.Split(tag, ",")[0] // use split to ignore tag "options" like omitempty, etc.

			r := reflect.ValueOf(artifact)
			f := reflect.Indirect(r).FieldByName(t.Field(i).Name)

			m[v] = f.Interface()
		}

		// remove fields that have bad values according to the json schema
		if v, ok := m["evergreen"]; ok {
			if v.(bool) == false {
				delete(m, "evergreen_last_reminder_at")
				delete(m, "evergreen_owner")
				delete(m, "evergreen_reminder_interval")
			}
		}

	} else {
		fmt.Println("not a stuct ", t.Kind())
	}
	return m
}
