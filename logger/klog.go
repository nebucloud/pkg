package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"sync"
	"sync/atomic"

	slogzap "github.com/samber/slog-zap"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Level is a shim
type Level int32

// Verbose is a shim
type Verbose bool

// Config is the mixture of zap config and klog config
type Config struct {
	// zap config
	zapConfig zap.Config
	level     Level

	// klog config
	v               int32
	alsologtostderr bool
}

// Klogger wraps a slog logger
type Klogger struct {
	logger *slog.Logger
	config Config
}

const (
	// MinLevel 0: default level, forbids DEBUG log
	MinLevel Level = iota
	_
	_
	_
	// MaxLevel 4: max level V(4)
	MaxLevel
)

var (
	klogger *Klogger
	once    sync.Once
)

func init() {
	zapLogger, _ := zap.NewProduction()
	logger := slog.New(slogzap.Option{Level: slog.LevelDebug, Logger: zapLogger}.NewZapHandler())
	klogger = &Klogger{
		logger: logger,
		config: Config{
			level:           0,
			v:               0,
			alsologtostderr: true,
		},
	}
}

// Singleton returns a singleton instance of Klogger.
// It initializes the logger lazily and ensures thread safety.
// It returns a pointer to Klogger.
func Singleton() *Klogger {
	once.Do(func() {
		klogger.config.level = Level(klogger.config.v)
		if l := klogger.config.level; l < MinLevel || l > MaxLevel {
			panic(fmt.Errorf("FATAL: 'v' must be in the range [0, 4]"))
		}

		klogger.config.zapConfig = zap.NewProductionConfig()

		// change time from ns to formatted
		klogger.config.zapConfig.EncoderConfig.TimeKey = "time"
		klogger.config.zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		// always set to debug level
		klogger.config.zapConfig.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)

		// due to gaps between zap and klog
		if !klogger.config.alsologtostderr {
			klogger.config.zapConfig.OutputPaths = []string{"stdout"}
		}

		// trace the real source caller due to manual inline is not supported
		zapLogger, err := klogger.config.zapConfig.Build(zap.AddCallerSkip(1))
		if err != nil {
			panic(err)
		}
		logger := slog.New(slogzap.Option{Level: slog.LevelDebug, Logger: zapLogger}.NewZapHandler())
		klogger.logger = logger
		Infof("Initialized zap logger...")
	})
	return klogger
}

// InitFlags is a shim, only accepts
func InitFlags(flagset *pflag.FlagSet) {
	if flagset == nil {
		flagset = pflag.CommandLine
	}
	flagset.Int32Var(&klogger.config.v, "v", klogger.config.v, "verbosity of info log")
	flagset.BoolVar(&klogger.config.alsologtostderr, "alsologtostderr", klogger.config.alsologtostderr, "also write logs to stderr, default to true")
}

// Flush is a shim
func Flush() {
	// No-op, as slog doesn't have a Flush method
}

// SetLevel updates level on the fly
func SetLevel(v Level) {
	klogger.SetLevel(v)
}

// SetLevel updates level on the fly
func (k *Klogger) SetLevel(v Level) {
	if v < MinLevel || v > MaxLevel {
		k.Warningf("failed setting level: expect [0, 4], get %d", v)
		return
	}
	if k.config.level.get() != v {
		k.config.level.set(v)
	}
}

// Set sets the value of the Level.
func (l *Level) set(val Level) {
	atomic.StoreInt32((*int32)(l), int32(val))
}

// get returns the value of the Level.
func (l *Level) get() Level {
	return Level(atomic.LoadInt32((*int32)(l)))
}

// V is a shim
func V(level Level) Verbose {
	return Verbose(level <= klogger.config.level.get())
}

// V is a shim
func (k *Klogger) V(level Level) Verbose {
	return Verbose(level <= k.config.level.get())
}

// Info is a shim
//
//go:noinline
func (v Verbose) Info(args ...interface{}) {
	if v {
		klogger.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprint(args...))
	}
}

// Infoln is a shim
//
//go:noinline
func (v Verbose) Infoln(args ...interface{}) {
	if v {
		klogger.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprint(args...))
	}
}

// Infof is a shim
//
//go:noinline
func (v Verbose) Infof(format string, args ...interface{}) {
	if v {
		klogger.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprintf(format, args...))
	}
}

// Info is a shim
//
//go:noinline
func Info(args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprint(args...))
}

// Info is a shim
//
//go:noinline
func (k *Klogger) Info(args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprint(args...))
}

// InfoDepth is a shim
//
//go:noinline
func InfoDepth(depth int, args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprint(args...))
}

// InfoDepth is a shim
//
//go:noinline
func (k *Klogger) InfoDepth(depth int, args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprint(args...))
}

// Infoln is a shim
//
//go:noinline
func Infoln(args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprint(args...))
}

// Infoln is a shim
//
//go:noinline
func (k *Klogger) Infoln(args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprint(args...))
}

// Infof is a shim
//
//go:noinline
func Infof(format string, args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprintf(format, args...))
}

// Infof is a shim
//
//go:noinline
func (k *Klogger) Infof(format string, args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprintf(format, args...))
}

// InfoS is a shim for structured logging
//
//go:noinline
func InfoS(msg string, keysAndValues ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelInfo, msg, slog.Group("", keysAndValues...))
}

// InfoS is a shim for structured logging
//
//go:noinline
func (k *Klogger) InfoS(msg string, keysAndValues ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelInfo, msg, slog.Group("", keysAndValues...))
}

// Warning is a shim
//
//go:noinline
func Warning(args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelWarn, fmt.Sprint(args...))
}

// Warning is a shim
//
//go:noinline
func (k *Klogger) Warning(args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelWarn, fmt.Sprint(args...))
}

// WarningDepth is a shim
//
//go:noinline
func WarningDepth(depth int, args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelWarn, fmt.Sprint(args...))
}

// WarningDepth is a shim
//
//go:noinline
func (k *Klogger) WarningDepth(depth int, args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelWarn, fmt.Sprint(args...))
}

// Warningln is a shim
//
//go:noinline
func Warningln(args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelWarn, fmt.Sprint(args...))
}

// Warningln is a shim
//
//go:noinline
func (k *Klogger) Warningln(args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelWarn, fmt.Sprint(args...))
}

// Warningf is a shim
//
//go:noinline
func Warningf(format string, args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelWarn, fmt.Sprintf(format, args...))
}

// Warningf is a shim
//
//go:noinline
func (k *Klogger) Warningf(format string, args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelWarn, fmt.Sprintf(format, args...))
}

// Error is a shim
//
//go:noinline
func Error(args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelError, fmt.Sprint(args...))
}

// Error is a shim
//
//go:noinline
func (k *Klogger) Error(args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelError, fmt.Sprint(args...))
}

// ErrorDepth is a shim
//
//go:noinline
func ErrorDepth(depth int, args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelError, fmt.Sprint(args...))
}

// ErrorDepth is a shim
//
//go:noinline
func (k *Klogger) ErrorDepth(depth int, args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelError, fmt.Sprint(args...))
}

// Errorln is a shim
//
//go:noinline
func Errorln(args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelError, fmt.Sprint(args...))
}

// Errorln is a shim
//
//go:noinline
func (k *Klogger) Errorln(args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelError, fmt.Sprint(args...))
}

// Errorf is a shim
//
//go:noinline
func Errorf(format string, args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelError, fmt.Sprintf(format, args...))
}

// Errorf is a shim
//
//go:noinline
func (k *Klogger) Errorf(format string, args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelError, fmt.Sprintf(format, args...))
}

// Fatal is a shim
//
//go:noinline
func Fatal(args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelError, fmt.Sprint(args...))
	os.Exit(255)
}

// Fatal is a shim
//
//go:noinline
func (k *Klogger) Fatal(args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelError, fmt.Sprint(args...))
	os.Exit(255)
}

// FatalDepth is a shim
//
//go:noinline
func FatalDepth(depth int, args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelError, fmt.Sprint(args...))
	os.Exit(255)
}

// FatalDepth is a shim
//
//go:noinline
func (k *Klogger) FatalDepth(depth int, args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelError, fmt.Sprint(args...))
	os.Exit(255)
}

// Fatalln is a shim
//
//go:noinline
func Fatalln(args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelError, fmt.Sprint(args...))
	os.Exit(255)
}

// Fatalln is a shim
//
//go:noinline
func (k *Klogger) Fatalln(args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelError, fmt.Sprint(args...))
	os.Exit(255)
}

// Fatalf is a shim
//
//go:noinline
func Fatalf(format string, args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelError, fmt.Sprintf(format, args...))
	os.Exit(255)
}

// Fatalf is a shim
//
//go:noinline
func (k *Klogger) Fatalf(format string, args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelError, fmt.Sprintf(format, args...))
	os.Exit(255)
}

// Exit is a shim
//
//go:noinline
func Exit(args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprint(args...))
	os.Exit(1)
}

// Exit is a shim
//
//go:noinline
func (k *Klogger) Exit(args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprint(args...))
	os.Exit(1)
}

// ExitDepth is a shim
//
//go:noinline
func ExitDepth(depth int, args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprint(args...))
	os.Exit(1)
}

// ExitDepth is a shim
//
//go:noinline
func (k *Klogger) ExitDepth(depth int, args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprint(args...))
	os.Exit(1)
}

// Exitln is a shim
//
//go:noinline
func Exitln(args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprint(args...))
	os.Exit(1)
}

// Exitln is a shim
//
//go:noinline
func (k *Klogger) Exitln(args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprint(args...))
	os.Exit(1)
}

// Exitf is a shim
//
//go:noinline
func Exitf(format string, args ...interface{}) {
	klogger.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprintf(format, args...))
	os.Exit(1)
}

// Exitf is a shim
//
//go:noinline
func (k *Klogger) Exitf(format string, args ...interface{}) {
	k.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprintf(format, args...))
	os.Exit(1)
}

// With adds structured context to the logger.
func With(args ...interface{}) *Klogger {
	return klogger.With(args...)
}

// With adds structured context to the logger.
func (k *Klogger) With(args ...interface{}) *Klogger {
	newLogger := k.logger
	if len(args) > 0 {
		newLogger = newLogger.With(slog.Group("", args...))
	}
	return &Klogger{
		logger: newLogger,
		config: k.config,
	}
}

// WithFields adds structured context to the logger.
func WithFields(args map[string]interface{}) *Klogger {
	return klogger.WithFields(args)
}

// WithFields adds structured context to the logger.
func (k *Klogger) WithFields(fields map[string]interface{}) *Klogger {
	newLogger := k.logger
	if len(fields) > 0 {
		newLogger = newLogger.With(slog.Group("", slog.Any("fields", fields)))
	}
	return &Klogger{
		logger: newLogger,
		config: k.config,
	}
}

// WithAll fills each arg directly without parsing fields and values.
// Only valid for exported fields.
func WithAll(args ...interface{}) *Klogger {
	return klogger.WithAll(args...)
}

// WithAll fills each arg directly without parsing fields and values.
// Only valid for exported fields.
func (k *Klogger) WithAll(args ...interface{}) *Klogger {
	newLogger := k.logger
	for _, arg := range args {
		t := reflect.TypeOf(arg)
		v := reflect.ValueOf(arg)
		fields := make([]interface{}, 0, t.NumField()*2)
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.IsExported() {
				fields = append(fields, field.Name, v.Field(i).Interface())
			}
		}
		newLogger = newLogger.With(slog.Group("", fields...))
	}
	return &Klogger{
		logger: newLogger,
		config: k.config,
	}
}
