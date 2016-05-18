package analyzer

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

type Analyzer struct {
	store store.Store

	logger lager.Logger
	clock  clock.Clock
	conf   *config.Config
}

func New(store store.Store, clock clock.Clock, logger lager.Logger, conf *config.Config,
) *Analyzer {
	return &Analyzer{
		store:  store,
		clock:  clock,
		logger: logger,
		conf:   conf,
	}
}

func (analyzer *Analyzer) Analyze(desiredAppQueue chan map[string]models.DesiredAppState) (map[string]*models.App, error) {
	err := analyzer.store.VerifyFreshness(analyzer.clock.Now())
	if err != nil {
		analyzer.logger.Error("Store is not fresh", err)
		return nil, err
	}

	runningApps, err := analyzer.store.GetApps()

	appsNotDesired := make(map[string]*models.App)
	for k, v := range runningApps {
		appsNotDesired[k] = v
	}

	if err != nil {
		analyzer.logger.Error("Failed to fetch apps", err)
		return nil, err
	}

	existingPendingStartMessages, err := analyzer.store.GetPendingStartMessages()
	if err != nil {
		analyzer.logger.Error("Failed to fetch pending start messages", err)
		return nil, err
	}

	existingPendingStopMessages, err := analyzer.store.GetPendingStopMessages()
	if err != nil {
		analyzer.logger.Error("Failed to fetch pending stop messages", err)
		return nil, err
	}

	allStartMessages := []models.PendingStartMessage{}
	allStopMessages := []models.PendingStopMessage{}
	allCrashCounts := []models.CrashCount{}

	startMessagesToDelete := existingPendingStartMessages
	stopMessagesToDelete := existingPendingStopMessages

	for desiredAppBatch := range desiredAppQueue {
		for _, desiredApp := range desiredAppBatch {
			app := runningApps[desiredApp.StoreKey()]

			if app == nil {
				app = models.NewApp(desiredApp.AppGuid, desiredApp.AppVersion, desiredApp, nil, nil)
				runningApps[desiredApp.StoreKey()] = app
			} else {
				app.Desired = desiredApp
				delete(appsNotDesired, desiredApp.StoreKey())
			}

			startMessages, stopMessages, crashCounts, _, _ := newAppAnalyzer(app,
				analyzer.clock.Now(),
				existingPendingStartMessages,
				existingPendingStopMessages,
				startMessagesToDelete,
				stopMessagesToDelete,
				analyzer.logger,
				analyzer.conf).analyzeApp()
			for _, startMessage := range startMessages {
				allStartMessages = append(allStartMessages, startMessage)
			}
			for _, stopMessage := range stopMessages {
				allStopMessages = append(allStopMessages, stopMessage)
			}
			allCrashCounts = append(allCrashCounts, crashCounts...)
		}
	}

	for _, app := range appsNotDesired {
		_, stopMessages, crashCounts, _, _ := newAppAnalyzer(app,
			analyzer.clock.Now(),
			existingPendingStartMessages,
			existingPendingStopMessages,
			startMessagesToDelete,
			stopMessagesToDelete,
			analyzer.logger,
			analyzer.conf).analyzeApp()
		//TODO: start messages should not be created
		for _, stopMessage := range stopMessages {
			allStopMessages = append(allStopMessages, stopMessage)
		}
		allCrashCounts = append(allCrashCounts, crashCounts...)
	}

	err = analyzer.store.SaveCrashCounts(allCrashCounts...)
	if err != nil {
		analyzer.logger.Error("Analyzer failed to save crash counts", err)
		return nil, err
	}

	err = analyzer.store.SavePendingStartMessages(allStartMessages...)
	if err != nil {
		analyzer.logger.Error("Analyzer failed to enqueue start messages", err)
		return nil, err
	}

	err = analyzer.store.SavePendingStopMessages(allStopMessages...)
	if err != nil {
		analyzer.logger.Error("Analyzer failed to enqueue stop messages", err)
		return nil, err
	}

	return runningApps, nil
}
