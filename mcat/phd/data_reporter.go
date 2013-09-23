package phd

import (
	"encoding/csv"
	"fmt"
	g "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"os"
	"strconv"
	"strings"
	"time"
)

type StorePerformanceReport struct {
	StoreType     string
	NumStoreNodes int
	RecordSize    int
	NumRecords    int
	Concurrency   int
	Subject       string
	Average       float64
	StdDeviation  float64
}

func (r StorePerformanceReport) String() string {
	return fmt.Sprintf("%s: %d %s node(s), %dbytes size, %d records, %d concurrency", strings.Title(r.Subject), r.NumStoreNodes, r.StoreType, r.RecordSize, r.NumRecords, r.Concurrency)
}

type DataReporter struct {
	writePerformanceReports []StorePerformanceReport
	readPerformanceReports  []StorePerformanceReport
	timestamp               string
}

func (reporter *DataReporter) SpecSuiteWillBegin(config config.GinkgoConfigType, summary *g.SuiteSummary) {
	reporter.timestamp = fmt.Sprintf("%d", time.Now().Unix())
	reporter.writePerformanceReports = make([]StorePerformanceReport, 0)
	reporter.readPerformanceReports = make([]StorePerformanceReport, 0)
}

func (reporter *DataReporter) ExampleWillRun(exampleSummary *g.ExampleSummary) {
}

func (reporter *DataReporter) ExampleDidComplete(exampleSummary *g.ExampleSummary) {
	for _, measurement := range exampleSummary.Measurements {
		if measurement.Info != nil {
			info := measurement.Info.(StorePerformanceReport)
			info.Average = measurement.Average
			info.StdDeviation = measurement.StdDeviation
			if info.Subject == "write" {
				reporter.writePerformanceReports = append(reporter.writePerformanceReports, info)
			} else if info.Subject == "read" {
				reporter.readPerformanceReports = append(reporter.readPerformanceReports, info)
			}
		}
	}

	reporter.generateCSV(false)
}

func (reporter *DataReporter) SpecSuiteDidEnd(summary *g.SuiteSummary) {
	//todo: collate and print, then use the data reporter in the test suite
	reporter.generateCSV(true)
}

func (reporter *DataReporter) generateCSV(final bool) {
	finalString := ""
	if final {
		finalString = "_final"
	}
	filename := fmt.Sprintf("CSV_%s%s.csv", reporter.timestamp, finalString)
	f, err := os.Create(filename)
	if err != nil {
		panic(err.Error())
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{
		"Store Type",
		"# Store Nodes",
		"# Concurrent Requests",
		"Record Size (bytes)",
		"Num Records Generated",
		"Write Records/s",
		"σ Write Records/s",
		"Write MB/s",
		"σ Write MB/s",
		"Read Records/s",
		"σ Read Records/s",
		"Reads MB/s",
		"σ Reads MB/s",
	})

	for i := 0; i < len(reporter.writePerformanceReports); i++ {
		writeReport := reporter.writePerformanceReports[i]
		readReport := reporter.readPerformanceReports[i]

		wRecordsPerS := float64(writeReport.NumRecords) / writeReport.Average
		wSigmaRecordsPerS := wRecordsPerS * writeReport.StdDeviation / writeReport.Average
		wMbPerS := wRecordsPerS * float64(writeReport.RecordSize) / 1024.0 / 1024.0
		wSigmaMbPerS := wSigmaRecordsPerS * float64(writeReport.RecordSize) / 1024.0 / 1024.0

		rRecordsPerS := float64(readReport.NumRecords) / readReport.Average
		rSigmaRecordsPerS := rRecordsPerS * readReport.StdDeviation / readReport.Average
		rMbPerS := rRecordsPerS * float64(readReport.RecordSize) / 1024.0 / 1024.0
		rSigmaMbPerS := wSigmaRecordsPerS * float64(readReport.RecordSize) / 1024.0 / 1024.0

		w.Write([]string{
			writeReport.StoreType,
			strconv.Itoa(writeReport.NumStoreNodes),
			strconv.Itoa(writeReport.Concurrency),
			strconv.Itoa(writeReport.RecordSize),
			strconv.Itoa(writeReport.NumRecords),
			strconv.FormatFloat(wRecordsPerS, 'f', 3, 64),
			strconv.FormatFloat(wSigmaRecordsPerS, 'f', 3, 64),
			strconv.FormatFloat(wMbPerS, 'f', 3, 64),
			strconv.FormatFloat(wSigmaMbPerS, 'f', 3, 64),
			strconv.FormatFloat(rRecordsPerS, 'f', 3, 64),
			strconv.FormatFloat(rSigmaRecordsPerS, 'f', 3, 64),
			strconv.FormatFloat(rMbPerS, 'f', 3, 64),
			strconv.FormatFloat(rSigmaMbPerS, 'f', 3, 64),
		})
	}

	w.Flush()
}
