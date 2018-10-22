package log

import (
	"time"

	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"github.com/spf13/cast"
)

// update 2018年10月17日14:40:32

// 这个独立在整个系统在，不依赖其他内部库

// Config log 配置
// 如果设置了 LogName，日志文件将使用 LogName + .log
// 如果开启了分割，日志文件 命名规则是 LogName + Layout + .log
// D 如果小于 time.Second 将不使用分割，只有在非 0 的时候讲进行分割,
type Config struct {
	LogPath       string        // 日志路径，如果为空，将使用当前路径
	LogName       string        // 日志文件名，如果为空，将使用 logger
	Level         int8          // 日志等级 zapcore.Level
	OutputFile    bool          // 是否输出文件
	OutputConsole bool          // 是否输出在控制台
	D             time.Duration // 分割的时间段，为低于 ，将不分割，例如 按照小时分割，time.Hour
	Layout        string        // 如果为空，默认将使用 timeLayoutDefault
}

type log struct {
	logger        *zap.Logger
	config        *zap.Config
	logPath       string
	logName       string
	outputFile    bool // 是否输出文件
	outputConsole bool // 是否输出在控制台
	create        time.Time
	d             time.Duration
	layout        string
	callerSkip    int
}

var logger *log

const timeLayoutDefault = "20060102150405"
const fileNameFormat = "%s.%s.log"

// 初始化日志
func Init(c *Config) (err error) {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "unix"
	cfg.Level = zap.NewAtomicLevelAt(zapcore.Level(c.Level))
	cfg.Sampling.Initial = 1000
	cfg.Sampling.Thereafter = 1000

	logger = &log{
		config:        &cfg,
		d:             c.D,
		logPath:       c.LogPath,
		logName:       c.LogName,
		outputFile:    c.OutputFile,
		outputConsole: c.OutputConsole,
		callerSkip:    1,
	}
	if c.OutputFile {
		if c.LogName == "" {
			c.LogName = "logger"
		}

		if c.Layout == "" {
			logger.layout = timeLayoutDefault
		}

		fileName, _ := getLogFileName(logger)
		logger.config.OutputPaths = []string{fmt.Sprintf("%s/%s", logger.logPath, fileName)}
	}

	logger.logger, err = logger.config.Build(zap.AddCallerSkip(logger.callerSkip))
	return err
}

func (l *log) check() (*zap.Logger, error) {
	if !(l.outputFile || l.d < time.Second) {
		return l.logger, nil
	}

	if fileName, ok := getLogFileName(l); ok {
		err := logger.getLogger().Sync()
		if err != nil {
			return nil, err
		}
		logger.config.OutputPaths = []string{fmt.Sprintf("%s/%s", logger.logPath, fileName)}
		logger.logger, err = logger.config.Build(zap.AddCallerSkip(logger.callerSkip))
		return logger.logger, err
	}

	return l.logger, nil
}

func (l *log) getLogger() *zap.Logger {
	logger, err := l.check()
	if err != nil {
		panic(err)
	}
	return logger
}

func Print(msg string, fields ...zap.Field) {
	defer logger.getLogger().Sync()
	logger.getLogger().Info(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	defer logger.getLogger().Sync()
	logger.getLogger().Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	defer logger.logger.Sync()
	logger.logger.Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	defer logger.getLogger().Sync()
	logger.getLogger().Warn(msg, fields...)
	return
}

func Error(msg string, fields ...zap.Field) {
	defer logger.getLogger().Sync()
	logger.getLogger().Error(msg, fields...)
}

func DPanic(msg string, fields ...zap.Field) {
	logger.getLogger().DPanic(msg, fields...)
}

func Panic(msg string, fields ...zap.Field) {
	logger.getLogger().Panic(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	logger.getLogger().Fatal(msg, fields...)
}

func Sync() error {
	return logger.getLogger().Sync()
}

func timeZero(t time.Time, data ...int) time.Time {
	switch len(data) {
	case 0:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case 1:
		return time.Date(t.Year(), t.Month(), t.Day(), data[0], 0, 0, 0, t.Location())
	case 2:
		return time.Date(t.Year(), t.Month(), t.Day(), data[0], data[1], 0, 0, t.Location())
	case 3:
		return time.Date(t.Year(), t.Month(), t.Day(), data[0], data[1], data[2], 0, t.Location())
	case 4:
		return time.Date(t.Year(), t.Month(), t.Day(), data[0], data[1], data[2], data[3], t.Location())
	default:
		return t
	}
}

func getLogFileName(l *log) (string, bool) {
	if l.d < time.Second {
		if l.create.IsZero() {
			l.create = time.Now().Local()
		}
		return fmt.Sprintf(fileNameFormat, l.logName, timeZero(l.create).Format(l.layout)), false
	}

	var h, m, s int
	dString := l.d.String()

	upIndex := 0
	for i := 0; i < len(dString); i++ {
		switch c := int(dString[i]); c {
		case 'h':
			h = cast.ToInt(dString[upIndex:i])
			upIndex = i
		case 'm':
			m = cast.ToInt(dString[upIndex:i])
			upIndex = i
		case 's':
			s = cast.ToInt(dString[upIndex:i])
			upIndex = i
		}
	}

	now := time.Now().Local()
	if h > 0 {
		if int(h)%24 == 0 {
			now = timeZero(now)
		} else {
			now = timeZero(now, now.Day(), now.Hour())
		}
	}
	if m > 0 {
		now = timeZero(now, now.Hour(), now.Minute())
	}
	if s > 0 {
		now = timeZero(now, now.Hour(), now.Minute(), now.Second())
	}

	if now.Unix()-l.create.Unix() >= l.d.Nanoseconds()/time.Second.Nanoseconds() {
		l.create = now
		return fmt.Sprintf(fileNameFormat, l.logName, l.create.Format(l.layout)), true
	}

	return fmt.Sprintf(fileNameFormat, l.logName, l.create.Format(l.layout)), false
}
