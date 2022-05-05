package config

import (
	"time"
)

type Config struct {
	TimeSpanSeconds           int64
	Discount                  float64
	ClusterName               string
	ClusterId                 string
	OutputMode                string
	History                   HistoryAnalyzeConfig
	EnableContainerCheckpoint bool
	EnableWorkloadTimeSeries  bool
	EnableWorkloadCheckpoint  bool
	DataPath                  string
}

type HistoryAnalyzeConfig struct {
	EndTime string
	Length  time.Duration
	Step    time.Duration
}

const (
	OutputModeCsv    = "csv"
	OutputModeStdOut = "stdout"
)
