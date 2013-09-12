package desiredstatepoller

import (
	"fmt"
	"github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/hm9000/helpers/http_client"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"net/http"
)

const initialBulkToken = "{}"

type desiredStatePoller struct {
	messageBus    cfmessagebus.MessageBus
	store         store.Store
	authorization string
	ccBaseURL     string
}

func NewDesiredStatePoller(messageBus cfmessagebus.MessageBus, store store.Store, ccBaseURL string) *desiredStatePoller {
	return &desiredStatePoller{messageBus: messageBus, store: store, ccBaseURL: ccBaseURL}
}

func (poller *desiredStatePoller) Poll() {
	if poller.authenticated() {
		poller.fetch()
	} else {
		poller.messageBus.Request("cloudcontroller.bulk.credentials.default", []byte{}, func(response []byte) {
			authInfo, err := models.NewBasicAuthInfoFromJSON(response)
			if err != nil {
				//TODO: Log
				return
			}

			poller.authorization = authInfo.Encode()
			poller.fetch()
		})
	}
}

func (poller *desiredStatePoller) authenticated() bool {
	return poller.authorization != ""
}

func (poller *desiredStatePoller) bulkURL(batchSize int, bulkToken string) string {
	return fmt.Sprintf("%s/bulk/apps?batch_size=%d&bulk_token=%s", poller.ccBaseURL, batchSize, bulkToken)
}

func (poller *desiredStatePoller) fetch() {
	req, err := http.NewRequest("GET", poller.bulkURL(10, initialBulkToken), nil)

	if err != nil {
		//TODO: Log
		return
	}

	req.Header.Add("Authorization", poller.authorization)

	factory := &http_client.RealHttpClientFactory{}

	client := factory.NewClient()
	responseErr := <-client.Do(req)
	defer responseErr.Response.Body.Close()

	if responseErr.Err != nil {
		//TODO: Log
		return
	}

	body := make([]byte, responseErr.Response.ContentLength)
	_, err = responseErr.Response.Body.Read(body)

	if err != nil {
		//TODO: Log
		return
	}

	desiredState, err := NewDesiredStateServerResponse(body)
	if err != nil {
		//TODO: Log
		return
	}

	poller.pushToStore(desiredState)
}

func (poller *desiredStatePoller) pushToStore(desiredState desiredStateServerResponse) {
	for _, desiredAppState := range desiredState.Results {
		key := "/desired/" + desiredAppState.AppGuid + "-" + desiredAppState.AppVersion
		poller.store.Set(key, desiredAppState.ToJson(), 10*60)
	}
}
