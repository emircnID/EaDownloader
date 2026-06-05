package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"eadownloader/internal/localization"
	"eadownloader/internal/logger"
	"go.uber.org/zap/zapcore"
)

func parseEnvString(env string, dest *string, required bool) {
	if value := os.Getenv(env); value != "" {
		*dest = value
	} else if required {
		logger.L.Fatalf("%s env is not set", env)
	}
}

func parseEnvBool(env string, dest *bool, required bool) {
	if value := os.Getenv(env); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			*dest = parsed
		} else {
			logger.L.Fatalf("%s env is not a valid boolean", env)
		}
	} else if required {
		logger.L.Fatalf("%s env is not set", env)
	}
}

func parseEnvInt(env string, dest *int, required bool) {
	if value := os.Getenv(env); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			*dest = parsed
		} else {
			logger.L.Fatalf("%s env is not a valid integer", env)
		}
	} else if required {
		logger.L.Fatalf("%s env is not set", env)
	}
}

func parseEnvFileSize(env string, dest *int64, required bool) {
	if value := os.Getenv(env); value != "" {
		parsed, err := parseFileSize(value)
		if err != nil {
			logger.L.Fatalf("%s env is not a valid file size: %v", env, err)
		}
		*dest = parsed
	} else if required {
		logger.L.Fatalf("%s env is not set", env)
	}
}

func parseFileSize(value string) (int64, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "")

	units := []struct {
		suffix     string
		multiplier int64
	}{
		{"gb", 1024 * 1024 * 1024},
		{"mb", 1024 * 1024},
		{"kb", 1024},
		{"g", 1024 * 1024 * 1024},
		{"m", 1024 * 1024},
		{"k", 1024},
		{"b", 1},
	}

	for _, unit := range units {
		if strings.HasSuffix(value, unit.suffix) {
			numeric := strings.TrimSuffix(value, unit.suffix)
			parsed, err := strconv.ParseInt(numeric, 10, 64)
			if err != nil {
				return 0, err
			}
			return parsed * unit.multiplier, nil
		}
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	if parsed < 1024*1024 {
		return parsed * 1024 * 1024, nil
	}
	return parsed, nil
}

func parseEnvDuration(env string, dest *time.Duration, required bool) {
	if value := os.Getenv(env); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			*dest = parsed
		} else {
			logger.L.Fatalf("%s env is not a valid duration: %v", env, err)
		}
	} else if required {
		logger.L.Fatalf("%s env is not set", env)
	}
}

func parseEnvLevel(env string, dest *zapcore.Level, required bool) {
	if value := os.Getenv(env); value != "" {
		parsed, err := zapcore.ParseLevel(value)
		if err != nil {
			logger.L.Fatalf("%s env is not a valid log level: %v", env, err)
		}
		*dest = parsed
	} else if required {
		logger.L.Fatalf("%s env is not set", env)
	}
}

func parseEnvInt64Slice(env string, dest *[]int64, required bool) {
	if value := os.Getenv(env); value != "" {
		parts := strings.SplitSeq(value, ",")
		for part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				logger.L.Fatalf("%s env contains an invalid int: %s", env, part)
			}
			*dest = append(*dest, id)
		}
	} else if required {
		logger.L.Fatalf("%s env is not set", env)
	}
}

func parseEnvInt32Range(env string, dest *int32, minVal int, maxVal int, required bool) {
	if value := os.Getenv(env); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			if parsed < minVal || parsed > maxVal {
				logger.L.Fatalf("%s env must be between %d and %d", env, minVal, maxVal)
			}
			*dest = int32(parsed)
		} else {
			logger.L.Fatalf("%s env is not a valid integer", env)
		}
	} else if required {
		logger.L.Fatalf("%s env is not set", env)
	}
}

func parseEnvLanguage(env string, dest *string, required bool) {
	if value := os.Getenv(env); value != "" {
		if !localization.IsCodeSupported(value) {
			logger.L.Fatalf("%s env contains unsupported language code: %s", env, value)
		}
		*dest = value
	} else if required {
		logger.L.Fatalf("%s env is not set", env)
	}
}
