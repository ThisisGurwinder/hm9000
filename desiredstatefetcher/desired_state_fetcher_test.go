package desiredstatefetcher_test

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/hm9000/config"
	. "github.com/cloudfoundry/hm9000/desiredstatefetcher"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	. "github.com/cloudfoundry/hm9000/testhelpers/custommatchers"
	"github.com/cloudfoundry/hm9000/testhelpers/fakestore"

	"github.com/cloudfoundry/hm9000/testhelpers/fakehttpclient"
	"github.com/cloudfoundry/hm9000/testhelpers/faketimeprovider"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
)

type brokenReader struct{}

func (b *brokenReader) Read([]byte) (int, error) {
	return 0, errors.New("oh no you didn't!")
}

func (b *brokenReader) Close() error {
	return nil
}

var _ = Describe("DesiredStateFetcher", func() {
	var (
		conf         config.Config
		fetcher      *DesiredStateFetcher
		httpClient   *fakehttpclient.FakeHttpClient
		timeProvider *faketimeprovider.FakeTimeProvider
		store        *fakestore.FakeStore
		resultChan   chan DesiredStateFetcherResult
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()
		Ω(err).ShouldNot(HaveOccured())

		resultChan = make(chan DesiredStateFetcherResult, 1)
		timeProvider = &faketimeprovider.FakeTimeProvider{
			TimeToProvide: time.Now(),
		}

		httpClient = fakehttpclient.NewFakeHttpClient()
		store = fakestore.NewFakeStore()

		fetcher = New(conf, store, httpClient, timeProvider)
		fetcher.Fetch(resultChan)
	})

	Describe("Fetching with an invalid URL", func() {
		BeforeEach(func() {
			conf.CCBaseURL = "http://example.com/#%ZZ"
			fetcher = New(conf, store, httpClient, timeProvider)
			fetcher.Fetch(resultChan)
		})

		It("should send an error down the result channel", func(done Done) {
			result := <-resultChan
			Ω(result.Success).Should(BeFalse())
			Ω(result.Message).Should(Equal("Failed to generate URL request"))
			Ω(result.Error).Should(HaveOccured())
			close(done)
		}, 0.1)
	})

	Describe("Fetching batches", func() {
		var response DesiredStateServerResponse

		It("should make the correct request", func() {
			Ω(httpClient.Requests).Should(HaveLen(1))
			request := httpClient.Requests[0]

			Ω(request.URL.String()).Should(ContainSubstring(conf.CCBaseURL))
			Ω(request.URL.Path).Should(ContainSubstring("/bulk/apps"))

			expectedAuth := models.BasicAuthInfo{
				User:     "mcat",
				Password: "testing",
			}.Encode()

			Ω(request.Header.Get("Authorization")).Should(Equal(expectedAuth))
		})

		It("should request a batch size with an empty bulk token", func() {
			query := httpClient.LastRequest().URL.Query()
			Ω(query.Get("batch_size")).Should(Equal(fmt.Sprintf("%d", conf.DesiredStateBatchSize)))
			Ω(query.Get("bulk_token")).Should(Equal("{}"))
		})

		assertFailure := func(expectedMessage string, numRequests int) {
			It("should stop requesting batches", func() {
				Ω(httpClient.Requests).Should(HaveLen(numRequests))
			})

			It("should not bump the freshness", func() {
				Ω(store.DesiredFreshnessTimestamp).Should(BeZero())
			})

			It("should send an error down the result channel", func(done Done) {
				result := <-resultChan
				Ω(result.Success).Should(BeFalse())
				Ω(result.Message).Should(Equal(expectedMessage))
				Ω(result.Error).Should(HaveOccured())
				close(done)
			}, 1.0)
		}

		Context("when a response with desired state is received", func() {
			var (
				a1         app.App
				a2         app.App
				stoppedApp app.App
				deletedApp app.App
			)

			BeforeEach(func() {
				deletedApp = app.NewApp()
				store.SaveDesiredState([]models.DesiredAppState{deletedApp.DesiredState(1)})

				a1 = app.NewApp()
				a2 = app.NewApp()
				stoppedApp = app.NewApp()
				stoppedDesiredState := stoppedApp.DesiredState(1)
				stoppedDesiredState.State = models.AppStateStopped
				response = DesiredStateServerResponse{
					Results: map[string]models.DesiredAppState{
						a1.AppGuid:         a1.DesiredState(1),
						a2.AppGuid:         a2.DesiredState(1),
						stoppedApp.AppGuid: stoppedDesiredState,
					},
					BulkToken: BulkToken{
						Id: 5,
					},
				}

				httpClient.LastRequest().Succeed(response.ToJSON())
			})

			It("should not store the desired state (yet)", func() {
				desired, _ := store.GetDesiredState()
				Ω(desired).Should(HaveLen(1))
				Ω(desired[0]).Should(EqualDesiredState(deletedApp.DesiredState(1)))
			})

			It("should request the next batch", func() {
				Ω(httpClient.Requests).Should(HaveLen(2))
				Ω(httpClient.LastRequest().URL.Query().Get("bulk_token")).Should(Equal(response.BulkTokenRepresentation()))
			})

			It("should not bump the freshness yet", func() {
				Ω(store.DesiredFreshnessTimestamp).Should(BeZero())
			})

			It("should not send a result down the resultChan yet", func() {
				Ω(resultChan).Should(HaveLen(0))
			})

			Context("when an empty response is received", func() {
				JustBeforeEach(func() {
					response = DesiredStateServerResponse{
						Results: map[string]models.DesiredAppState{},
						BulkToken: BulkToken{
							Id: 17,
						},
					}

					httpClient.LastRequest().Succeed(response.ToJSON())
				})

				It("should stop requesting batches", func() {
					Ω(httpClient.Requests).Should(HaveLen(2))
				})

				It("should bump the freshness", func() {
					Ω(store.DesiredFreshnessTimestamp).Should(Equal(timeProvider.Time()))
				})

				It("should store any desired state that is in the STARTED appstate, and delete any stale data", func() {
					desired, _ := store.GetDesiredState()
					Ω(desired).Should(HaveLen(2))
					Ω(desired).Should(ContainElement(EqualDesiredState(a1.DesiredState(1))))
					Ω(desired).Should(ContainElement(EqualDesiredState(a2.DesiredState(1))))
				})

				It("should send a succesful result down the result channel", func(done Done) {
					result := <-resultChan
					Ω(result.Success).Should(BeTrue())
					Ω(result.Message).Should(BeZero())
					Ω(result.Error).ShouldNot(HaveOccured())
					close(done)
				}, 0.1)

				Context("and it fails to write to the store", func() {
					BeforeEach(func() {
						store.SaveDesiredStateError = errors.New("oops!")
					})

					assertFailure("Failed to sync desired state to the store", 2)
				})

				Context("and it fails to read from the store", func() {
					BeforeEach(func() {
						store.GetDesiredStateError = errors.New("oops!")
					})

					assertFailure("Failed to sync desired state to the store", 2)
				})
			})
		})

		Context("when an unauthorized response is received", func() {
			BeforeEach(func() {
				httpClient.LastRequest().RespondWithStatus(http.StatusUnauthorized)
			})

			assertFailure("HTTP request received unauthorized response code", 1)
		})

		Context("when the HTTP request returns a non-200 response", func() {
			BeforeEach(func() {
				httpClient.LastRequest().RespondWithStatus(http.StatusNotFound)
			})

			assertFailure("HTTP request received non-200 response (404)", 1)
		})

		Context("when the HTTP request fails with an error", func() {
			BeforeEach(func() {
				httpClient.LastRequest().RespondWithError(errors.New(":("))
			})

			assertFailure("HTTP request failed with error", 1)
		})

		Context("when a broken body is received", func() {
			BeforeEach(func() {
				response := &http.Response{
					Status:     "StatusOK (200)",
					StatusCode: http.StatusOK,

					ContentLength: 17,
					Body:          &brokenReader{},
				}

				httpClient.LastRequest().Callback(response, nil)
			})

			assertFailure("Failed to read HTTP response body", 1)
		})

		Context("when a malformed response is received", func() {
			BeforeEach(func() {
				httpClient.LastRequest().Succeed([]byte("ß"))
			})

			assertFailure("Failed to parse HTTP response body JSON", 1)
		})
	})
})
