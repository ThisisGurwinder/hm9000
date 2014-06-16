package mcat_test

import (
	"time"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/vito/cmdtest/matchers"
)

var _ = Describe("Locking", func() {
	Describe("vieing for the lock", func() {
		Context("when two long-lived processes try to run", func() {
			It("one waits for the other to exit and then grabs the lock", func() {
				listenerA := cliRunner.StartSession("listen", 1)

				Ω(listenerA).Should(Say("Acquired lock"))
				defer interruptSession(listenerA)

				listenerB := cliRunner.StartSession("listen", 1)
				defer interruptSession(listenerB)

				Ω(listenerB).Should(Say("Acquiring"))
				Ω(listenerB).ShouldNot(SayWithTimeout("Acquired", 1*time.Second))

				interruptSession(listenerA)

				coordinator.StoreRunner.FastForwardTime(10)

				Ω(listenerB).Should(SayWithTimeout("Acquired", 3*time.Second))
			})
		})

		Context("when two polling processes try to run", func() {
			It("one waits for the other to exit and then grabs the lock", func() {
				analyzerA := cliRunner.StartSession("analyze", 1, "--poll")
				defer interruptSession(analyzerA)

				Ω(analyzerA).Should(Say("Acquired lock"))

				analyzerB := cliRunner.StartSession("analyze", 1, "--poll")
				defer interruptSession(analyzerB)

				Ω(analyzerB).Should(Say("Acquiring"))
				Ω(analyzerB).ShouldNot(SayWithTimeout("Acquired", 1*time.Second))

				interruptSession(analyzerA)

				coordinator.StoreRunner.FastForwardTime(10)

				Ω(analyzerB).Should(SayWithTimeout("Acquired", 3*time.Second))
			})
		})
	})

	Context("when the lock disappears", func() {
		Context("long-lived processes", func() {
			It("should exit 17", func() {
				listenerA := cliRunner.StartSession("listen", 1)
				defer interruptSession(listenerA)

				Ω(listenerA).Should(Say("Acquired lock"))

				coordinator.StoreAdapter.Delete("/hm/locks")

				Ω(listenerA).Should(Say("Lost the lock"))
				status, err := listenerA.Wait(20 * time.Second)

				Ω(err).ShouldNot(HaveOccurred())
				Ω(status).Should(Equal(197))
			})
		})

		Context("polling processes", func() {
			It("should exit 17", func() {
				analyzerA := cliRunner.StartSession("analyze", 1, "--poll")
				defer interruptSession(analyzerA)

				Ω(analyzerA).Should(Say("Acquired lock"))

				coordinator.StoreAdapter.Delete("/hm/locks")

				Ω(analyzerA).Should(Say("Lost the lock"))
				status, err := analyzerA.Wait(20 * time.Second)

				Ω(err).ShouldNot(HaveOccurred())
				Ω(status).Should(Equal(197))
			})
		})
	})
})
