package log

type Logger interface {
	IsTraceEnabled() bool

	Tracef(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Printf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Warningf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Panicf(format string, args ...interface{})

	Trace(args ...interface{})
	Debug(args ...interface{})
	Info(args ...interface{})
	Print(args ...interface{})
	Warn(args ...interface{})
	Warning(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})
	Panic(args ...interface{})

	Traceln(args ...interface{})
	Debugln(args ...interface{})
	Infoln(args ...interface{})
	Println(args ...interface{})
	Warnln(args ...interface{})
	Warningln(args ...interface{})
	Errorln(args ...interface{})
	Fatalln(args ...interface{})
	Panicln(args ...interface{})
}

var std Logger

func SetLogger(logger Logger) {
	std = logger
}

// IsTraceEnabled returns true if Trace logging is enabled on the standard logger.
func IsTraceEnabled() bool {
	if std == nil {
		return false
	}
	return std.IsTraceEnabled()
}

// Trace logs a message at level Trace on the standard logger.
func Trace(args ...interface{}) {
	if std != nil {
		std.Trace(args...)
	}
}

// Debug logs a message at level Debug on the standard logger.
func Debug(args ...interface{}) {
	if std != nil {
		std.Debug(args...)
	}
}

// Print logs a message at level Info on the standard logger.
func Print(args ...interface{}) {
	if std != nil {
		std.Print(args...)
	}
}

// Info logs a message at level Info on the standard logger.
func Info(args ...interface{}) {
	if std != nil {
		std.Info(args...)
	}
}

// Warn logs a message at level Warn on the standard logger.
func Warn(args ...interface{}) {
	if std != nil {
		std.Warn(args...)
	}
}

// Warning logs a message at level Warn on the standard logger.
func Warning(args ...interface{}) {
	if std != nil {
		std.Warning(args...)
	}
}

// Error logs a message at level Error on the standard logger.
func Error(args ...interface{}) {
	if std != nil {
		std.Error(args...)
	}
}

// Panic logs a message at level Panic on the standard logger.
func Panic(args ...interface{}) {
	if std != nil {
		std.Panic(args...)
	}
}

// Fatal logs a message at level Fatal on the standard logger then the process will exit with status set to 1.
func Fatal(args ...interface{}) {
	if std != nil {
		std.Fatal(args...)
	}
}

// Tracef logs a message at level Trace on the standard logger.
func Tracef(format string, args ...interface{}) {
	if std != nil {
		std.Tracef(format, args...)
	}
}

// Debugf logs a message at level Debug on the standard logger.
func Debugf(format string, args ...interface{}) {
	if std != nil {
		std.Debugf(format, args...)
	}
}

// Printf logs a message at level Info on the standard logger.
func Printf(format string, args ...interface{}) {
	if std != nil {
		std.Printf(format, args...)
	}
}

// Infof logs a message at level Info on the standard logger.
func Infof(format string, args ...interface{}) {
	if std != nil {
		std.Infof(format, args...)
	}
}

// Warnf logs a message at level Warn on the standard logger.
func Warnf(format string, args ...interface{}) {
	if std != nil {
		std.Warnf(format, args...)
	}
}

// Warningf logs a message at level Warn on the standard logger.
func Warningf(format string, args ...interface{}) {
	if std != nil {
		std.Warningf(format, args...)
	}
}

// Errorf logs a message at level Error on the standard logger.
func Errorf(format string, args ...interface{}) {
	if std != nil {
		std.Errorf(format, args...)
	}
}

// Panicf logs a message at level Panic on the standard logger.
func Panicf(format string, args ...interface{}) {
	if std != nil {
		std.Panicf(format, args...)
	}
}

// Fatalf logs a message at level Fatal on the standard logger then the process will exit with status set to 1.
func Fatalf(format string, args ...interface{}) {
	if std != nil {
		std.Fatalf(format, args...)
	}
}

// Traceln logs a message at level Trace on the standard logger.
func Traceln(args ...interface{}) {
	if std != nil {
		std.Traceln(args...)
	}
}

// Debugln logs a message at level Debug on the standard logger.
func Debugln(args ...interface{}) {
	if std != nil {
		std.Debugln(args...)
	}
}

// Println logs a message at level Info on the standard logger.
func Println(args ...interface{}) {
	if std != nil {
		std.Println(args...)
	}
}

// Infoln logs a message at level Info on the standard logger.
func Infoln(args ...interface{}) {
	if std != nil {
		std.Infoln(args...)
	}
}

// Warnln logs a message at level Warn on the standard logger.
func Warnln(args ...interface{}) {
	if std != nil {
		std.Warnln(args...)
	}
}

// Warningln logs a message at level Warn on the standard logger.
func Warningln(args ...interface{}) {
	if std != nil {
		std.Warningln(args...)
	}
}

// Errorln logs a message at level Error on the standard logger.
func Errorln(args ...interface{}) {
	if std != nil {
		std.Errorln(args...)
	}
}

// Panicln logs a message at level Panic on the standard logger.
func Panicln(args ...interface{}) {
	if std != nil {
		std.Panicln(args...)
	}
}

// Fatalln logs a message at level Fatal on the standard logger then the process will exit with status set to 1.
func Fatalln(args ...interface{}) {
	if std != nil {
		std.Fatalln(args...)
	}
}
