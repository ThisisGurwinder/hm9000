package desiredstateserver

import (
	"encoding/json"
	"fmt"
	"github.com/cloudfoundry/go_cfmessagebus"
	. "github.com/cloudfoundry/hm9000/models"
	"net/http"
	"strconv"
)

type DesiredStateServerInterface interface {
	SpinUp(port int)
	SetDesiredState([]DesiredAppState)
}

type DesiredStateServer struct {
	messageBus              cfmessagebus.MessageBus
	apps                    []DesiredAppState
	NumberOfCompleteFetches int
}

type desiredStateServerResponse struct {
	Results   map[string]DesiredAppState `json:"results"`
	BulkToken struct {
		Id int `json:"id"`
	} `json:"bulk_token"`
}

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func NewDesiredStateServer(messageBus cfmessagebus.MessageBus) *DesiredStateServer {
	server := &DesiredStateServer{
		messageBus: messageBus,
	}

	messageBus.RespondToChannel("cloudcontroller.bulk.credentials.default", func(request []byte) []byte {
		return []byte(`{"user":"mcat","password":"testing"}`)
	})

	return server
}

func (server *DesiredStateServer) SpinUp(port int) {
	http.HandleFunc("/bulk/apps", func(w http.ResponseWriter, r *http.Request) {
		server.handleApps(w, r)
	})

	http.HandleFunc("/bulk/counts", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"counts":{"user":17}}`)
	})

	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

func (server *DesiredStateServer) SetDesiredState(newState []DesiredAppState) {
	server.apps = newState
}

func (server *DesiredStateServer) Reset() {
	server.apps = make([]DesiredAppState, 0)
	server.NumberOfCompleteFetches = 0
}

func (server *DesiredStateServer) GetNumberOfCompleteFetches() int {
	return server.NumberOfCompleteFetches
}

func (server *DesiredStateServer) handleApps(w http.ResponseWriter, r *http.Request) {
	credentials := r.Header.Get("Authorization")
	if credentials != "Basic bWNhdDp0ZXN0aW5n" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	batchSize := server.extractBatchSize(r)
	bulkToken := server.extractBulkToken(r)

	endIndex := bulkToken + batchSize
	endIndex = min(endIndex, len(server.apps))

	results := make(map[string]DesiredAppState, 0)

	for _, app := range server.apps[bulkToken:endIndex] {
		results[app.AppGuid] = app
	}

	if bulkToken == len(server.apps) {
		server.NumberOfCompleteFetches += 1
	}

	response := desiredStateServerResponse{
		Results: results,
	}
	response.BulkToken.Id = endIndex
	json.NewEncoder(w).Encode(response)
}

func (server *DesiredStateServer) extractBatchSize(r *http.Request) int {
	batchSize, _ := strconv.Atoi(r.URL.Query()["batch_size"][0])
	return batchSize
}

func (server *DesiredStateServer) extractBulkToken(r *http.Request) int {
	var bulkToken map[string]interface{}
	json.Unmarshal([]byte(r.URL.Query()["bulk_token"][0]), &bulkToken)

	bulkTokenIndex := 0
	if bulkToken["id"] != nil {
		bulkTokenIndex = int(bulkToken["id"].(float64))
	}

	return bulkTokenIndex
}
