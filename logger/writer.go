package logger

import (
	"go.uber.org/zap"
	"k8s.io/klog/v2"
)

type klogToZapWriter struct {
	zapLogger *zap.Logger
}

func (k *klogToZapWriter) Write(p []byte) (n int, err error) {
	k.zapLogger.Info(string(p)) // Assumes all klog output as info level.
	return len(p), nil
}

func initKlogToZap(zapLogger *zap.Logger) {
	klog.SetOutput(&klogToZapWriter{zapLogger: zapLogger})
}
