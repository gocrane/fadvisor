package log

import (
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
)

var (
	once   sync.Once
	logger logr.Logger
)

func Logger() logr.Logger {
	once.Do(func() {
		logger = klogr.New()
	})

	return logger
}

func GenerateKey(name string, namespace string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

func GenerateObj(obj klog.KMetadata) string {
	return klog.KObj(obj).String()
}
