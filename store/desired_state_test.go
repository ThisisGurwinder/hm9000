package store_test

import (
	. "github.com/cloudfoundry/hm9000/store"
	. "github.com/cloudfoundry/hm9000/testhelpers/custommatchers"
	"github.com/cloudfoundry/storeadapter/workerpool"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
)

var _ = Describe("Desired State", func() {
	var (
		store        Store
		storeAdapter storeadapter.StoreAdapter
		conf         *config.Config
		app1         appfixture.AppFixture
		app2         appfixture.AppFixture
		app3         appfixture.AppFixture
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()
		Ω(err).ShouldNot(HaveOccurred())
		storeAdapter = etcdstoreadapter.NewETCDStoreAdapter(etcdRunner.NodeURLS(), workerpool.NewWorkerPool(conf.StoreMaxConcurrentRequests))
		err = storeAdapter.Connect()
		Ω(err).ShouldNot(HaveOccurred())

		app1 = appfixture.NewAppFixture()
		app2 = appfixture.NewAppFixture()
		app3 = appfixture.NewAppFixture()

		store = NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())
	})

	AfterEach(func() {
		storeAdapter.Disconnect()
	})

	Describe("Syncing desired state", func() {
		BeforeEach(func() {
			err := store.SyncDesiredState(
				app1.DesiredState(1),
				app2.DesiredState(1),
			)
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("should store the passed in desired state", func() {
			desiredState, err := store.GetDesiredState()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(desiredState).Should(HaveLen(2))
			Ω(desiredState[app1.DesiredState(1).StoreKey()]).Should(EqualDesiredState(app1.DesiredState(1)))
			Ω(desiredState[app2.DesiredState(1).StoreKey()]).Should(EqualDesiredState(app2.DesiredState(1)))
		})

		Context("When the desired state already exists", func() {
			Context("and the state-to-sync has differences", func() {
				BeforeEach(func() {
					err := store.SyncDesiredState(
						app2.DesiredState(2),
						app3.DesiredState(1),
					)
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("should store the differences, adding any new state and removing any unrepresented state", func() {
					desiredState, err := store.GetDesiredState()
					Ω(err).ShouldNot(HaveOccurred())

					Ω(desiredState).Should(HaveLen(2))
					Ω(desiredState[app2.DesiredState(2).StoreKey()]).Should(EqualDesiredState(app2.DesiredState(2)))
					Ω(desiredState[app3.DesiredState(1).StoreKey()]).Should(EqualDesiredState(app3.DesiredState(1)))
				})
			})
		})
	})

	Describe("Fetching desired state", func() {
		Context("When the desired state is present", func() {
			BeforeEach(func() {
				err := store.SyncDesiredState(
					app1.DesiredState(1),
					app2.DesiredState(1),
				)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("can fetch the desired state", func() {
				desired, err := store.GetDesiredState()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(desired[app1.DesiredState(1).StoreKey()]).Should(EqualDesiredState(app1.DesiredState(1)))
				Ω(desired[app2.DesiredState(1).StoreKey()]).Should(EqualDesiredState(app2.DesiredState(1)))
			})
		})

		Context("when the desired state is empty", func() {
			It("returns an empty hash", func() {
				desired, err := store.GetDesiredState()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(desired).Should(BeEmpty())
			})
		})
	})
})
