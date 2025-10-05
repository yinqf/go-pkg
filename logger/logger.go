package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	logDir            = "logs"
	logRetention      = 7 * 24 * time.Hour
	logMaxSize        = 100 * 1024 * 1024 // 100 MB
	logDateLayout     = "2006-01-02"
	consoleTimeLayout = "2006-01-02 15:04:05,000"
)

var (
	once        sync.Once
	infoLogger  *zap.Logger
	debugLogger *zap.Logger
	errorLogger *zap.Logger
)

func ensureLoggers() {
	once.Do(func() {
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			panic("create log directory: " + err.Error())
		}

		infoLogger = newLevelLogger("info", zapcore.InfoLevel)
		debugLogger = newLevelLogger("debug", zapcore.DebugLevel)
		errorLogger = newLevelLogger("error", zapcore.ErrorLevel)
	})
}

func newLevelLogger(levelName string, level zapcore.Level) *zap.Logger {
	writer := newRotatingWriter(levelName)

	levelFilter := zap.LevelEnablerFunc(func(l zapcore.Level) bool { return l == level })
	fileEncoder := zapcore.NewConsoleEncoder(newHumanEncoderConfig())
	fileCore := zapcore.NewCore(
		fileEncoder,
		zapcore.AddSync(writer),
		levelFilter,
	)

	consoleEncoder := zapcore.NewConsoleEncoder(newHumanEncoderConfig())
	consoleCore := zapcore.NewCore(
		consoleEncoder,
		zapcore.Lock(os.Stdout),
		levelFilter,
	)

	tee := zapcore.NewTee(fileCore, consoleCore)
	return zap.New(tee, zap.AddCaller(), zap.AddCallerSkip(1))
}

type rotatingWriter struct {
	mu           sync.Mutex
	level        string
	currentDate  string
	currentIndex int
	currentSize  int64
	file         *os.File
	nextRotation time.Time
}

func newRotatingWriter(level string) *rotatingWriter {
	return &rotatingWriter{level: level}
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	now := time.Now()
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.ensureFile(now); err != nil {
		return 0, err
	}

	if w.currentSize+int64(len(p)) > logMaxSize {
		if err := w.rotate(now); err != nil {
			return 0, err
		}
	}

	n, err := w.file.Write(p)
	w.currentSize += int64(n)
	return n, err
}

func (w *rotatingWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}

	return w.file.Sync()
}

func (w *rotatingWriter) ensureFile(now time.Time) error {
	if w.file == nil {
		return w.openFile(now.Format(logDateLayout), 0, now)
	}

	if now.After(w.nextRotation) {
		return w.openFile(now.Format(logDateLayout), 0, now)
	}

	return nil
}

func (w *rotatingWriter) rotate(now time.Time) error {
	return w.openFile(w.currentDate, w.currentIndex+1, now)
}

func (w *rotatingWriter) openFile(date string, index int, now time.Time) error {
	if w.file != nil {
		_ = w.file.Close()
	}

	filename := w.buildFilename(date, index)
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}

	w.file = file
	w.currentDate = date
	w.currentIndex = index
	w.currentSize = info.Size()
	w.nextRotation = startOfNextDay(now)

	if index == 0 {
		w.scheduleCleanup()
	}
	return nil
}

func (w *rotatingWriter) buildFilename(date string, index int) string {
	name := fmt.Sprintf("%s-%s", w.level, date)
	if index > 0 {
		name = fmt.Sprintf("%s-%02d", name, index)
	}
	return filepath.Join(logDir, name+".log")
}

func startOfNextDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location()).Add(24 * time.Hour)
}

func newHumanEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:          "ts",
		LevelKey:         "level",
		NameKey:          zapcore.OmitKey,
		CallerKey:        "caller",
		MessageKey:       "msg",
		StacktraceKey:    zapcore.OmitKey,
		LineEnding:       zapcore.DefaultLineEnding,
		EncodeLevel:      consoleLevelEncoder,
		EncodeTime:       consoleTimeEncoder,
		EncodeDuration:   zapcore.StringDurationEncoder,
		EncodeCaller:     consoleCallerEncoder,
		EncodeName:       zapcore.FullNameEncoder,
		ConsoleSeparator: " ",
	}
}

func (w *rotatingWriter) scheduleCleanup() {
	cutoff := time.Now().Add(-logRetention)
	level := w.level
	go func() {
		entries, err := os.ReadDir(logDir)
		if err != nil {
			Error("扫描日志目录失败", zap.Error(err))
			return
		}

		prefix := level + "-"
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}

			if info.ModTime().Before(cutoff) {
				path := filepath.Join(logDir, name)
				if err := os.Remove(path); err != nil {
					Error("删除过期日志失败", zap.String("path", path), zap.Error(err))
				}
			}
		}
	}()
}

func consoleTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format(consoleTimeLayout))
}

func consoleLevelEncoder(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(strings.ToUpper(level.String()))
}

func consoleCallerEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	if !caller.Defined {
		enc.AppendString("()-")
		return
	}

	enc.AppendString(fmt.Sprintf("(%s:%d)-", filepath.Base(caller.File), caller.Line))
}

func Info(msg string, fields ...zap.Field) {
	ensureLoggers()
	infoLogger.Info(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	ensureLoggers()
	debugLogger.Debug(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	ensureLoggers()
	errorLogger.Error(msg, fields...)
}
