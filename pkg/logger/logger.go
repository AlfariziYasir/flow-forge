package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	Logger *zap.Logger
}

func New(level string, service string, appVersion string) (*Logger, error) {
	var (
		l      *zap.Logger
		config zap.Config
		err    error
	)

	zapLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		zapLevel = zapcore.InfoLevel
	}

	config = zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	config.Level = zap.NewAtomicLevelAt(zapLevel)

	l, err = config.Build(zap.AddCallerSkip(1))
	if err != nil {
		return nil, err
	}

	l = l.With(
		zap.String("service", service),
		zap.String("version", appVersion),
		zap.Int("pid", os.Getpid()),
	)

	return &Logger{
		Logger: l,
	}, nil

}

func NewNop() *Logger {
	return &Logger{
		Logger: zap.NewNop(),
	}
}

func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.Logger.Info(msg, fields...)
}

func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.Logger.Error(msg, fields...)
}

func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.Logger.Debug(msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.Logger.Warn(msg, fields...)
}

func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.Logger.Fatal(msg, fields...)
}

func (l *Logger) Sync() error {
	if l.Logger != nil {
		return l.Logger.Sync()
	}
	return nil
}

func (l *Logger) GetZapLogger() *zap.Logger {
	return l.Logger
}
