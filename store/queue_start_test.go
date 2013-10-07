package store_test

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("Storing PendingStartMessages", func() {
	var (
		store       Store
		etcdAdapter storeadapter.StoreAdapter
		conf        config.Config
		message1    models.PendingStartMessage
		message2    models.PendingStartMessage
		message3    models.PendingStartMessage
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()
		Ω(err).ShouldNot(HaveOccured())
		etcdAdapter = storeadapter.NewETCDStoreAdapter(etcdRunner.NodeURLS(), conf.StoreMaxConcurrentRequests)
		err = etcdAdapter.Connect()
		Ω(err).ShouldNot(HaveOccured())

		message1 = models.NewPendingStartMessage(time.Unix(100, 0), 10, 4, "ABC", "123", 1, 1.0)
		message2 = models.NewPendingStartMessage(time.Unix(100, 0), 10, 4, "DEF", "123", 1, 1.0)
		message3 = models.NewPendingStartMessage(time.Unix(100, 0), 10, 4, "ABC", "456", 1, 1.0)

		store = NewStore(conf, etcdAdapter, fakelogger.NewFakeLogger())
	})

	AfterEach(func() {
		etcdAdapter.Disconnect()
	})

	Describe("Saving start messages", func() {
		BeforeEach(func() {
			err := store.SavePendingStartMessages([]models.PendingStartMessage{
				message1,
				message2,
			})
			Ω(err).ShouldNot(HaveOccured())
		})

		It("stores the passed in start messages", func() {
			nodes, err := etcdAdapter.List("/start")
			Ω(err).ShouldNot(HaveOccured())
			Ω(nodes).Should(HaveLen(2))
			Ω(nodes).Should(ContainElement(storeadapter.StoreNode{
				Key:   "/start/" + message1.StoreKey(),
				Value: message1.ToJSON(),
				TTL:   0,
			}))
			Ω(nodes).Should(ContainElement(storeadapter.StoreNode{
				Key:   "/start/" + message2.StoreKey(),
				Value: message2.ToJSON(),
				TTL:   0,
			}))
		})
	})

	Describe("Fetching start message", func() {
		Context("When the start message is present", func() {
			BeforeEach(func() {
				err := store.SavePendingStartMessages([]models.PendingStartMessage{
					message1,
					message2,
				})
				Ω(err).ShouldNot(HaveOccured())
			})

			It("can fetch the start message", func() {
				desired, err := store.GetPendingStartMessages()
				Ω(err).ShouldNot(HaveOccured())
				Ω(desired).Should(HaveLen(2))
				Ω(desired).Should(ContainElement(message1))
				Ω(desired).Should(ContainElement(message2))
			})
		})

		Context("when the start message is empty", func() {
			BeforeEach(func() {
				hb := message1
				err := store.SavePendingStartMessages([]models.PendingStartMessage{hb})
				Ω(err).ShouldNot(HaveOccured())
				err = store.DeletePendingStartMessages([]models.PendingStartMessage{hb})
				Ω(err).ShouldNot(HaveOccured())
			})

			It("returns an empty array", func() {
				start, err := store.GetPendingStartMessages()
				Ω(err).ShouldNot(HaveOccured())
				Ω(start).Should(BeEmpty())
			})
		})

		Context("When the start message key is missing", func() {
			BeforeEach(func() {
				_, err := etcdAdapter.List("/start")
				Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))
			})

			It("returns an empty array and no error", func() {
				start, err := store.GetPendingStartMessages()
				Ω(err).ShouldNot(HaveOccured())
				Ω(start).Should(BeEmpty())
			})
		})
	})

	Describe("Deleting start message", func() {
		BeforeEach(func() {
			err := store.SavePendingStartMessages([]models.PendingStartMessage{
				message1,
				message2,
				message3,
			})
			Ω(err).ShouldNot(HaveOccured())
		})

		Context("When the start message is present", func() {
			It("can delete the start message (and only cares about the relevant fields)", func() {
				toDelete := []models.PendingStartMessage{
					models.PendingStartMessage{AppGuid: message1.AppGuid, AppVersion: message1.AppVersion, IndexToStart: message1.IndexToStart},
					models.PendingStartMessage{AppGuid: message3.AppGuid, AppVersion: message3.AppVersion, IndexToStart: message3.IndexToStart},
				}
				err := store.DeletePendingStartMessages(toDelete)
				Ω(err).ShouldNot(HaveOccured())

				desired, err := store.GetPendingStartMessages()
				Ω(err).ShouldNot(HaveOccured())
				Ω(desired).Should(HaveLen(1))
				Ω(desired).Should(ContainElement(message2))
			})
		})

		Context("When the desired message key is not present", func() {
			It("returns an error, but does leave things in a broken state... for now...", func() {
				toDelete := []models.PendingStartMessage{
					models.PendingStartMessage{AppGuid: message1.AppGuid, AppVersion: message1.AppVersion, IndexToStart: message1.IndexToStart},
					models.PendingStartMessage{AppGuid: "floobedey", AppVersion: "abc"},
					models.PendingStartMessage{AppGuid: message3.AppGuid, AppVersion: message3.AppVersion, IndexToStart: message3.IndexToStart},
				}
				err := store.DeletePendingStartMessages(toDelete)
				Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))

				start, err := store.GetPendingStartMessages()
				Ω(err).ShouldNot(HaveOccured())
				Ω(start).Should(HaveLen(2))
				Ω(start).Should(ContainElement(message2))
				Ω(start).Should(ContainElement(message3))
			})
		})
	})
})
