package analyzer

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/pivotal-golang/clock"
)

type Analyzer struct {
	store store.Store

	logger logger.Logger
	clock  clock.Clock
	conf   *config.Config
}

func New(store store.Store, clock clock.Clock, logger logger.Logger, conf *config.Config) *Analyzer {
	return &Analyzer{
		store:  store,
		clock:  clock,
		logger: logger,
		conf:   conf,
	}
}

func (analyzer *Analyzer) Analyze() (map[string]*models.App, error) {
	err := analyzer.store.VerifyFreshness(analyzer.clock.Now())
	if err != nil {
		analyzer.logger.Error("Store is not fresh", err)
		return nil, err
	}

	apps, err := analyzer.store.GetApps()
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

	for _, app := range apps {
		startMessages, stopMessages, crashCounts := newAppAnalyzer(app, analyzer.clock.Now(), existingPendingStartMessages, existingPendingStopMessages, analyzer.logger, analyzer.conf).analyzeApp()
		for _, startMessage := range startMessages {
			allStartMessages = append(allStartMessages, startMessage)
		}
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

	return apps, nil
}
