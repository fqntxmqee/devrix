// Package logger is a backward-compatibility bridge.
//
// Deprecated: use github.com/devrix/devrix/internal/layers/observability/instrument/logger instead.
// This bridge will be removed in v2.1.
package logger

import "github.com/devrix/devrix/internal/layers/observability/instrument/logger"

// Types — core

type (
	StructuredLogger = logger.StructuredLogger
	SamplingConfig   = logger.SamplingConfig
	LoggerConfig     = logger.LoggerConfig
	RedactorConfig   = logger.RedactorConfig
)

// Types — handler

type (
	LogLevel    = logger.LogLevel
	LogEntry    = logger.LogEntry
	Handler     = logger.Handler
	JSONHandler = logger.JSONHandler
	TextHandler = logger.TextHandler
)

// Types — slog bridge

type ContextHandler = logger.ContextHandler

// Types — redactor

type Redactor = logger.Redactor

// Functions

var (
	NewStructuredLogger = logger.NewStructuredLogger
	DefaultLoggerConfig = logger.DefaultLoggerConfig
	ParseLogLevel       = logger.ParseLogLevel
	NewJSONHandler      = logger.NewJSONHandler
	NewTextHandler      = logger.NewTextHandler
	NewHandler          = logger.NewHandler
	NewContextHandler   = logger.NewContextHandler
	NewRedactor         = logger.NewRedactor
)
