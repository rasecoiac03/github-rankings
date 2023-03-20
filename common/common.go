package common

import (
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	DefaultLocation      *time.Location
	LocalLayoutTime      string = "2006-01-02"
	LocalLayoutTimestamp string = "2006-01-02 15:04:05"
	NonNumber                   = regexp.MustCompile("[^0-9]+")
)

var config = map[string]string{
	"LOG_LEVEL":   "debug",
	"ENVIRONMENT": "local",
	"GH_TOKEN":    "xxx",
}

func init() {
	// Env vars
	LoadEnvironment()

	// Logging
	logLevel, err := logrus.ParseLevel(GetEnv("LOG_LEVEL"))
	if err != nil {
		logLevel = logrus.DebugLevel
	}
	logrus.SetLevel(logLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyMsg:   "message",
			logrus.FieldKeyLevel: "severity",
		},
	})
	logrus.RegisterExitHandler(func() {
		logrus.Info("application will stop probably due to a OS signal")
	})

	if GetEnv("ENVIRONMENT") == "test" {
		logrus.SetOutput(io.Discard)
	} else {
		logrus.SetOutput(os.Stdout)
	}

	DefaultLocation, _ = time.LoadLocation("America/Recife")
}

func GetEnv(configKey string) string {
	return config[configKey]
}

func GetEnvInt(configKey string) int {
	i := 0
	i, _ = strconv.Atoi(GetEnv(configKey))
	return i
}

func GetEnvFloat(configKey string) float64 {
	i := 0.0
	i, _ = strconv.ParseFloat(GetEnv(configKey), 64)
	return i
}

func GetEnvBool(configKey string) bool {
	v := GetEnv(configKey)
	return strings.ToLower(v) == "true"
}

// LoadEnvironment
// Set default value from config map, else load all setted environment variables.
func LoadEnvironment() {
	for k := range config {
		v := os.Getenv(k)
		if v != "" {
			config[k] = v
		}
	}
}
