package actualstatelistener_test

import (
	"errors"
	. "github.com/cloudfoundry/hm9000/actualstatelistener"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"time"

	. "github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/testhelpers/app"

	"github.com/cloudfoundry/go_cfmessagebus/fake_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/hm9000/testhelpers/fakestore"
	"github.com/cloudfoundry/hm9000/testhelpers/faketimeprovider"
)

var _ = Describe("Actual state listener", func() {
	var (
		app          App
		anotherApp   App
		store        *fakestore.FakeStore
		listener     *ActualStateListener
		timeProvider *faketimeprovider.FakeTimeProvider
		messageBus   *fake_cfmessagebus.FakeMessageBus
		logger       *fakelogger.FakeLogger
		conf         config.Config
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()
		Ω(err).ShouldNot(HaveOccured())

		timeProvider = &faketimeprovider.FakeTimeProvider{
			TimeToProvide: time.Now(),
		}

		app = NewApp()
		anotherApp = NewApp()

		store = fakestore.NewFakeStore()
		messageBus = fake_cfmessagebus.NewFakeMessageBus()
		logger = fakelogger.NewFakeLogger()

		listener = New(conf, messageBus, store, timeProvider, logger)
		listener.Start()
	})

	It("should subscribe to the dea.heartbeat subject", func() {
		Ω(messageBus.Subscriptions).Should(HaveKey("dea.heartbeat"))
		Ω(messageBus.Subscriptions["dea.heartbeat"]).Should(HaveLen(1))
	})

	Context("When it receives a simple heartbeat over the message bus", func() {
		BeforeEach(func() {
			messageBus.Subscriptions["dea.heartbeat"][0].Callback(app.Heartbeat(1, 17).ToJSON())
		})

		It("Stores it in the store", func() {
			actual, _ := store.GetActualState()
			Ω(actual).Should(ContainElement(app.GetInstance(0).Heartbeat(17)))
		})
	})

	Context("When it receives a complex heartbeat with multiple apps and instances", func() {
		JustBeforeEach(func() {
			Ω(store.ActualFreshnessTimestamp).Should(BeZero())

			heartbeat := Heartbeat{
				DeaGuid: Guid(),
				InstanceHeartbeats: []InstanceHeartbeat{
					app.GetInstance(0).Heartbeat(17),
					app.GetInstance(1).Heartbeat(22),
					anotherApp.GetInstance(0).Heartbeat(11),
				},
			}

			messageBus.Subscriptions["dea.heartbeat"][0].Callback(heartbeat.ToJSON())
		})

		It("Stores it in the store", func() {
			actual, _ := store.GetActualState()
			Ω(actual).Should(ContainElement(app.GetInstance(0).Heartbeat(17)))
			Ω(actual).Should(ContainElement(app.GetInstance(1).Heartbeat(22)))
			Ω(actual).Should(ContainElement(anotherApp.GetInstance(0).Heartbeat(11)))
		})

		Context("when the save succeeds", func() {
			It("bumps the freshness", func() {
				Ω(store.ActualFreshnessTimestamp).Should(Equal(timeProvider.Time()))
				Ω(logger.LoggedSubjects).Should(BeEmpty())
			})

			Context("when the freshness bump fails", func() {
				BeforeEach(func() {
					store.BumpActualFreshnessError = errors.New("oops")
				})

				It("logs about the failed freshness bump", func() {
					Ω(logger.LoggedSubjects).Should(ContainElement("Could not update actual freshness"))
				})
			})
		})

		Context("when the save fails", func() {
			BeforeEach(func() {
				store.SaveActualStateError = errors.New("oops")
			})

			It("does not bump the freshness", func() {
				Ω(store.ActualFreshnessTimestamp).Should(BeZero())
			})

			It("logs about the failed save", func() {
				Ω(logger.LoggedSubjects).Should(ContainElement(ContainSubstring("Could not put instance heartbeats in store")))
			})
		})
	})

	Context("When it fails to parse the heartbeat message", func() {
		BeforeEach(func() {
			messageBus.Subscriptions["dea.heartbeat"][0].Callback([]byte("ß"))
		})

		It("Stores nothing in the store", func() {
			actual, _ := store.GetActualState()
			Ω(actual).Should(BeEmpty())
		})

		It("does not bump the freshness", func() {
			Ω(store.ActualFreshnessTimestamp).Should(BeZero())
		})

		It("logs about the failed parse", func() {
			Ω(logger.LoggedSubjects).Should(ContainElement("Could not unmarshal heartbeat"))
		})
	})
})
