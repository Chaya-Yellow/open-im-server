package log

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/OpenIMSDK/Open-IM-Server/pkg/common/config"
	"github.com/OpenIMSDK/Open-IM-Server/pkg/common/constant"
	"github.com/OpenIMSDK/Open-IM-Server/pkg/common/mcontext"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	pkgLogger   Logger = &ZapLogger{}
	sp                 = string(filepath.Separator)
	logLevelMap        = map[int]zapcore.Level{
		6: zapcore.DebugLevel,
		5: zapcore.DebugLevel,
		4: zapcore.InfoLevel,
		3: zapcore.WarnLevel,
		2: zapcore.ErrorLevel,
		1: zapcore.FatalLevel,
		0: zapcore.PanicLevel,
	}
)

// InitFromConfig initializes a Zap-based logger
func InitFromConfig(name string, logLevel int) error {
	l, err := NewZapLogger(logLevel)
	if err != nil {
		return err
	}
	pkgLogger = l.WithCallDepth(2).WithName(name)
	return nil
}

func ZDebug(ctx context.Context, msg string, keysAndValues ...interface{}) {
	pkgLogger.Debug(ctx, msg, keysAndValues...)
}

func ZInfo(ctx context.Context, msg string, keysAndValues ...interface{}) {
	pkgLogger.Info(ctx, msg, keysAndValues...)
}

func ZWarn(ctx context.Context, msg string, err error, keysAndValues ...interface{}) {
	pkgLogger.Warn(ctx, msg, err, keysAndValues...)
}

func ZError(ctx context.Context, msg string, err error, keysAndValues ...interface{}) {
	pkgLogger.Error(ctx, msg, err, keysAndValues...)
}

type ZapLogger struct {
	zap *zap.SugaredLogger
}

func NewZapLogger(logLevel int) (*ZapLogger, error) {
	zapConfig := zap.Config{
		Level:             zap.NewAtomicLevelAt(logLevelMap[logLevel]),
		Encoding:          "json",
		EncoderConfig:     zap.NewProductionEncoderConfig(),
		InitialFields:     map[string]interface{}{"PID": os.Getegid()},
		DisableStacktrace: true,
	}
	if config.Config.Log.Stderr {
		zapConfig.OutputPaths = append(zapConfig.OutputPaths, "stderr")
	}
	zl := &ZapLogger{}
	opts, err := zl.cores()
	if err != nil {
		return nil, err
	}
	l, err := zapConfig.Build(opts)
	if err != nil {
		return nil, err
	}
	zl.zap = l.Sugar()
	return zl, nil
}

func (l *ZapLogger) cores() (zap.Option, error) {
	c := zap.NewProductionEncoderConfig()
	c.EncodeTime = zapcore.ISO8601TimeEncoder
	c.EncodeDuration = zapcore.SecondsDurationEncoder
	c.EncodeLevel = zapcore.CapitalLevelEncoder
	c.MessageKey = "msg"
	c.LevelKey = "level"
	c.TimeKey = "time"
	c.CallerKey = "caller"
	fileEncoder := zapcore.NewJSONEncoder(c)
	fileEncoder.AddInt("PID", os.Getpid())
	writer, err := l.getWriter()
	if err != nil {
		return nil, err
	}
	var cores []zapcore.Core
	if config.Config.Log.StorageLocation != "" {
		cores = []zapcore.Core{
			zapcore.NewCore(fileEncoder, writer, zap.NewAtomicLevelAt(zapcore.Level(config.Config.Log.RemainLogLevel))),
		}
	}
	if config.Config.Log.Stderr {
		cores = append(cores, zapcore.NewCore(fileEncoder, zapcore.Lock(os.Stdout), zap.NewAtomicLevelAt(zapcore.Level(config.Config.Log.RemainLogLevel))))
	}
	return zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(cores...)
	}), nil
}

func (l *ZapLogger) getWriter() (zapcore.WriteSyncer, error) {
	logf, err := rotatelogs.New(config.Config.Log.StorageLocation+sp+"OpenIM.log.all"+".%Y-%m-%d",
		rotatelogs.WithRotationCount(config.Config.Log.RemainRotationCount),
		rotatelogs.WithRotationTime(time.Duration(config.Config.Log.RotationTime)*time.Hour),
	)
	if err != nil {
		return nil, err
	}
	return zapcore.AddSync(logf), nil
}

func (l *ZapLogger) ToZap() *zap.SugaredLogger {
	return l.zap
}

func (l *ZapLogger) Debug(ctx context.Context, msg string, keysAndValues ...interface{}) {
	keysAndValues = l.kvAppend(ctx, keysAndValues)
	l.zap.Debugw(msg, keysAndValues...)
}

func (l *ZapLogger) Info(ctx context.Context, msg string, keysAndValues ...interface{}) {
	keysAndValues = l.kvAppend(ctx, keysAndValues)
	l.zap.Infow(msg, keysAndValues...)
}

func (l *ZapLogger) Warn(ctx context.Context, msg string, err error, keysAndValues ...interface{}) {
	if err != nil {
		keysAndValues = append(keysAndValues, "error", err.Error())
	}
	keysAndValues = l.kvAppend(ctx, keysAndValues)
	l.zap.Warnw(msg, keysAndValues...)
}

func (l *ZapLogger) Error(ctx context.Context, msg string, err error, keysAndValues ...interface{}) {
	if err != nil {
		keysAndValues = append(keysAndValues, "error", err.Error())
	}
	keysAndValues = append([]interface{}{constant.OperationID, mcontext.GetOperationID(ctx)}, keysAndValues...)
	l.zap.Errorw(msg, keysAndValues...)
}

func (l *ZapLogger) kvAppend(ctx context.Context, keysAndValues []interface{}) []interface{} {
	operationID := mcontext.GetOperationID(ctx)
	opUserID := mcontext.GetOpUserID(ctx)
	connID := mcontext.GetConnID(ctx)
	triggerID := mcontext.GetTriggerID(ctx)
	opUserPlatform := mcontext.GetOpUserPlatform(ctx)
	remoteAddr := mcontext.GetRemoteAddr(ctx)
	if opUserID != "" {
		keysAndValues = append([]interface{}{constant.OpUserID, opUserID}, keysAndValues...)
	}
	if operationID != "" {
		keysAndValues = append([]interface{}{constant.OperationID, operationID}, keysAndValues...)
	}
	if connID != "" {
		keysAndValues = append([]interface{}{constant.ConnID, connID}, keysAndValues...)
	}
	if triggerID != "" {
		keysAndValues = append([]interface{}{constant.TriggerID, triggerID}, keysAndValues...)
	}
	if opUserPlatform != "" {
		keysAndValues = append([]interface{}{constant.OpUserPlatform, opUserPlatform}, keysAndValues...)
	}
	if remoteAddr != "" {
		keysAndValues = append([]interface{}{constant.RemoteAddr, remoteAddr}, keysAndValues...)
	}
	return keysAndValues
}

func (l *ZapLogger) WithValues(keysAndValues ...interface{}) Logger {
	dup := *l
	dup.zap = l.zap.With(keysAndValues...)
	return &dup
}

func (l *ZapLogger) WithName(name string) Logger {
	dup := *l
	dup.zap = l.zap.Named(name)
	return &dup
}

func (l *ZapLogger) WithCallDepth(depth int) Logger {
	dup := *l
	dup.zap = l.zap.WithOptions(zap.AddCallerSkip(depth))
	return &dup
}
