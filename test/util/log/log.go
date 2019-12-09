package log

import (
	"github.com/onsi/ginkgo"
	"github.com/sirupsen/logrus"

	utillog "github.com/jim-minter/rp/pkg/util/log"
)

func GetTestLogger() *logrus.Entry {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(ginkgo.GinkgoWriter)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:    true,
		CallerPrettyfier: utillog.RelativeFilePathPrettier,
	})
	logrus.SetReportCaller(true)
	return logrus.NewEntry(logrus.StandardLogger())
}
