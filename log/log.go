package vlog

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	loggerInstance Logger
	once           sync.Once
	logDir         = flag.String("log_dir", ".", "If non-empty, write log files in this directory")
	logAsync       = flag.Bool("log_async", true, "If false, write log sync, default is true")
)

const (
	defaultLogMaxSize    = 500 //megabytes
	defaultLogMaxAge     = 30  //days
	defaultLogMaxBackups = 20
	defaultAsyncLogLen   = 1000

	defaultLogSuffix      = ".log"
	defaultAccessLogName  = "access"
	defaultMetricsLogName = "metrics"
	defaultLogLevel       = InfoLevel
	defaultFlushInterval  = time.Second
)

const (
	TraceLevel LogLevel = iota - 2
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	DPanicLevel
	PanicLevel
	FatalLevel
)

type LogLevel int8

func (l LogLevel) String() string {
	switch l {
	case TraceLevel:
		return "trace"
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	case DPanicLevel:
		return "dpanic"
	case PanicLevel:
		return "panic"
	case FatalLevel:
		return "fatal"
	default:
		return fmt.Sprintf("Level(%d)", l)
	}
}

func (l *LogLevel) Set(level string) error {
	if l == nil {
		return errors.New("level is nil")
	}
	switch strings.ToLower(level) {
	case "":
		*l = defaultLogLevel
	case "trace":
		*l = TraceLevel
	case "debug":
		*l = DebugLevel
	case "info":
		*l = InfoLevel
	case "warn":
		*l = WarnLevel
	case "error":
		*l = ErrorLevel
	case "dpanic":
		*l = DPanicLevel
	case "panic":
		*l = PanicLevel
	case "fatal":
		*l = FatalLevel
	default:
		return errors.New("unrecognized level:" + string(level))
	}
	return nil
}

type AccessLogEntity struct {
	Msg           string `json:"msg"`
	Role          string `json:"role"`
	RequestID     uint64 `json:"requestID"`
	Service       string `json:"service"`
	Method        string `json:"method"`
	RemoteAddress string `json:"remoteAddress"`
	Desc          string `json:"desc"`
	ReqSize       int    `json:"reqSize"`
	ResSize       int    `json:"resSize"`
	BizTime       int64  `json:"bizTime"`
	TotalTime     int64  `json:"totalTime"`
	Success       bool   `json:"success"`
	Exception     string `json:"exception"`
}

type Logger interface {
	Infoln(...interface{})
	Infof(string, ...interface{})
	Warningln(...interface{})
	Warningf(string, ...interface{})
	Errorln(...interface{})
	Errorf(string, ...interface{})
	Fatalln(...interface{})
	Fatalf(string, ...interface{})
	AccessLog(AccessLogEntity)
	MetricsLog(string)
	Flush()
	SetAsync(bool)
	GetLevel() LogLevel
	SetLevel(LogLevel)
	GetAccessLogAvailable() bool
	SetAccessLogAvailable(bool)
	GetMetricsLogAvailable() bool
	SetMetricsLogAvailable(bool)
}

func LogInit(logger Logger) {
	once.Do(func() {
		if logger == nil {
			loggerInstance = newDefaultLog(*logDir)
		} else {
			loggerInstance = logger
		}
		SetAsync(*logAsync)
		startFlush()
	})
}

func Infoln(args ...interface{}) {
	if loggerInstance != nil {
		loggerInstance.Infoln(args...)
	} else {
		log.Println(args...)
	}
}

func Infof(format string, args ...interface{}) {
	if loggerInstance != nil {
		loggerInstance.Infof(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

func Warningln(args ...interface{}) {
	if loggerInstance != nil {
		loggerInstance.Warningln(args...)
	} else {
		log.Println(args...)
	}
}

func Warningf(format string, args ...interface{}) {
	if loggerInstance != nil {
		loggerInstance.Warningf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

func Errorln(args ...interface{}) {
	if loggerInstance != nil {
		loggerInstance.Errorln(args...)
	} else {
		log.Println(args...)
	}
}

func Errorf(format string, args ...interface{}) {
	if loggerInstance != nil {
		loggerInstance.Errorf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

func Fatalln(args ...interface{}) {
	if loggerInstance != nil {
		loggerInstance.Fatalln(args...)
	} else {
		log.Println(args...)
	}
}

func Fatalf(format string, args ...interface{}) {
	if loggerInstance != nil {
		loggerInstance.Fatalf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

func AccessLog(logType AccessLogEntity) {
	if loggerInstance != nil {
		loggerInstance.AccessLog(logType)
	}
}

func MetricsLog(msg string) {
	if loggerInstance != nil {
		loggerInstance.MetricsLog(msg)
	}
}

func startFlush() {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("flush logs failed. err:%v", err)
				Warningf("flush logs failed. err:%v", err)
			}
		}()
		ticker := time.NewTicker(defaultFlushInterval)
		defer ticker.Stop()
		if loggerInstance != nil {
			for range ticker.C {
				loggerInstance.Flush()
			}
		}
	}()
}

func SetAsync(value bool) {
	if loggerInstance != nil {
		loggerInstance.SetAsync(value)
	}
}

func GetLevel() LogLevel {
	if loggerInstance != nil {
		return loggerInstance.GetLevel()
	}
	return defaultLogLevel
}

func SetLevel(level LogLevel) {
	if loggerInstance != nil {
		loggerInstance.SetLevel(level)
	}
}

func GetAccessLogAvailable() bool {
	if loggerInstance != nil {
		return loggerInstance.GetAccessLogAvailable()
	}
	return false
}

func SetAccessLogAvailable(status bool) {
	if loggerInstance != nil {
		loggerInstance.SetAccessLogAvailable(status)
	}
}

func GetMetricsLogAvailable() bool {
	if loggerInstance != nil {
		return loggerInstance.GetMetricsLogAvailable()
	}
	return false
}

func SetMetricsLogAvailable(status bool) {
	if loggerInstance != nil {
		loggerInstance.SetMetricsLogAvailable(status)
	}
}

func newDefaultLog(logDir string) Logger {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:      "time",
		LevelKey:     "level",
		CallerKey:    "caller",
		MessageKey:   "message",
		EncodeLevel:  zapcore.CapitalLevelEncoder,
		EncodeTime:   zapcore.ISO8601TimeEncoder,
		EncodeCaller: zapcore.ShortCallerEncoder,
	}
	level := zap.NewAtomicLevel()
	zapCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(newRotateHook(logDir, filepath.Base(os.Args[0])+defaultLogSuffix))),
		level,
	)
	logger := zap.New(zapCore, zap.AddCaller(), zap.AddCallerSkip(2))

	accessLogger, accessLevel := newZapLogger(logDir, defaultAccessLogName+defaultLogSuffix)
	metricsLogger, metricsLevel := newZapLogger(logDir, defaultMetricsLogName+defaultLogSuffix)
	return &defaultLogger{
		logger:        logger.Sugar(),
		loggerLevel:   level,
		accessLogger:  accessLogger,
		accessLevel:   accessLevel,
		metricsLogger: metricsLogger,
		metricsLevel:  metricsLevel,
	}
}

func newRotateHook(logDir, logName string) *lumberjack.Logger {
	return &lumberjack.Logger{
		LocalTime:  true,
		MaxAge:     defaultLogMaxAge,
		MaxSize:    defaultLogMaxSize,
		MaxBackups: defaultLogMaxBackups,
		Filename:   filepath.Join(logDir, logName),
	}
}

func newZapLogger(logDir, logName string) (*zap.Logger, zap.AtomicLevel) {
	zapEncoderConfig := zapcore.EncoderConfig{
		TimeKey:    "time",
		MessageKey: "message",
		EncodeTime: zapcore.ISO8601TimeEncoder,
	}
	zapLevel := zap.NewAtomicLevel()
	zapCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zapEncoderConfig),
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(newRotateHook(logDir, logName))),
		zapLevel,
	)
	zapLogger := zap.New(zapCore)
	return zapLogger, zapLevel
}

type defaultLogger struct {
	async         bool
	outputChan    chan AccessLogEntity
	logger        *zap.SugaredLogger
	accessLogger  *zap.Logger
	metricsLogger *zap.Logger
	loggerLevel   zap.AtomicLevel
	accessLevel   zap.AtomicLevel
	metricsLevel  zap.AtomicLevel
}

func (d *defaultLogger) Infoln(fields ...interface{}) {
	d.logger.Info(fields...)
}

func (d *defaultLogger) Infof(format string, fields ...interface{}) {
	d.logger.Infof(format, fields...)
}

func (d *defaultLogger) Warningln(fields ...interface{}) {
	d.logger.Warn(fields...)
}

func (d *defaultLogger) Warningf(format string, fields ...interface{}) {
	d.logger.Warnf(format, fields...)
}

func (d *defaultLogger) Errorln(fields ...interface{}) {
	d.logger.Error(fields...)
}

func (d *defaultLogger) Errorf(format string, fields ...interface{}) {
	d.logger.Errorf(format, fields...)
}

func (d *defaultLogger) Fatalln(fields ...interface{}) {
	d.logger.Fatal(fields...)
}

func (d *defaultLogger) Fatalf(format string, fields ...interface{}) {
	d.logger.Fatalf(format, fields...)
}

func (d *defaultLogger) AccessLog(logObject AccessLogEntity) {
	if d.async {
		d.outputChan <- logObject
	} else {
		d.doAccessLog(logObject)
	}
}

func (d *defaultLogger) doAccessLog(logObject AccessLogEntity) {
	d.accessLogger.Info(logObject.Msg,
		zap.String("role", logObject.Role),
		zap.Uint64("requestID", logObject.RequestID),
		zap.String("service", logObject.Service),
		zap.String("method", logObject.Method),
		zap.String("remoteAddress", logObject.RemoteAddress),
		zap.String("desc", logObject.Desc),
		zap.Int("reqSize", logObject.ReqSize),
		zap.Int("resSize", logObject.ResSize),
		zap.Int64("bizTime", logObject.BizTime),
		zap.Int64("totalTime", logObject.TotalTime),
		zap.Bool("success", logObject.Success),
		zap.String("exception", logObject.Exception))
}

func (d *defaultLogger) MetricsLog(msg string) {
	d.metricsLogger.Info(msg)
}

func (d *defaultLogger) Flush() {
	_ = d.logger.Sync()
	_ = d.accessLogger.Sync()
	_ = d.metricsLogger.Sync()
}

func (d *defaultLogger) SetAsync(value bool) {
	d.async = value
	if value {
		d.outputChan = make(chan AccessLogEntity, defaultAsyncLogLen)
		go func() {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("logs async output failed. err:%v", err)
					Warningf("logs async output failed. err:%v", err)
				}
			}()
			for {
				select {
				case l := <-d.outputChan:
					d.doAccessLog(l)
				}
			}
		}()
	}
}

func (d *defaultLogger) GetLevel() LogLevel {
	return LogLevel(d.loggerLevel.Level())
}

func (d *defaultLogger) SetLevel(level LogLevel) {
	d.loggerLevel.SetLevel(zapcore.Level(level))
}

func (d *defaultLogger) GetAccessLogAvailable() bool {
	return d.accessLevel.Level() <= zapcore.Level(defaultLogLevel)
}

func (d *defaultLogger) SetAccessLogAvailable(status bool) {
	if status {
		d.accessLevel.SetLevel(zapcore.Level(defaultLogLevel))
	} else {
		d.accessLevel.SetLevel(zapcore.Level(defaultLogLevel + 1))
	}
}

func (d *defaultLogger) GetMetricsLogAvailable() bool {
	return d.metricsLevel.Level() <= zapcore.Level(defaultLogLevel)
}

func (d *defaultLogger) SetMetricsLogAvailable(status bool) {
	if status {
		d.metricsLevel.SetLevel(zapcore.Level(defaultLogLevel))
	} else {
		d.metricsLevel.SetLevel(zapcore.Level(defaultLogLevel + 1))
	}
}