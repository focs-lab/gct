package config

// Env variable for setting configs
const TRACE_LOC = "_CCT_TRACE_LOC"
const SCHEDULER_NAME = "_CCT_SCHEDULER_NAME"
const RECORD_FLAG = "_CCT_RECORD_FLAG"
const TRACE_FLAG = "_CCT_TRACE_FLAG"
const ROOT_PROJ_LOC = "_CCT_ROOT_PROJ_LOC"

// PCT
const (
	MAX_PCT_EVENTS = "_CCT_MAX_PCT_EVENTS"
	DEFAULT_MAX_PCT_EVENTS = 500
	PCT_BUG_DEPTH = "_CCT_PCT_BUG_DEPTH"
	DEFAULT_PCT_BUG_DEPTH = 2
)

// log and env variable
const (
	RECORDER_TRACE_LOC = "trace.log"
	ENV_CONFIG_FILE_NAME = "_CCT_ENV_CONFIG_FILE_NAME"
)

const (
	TRACER_TRACE_LOC = "event_trace.log"
)
