// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ayatsuri-lab/ayatsuri/internal/cmn/cmdutil"
	"github.com/ayatsuri-lab/ayatsuri/internal/cmn/eval"
	"github.com/ayatsuri-lab/ayatsuri/internal/core"
	"github.com/ayatsuri-lab/ayatsuri/internal/core/spec/types"
	"github.com/go-viper/mapstructure/v2"
)

// dag is the intermediate representation of a DAG specification.
// It mirrors the YAML structure and gets validated/transformed into core.DAG.
type dag struct {
	// Name is the name of the DAG.
	Name string `yaml:"name,omitempty"`
	// Group is the group of the DAG for grouping DAGs on the UI.
	Group string `yaml:"group,omitempty"`
	// Description is the description of the DAG.
	Description string `yaml:"description,omitempty"`
	// Shell is the default shell to use for all steps in this DAG.
	// If not specified, the system default shell is used.
	// Can be overridden at the step level.
	// Can be a string (e.g., "bash -e") or an array (e.g., ["bash", "-e"]).
	Shell types.ShellValue `yaml:"shell,omitempty"`
	// WorkingDir is working directory for DAG execution
	WorkingDir string `yaml:"working_dir,omitempty"`
	// Dotenv is the path to the dotenv file (string or []string).
	Dotenv types.StringOrArray `yaml:"dotenv,omitempty"`
	// Schedule is the cron schedule to run the DAG.
	Schedule types.ScheduleValue `yaml:"schedule,omitempty"`
	// SkipIfSuccessful is the flag to skip the DAG on schedule when it is
	// executed manually before the schedule.
	SkipIfSuccessful bool `yaml:"skip_if_successful,omitempty"`
	// CatchupWindow is the lookback horizon for missed intervals (e.g. "6h", "2d12h").
	// If set, enables catch-up on scheduler restart. If omitted, no catch-up.
	CatchupWindow string `yaml:"catchup_window,omitempty"`
	// OverlapPolicy controls how multiple catch-up runs are handled: "skip" or "all".
	OverlapPolicy string `yaml:"overlap_policy,omitempty"`
	// LogDir is the directory where the logs are stored.
	LogDir string `yaml:"log_dir,omitempty"`
	// LogOutput specifies how stdout and stderr are handled in log files.
	// Can be "separate" (default) for separate .out and .err files,
	// or "merged" for a single combined .log file.
	LogOutput types.LogOutputValue `yaml:"log_output,omitempty"`
	// Env is the environment variables setting.
	Env types.EnvValue `yaml:"env,omitempty"`
	// HandlerOn is the handler configuration.
	HandlerOn handlerOn `yaml:"handler_on,omitempty"`
	// Steps is the list of steps to run.
	Steps any `yaml:"steps,omitempty"` // []step or map[string]step
	// SMTP is the SMTP configuration.
	SMTP smtpConfig `yaml:"smtp,omitempty"`
	// MailOn is the mail configuration.
	MailOn *mailOn `yaml:"mail_on,omitempty"`
	// ErrorMail is the mail configuration for error.
	ErrorMail mailConfig `yaml:"error_mail,omitempty"`
	// InfoMail is the mail configuration for information.
	InfoMail mailConfig `yaml:"info_mail,omitempty"`
	// WaitMail is the mail configuration for wait status.
	WaitMail mailConfig `yaml:"wait_mail,omitempty"`
	// TimeoutSec is the timeout in seconds to finish the DAG.
	TimeoutSec int `yaml:"timeout_sec,omitempty"`
	// DelaySec is the delay in seconds to start the first node.
	DelaySec int `yaml:"delay_sec,omitempty"`
	// RestartWaitSec is the wait in seconds to when the DAG is restarted.
	RestartWaitSec int `yaml:"restart_wait_sec,omitempty"`
	// HistRetentionDays is the retention days of the dag-runs history.
	HistRetentionDays *int `yaml:"hist_retention_days,omitempty"`
	// Preconditions is the condition to run the DAG.
	Preconditions any `yaml:"preconditions,omitempty"`
	// MaxActiveRuns is the maximum number of concurrent dag-runs.
	MaxActiveRuns int `yaml:"max_active_runs,omitempty"`
	// MaxActiveSteps is the maximum number of concurrent steps.
	MaxActiveSteps int `yaml:"max_active_steps,omitempty"`
	// Params is the default parameters for the steps.
	Params any `yaml:"params,omitempty"`
	// MaxCleanUpTimeSec is the maximum time in seconds to clean up the DAG.
	// It is a wait time to kill the processes when it is requested to stop.
	// If the time is exceeded, the process is killed.
	MaxCleanUpTimeSec *int `yaml:"max_clean_up_time_sec,omitempty"`
	// Tags is the tags for the DAG.
	Tags types.TagsValue `yaml:"tags,omitempty"`
	// Queue is the name of the queue to assign this DAG to.
	Queue string `yaml:"queue,omitempty"`
	// RetryPolicy is the DAG-level retry policy.
	RetryPolicy *dagRetryPolicy `yaml:"retry_policy,omitempty"`
	// MaxOutputSize is the maximum size of the output for each step.
	MaxOutputSize int `yaml:"max_output_size,omitempty"`
	// OTel is the OpenTelemetry configuration.
	OTel any `yaml:"otel,omitempty"`
	// WorkerSelector specifies required worker labels for execution.
	// Can be a map of label key-value pairs or the string "local" to force local execution.
	WorkerSelector any `yaml:"worker_selector,omitempty"`
	// Container is the container definition for the DAG.
	// Can be a string (existing container name to exec into) or an object (container configuration).
	Container any `yaml:"container,omitempty"`
	// RunConfig contains configuration for controlling user interactions during DAG runs.
	RunConfig *runConfig `yaml:"run_config,omitempty"`
	// SSH is the default SSH configuration for the DAG.
	SSH *ssh `yaml:"ssh,omitempty"`
	// LLM is the default LLM configuration for all chat steps in this DAG.
	// Steps can override this configuration by specifying their own llm field.
	LLM *llmConfig `yaml:"llm,omitempty"`
	// Secrets contains references to external secrets.
	Secrets []secretRef `yaml:"secrets,omitempty"`
	// Defaults defines default values for step configuration fields.
	// Steps inherit these defaults and can override them individually.
	Defaults any `yaml:"defaults,omitempty"`
}

// dagRetryPolicy defines the retry policy for a DAG run.
type dagRetryPolicy struct {
	Limit          any `yaml:"limit,omitempty"`
	IntervalSec    any `yaml:"interval_sec,omitempty"`
	Backoff        any `yaml:"backoff,omitempty"`
	MaxIntervalSec any `yaml:"max_interval_sec,omitempty"`
}

// smtpConfig defines the SMTP configuration.
type smtpConfig struct {
	Host     string          `yaml:"host,omitempty"`     // SMTP host
	Port     types.PortValue `yaml:"port,omitempty"`     // SMTP port (can be string or number)
	Username string          `yaml:"username,omitempty"` // SMTP username
	Password string          `yaml:"password,omitempty"` // SMTP password
}

// IsZero returns true if all fields are empty/default.
func (s smtpConfig) IsZero() bool {
	return s == smtpConfig{}
}

// mailConfig defines the mail configuration.
type mailConfig struct {
	From       string              `yaml:"from,omitempty"`        // Sender email address
	To         types.StringOrArray `yaml:"to,omitempty"`          // Recipient email address(es) - can be string or []string
	Prefix     string              `yaml:"prefix,omitempty"`      // Prefix for the email subject
	AttachLogs bool                `yaml:"attach_logs,omitempty"` // Flag to attach logs to the email
}

// IsZero returns true if all fields are empty/default.
func (m mailConfig) IsZero() bool {
	return reflect.DeepEqual(m, mailConfig{})
}

// mailOn defines the conditions to send mail.
type mailOn struct {
	Failure bool `yaml:"failure,omitempty"` // Send mail on failure
	Success bool `yaml:"success,omitempty"` // Send mail on success
	Wait    bool `yaml:"wait,omitempty"`    // Send mail on wait status
}

// handlerOn defines the steps to be executed on different events.
type handlerOn struct {
	Init    *step `yaml:"init,omitempty"`    // Step to execute before steps (after preconditions pass)
	Failure *step `yaml:"failure,omitempty"` // Step to execute on failure
	Success *step `yaml:"success,omitempty"` // Step to execute on success
	Abort   *step `yaml:"abort,omitempty"`   // Step to execute on abort
	Exit    *step `yaml:"exit,omitempty"`    // Step to execute on exit
	Wait    *step `yaml:"wait,omitempty"`    // Step to execute when DAG enters wait status (approval)
}

// container defines the container configuration for the DAG.
type container struct {
	// Exec specifies an existing container to exec into.
	// Mutually exclusive with Image.
	Exec string `yaml:"exec,omitempty"`
	// Name is the container name to use. If empty, Docker generates a random name.
	Name string `yaml:"name,omitempty"`
	// Image is the container image to use.
	Image string `yaml:"image,omitempty"`
	// PullPolicy is the policy to pull the image (e.g., "Always", "IfNotPresent").
	PullPolicy any `yaml:"pull_policy,omitempty"`
	// Env specifies environment variables for the container.
	Env any `yaml:"env,omitempty"` // Can be a map or struct
	// Volumes specifies the volumes to mount in the container.
	Volumes []string `yaml:"volumes,omitempty"` // Map of volume names to volume definitions
	// User is the user to run the container as.
	User string `yaml:"user,omitempty"` // User to run the container as
	// WorkingDir is the working directory inside the container.
	WorkingDir string `yaml:"working_dir,omitempty"` // Working directory inside the container
	// Platform specifies the platform for the container (e.g., "linux/amd64").
	Platform string `yaml:"platform,omitempty"` // Platform for the container
	// Ports specifies the ports to expose from the container.
	Ports []string `yaml:"ports,omitempty"` // List of ports to expose
	// Network is the network configuration for the container.
	Network string `yaml:"network,omitempty"` // Network configuration for the container
	// KeepContainer is the flag to keep the container after the DAG run.
	KeepContainer bool `yaml:"keep_container,omitempty"` // Keep the container after the DAG run
	// Startup determines how the DAG-level container starts up.
	Startup string `yaml:"startup,omitempty"`
	// Command used when Startup == "command".
	Command []string `yaml:"command,omitempty"`
	// WaitFor readiness condition: running|healthy
	WaitFor string `yaml:"wait_for,omitempty"`
	// LogPattern regex to wait for in container logs.
	LogPattern string `yaml:"log_pattern,omitempty"`
	// RestartPolicy: no|always|unless-stopped
	RestartPolicy string `yaml:"restart_policy,omitempty"`
	// Healthcheck defines a custom healthcheck for the container.
	Healthcheck *healthcheck `yaml:"healthcheck,omitempty"`
	// Shell specifies the shell wrapper for executing step commands.
	Shell []string `yaml:"shell,omitempty"`
}

// healthcheck is the spec representation for custom health checks.
// Durations are specified as strings (e.g., "5s", "1m") for YAML convenience.
type healthcheck struct {
	// Test is the command to run. Must start with NONE, CMD, or CMD-SHELL.
	Test []string `yaml:"test,omitempty"`
	// Interval is the time between checks (e.g., "5s").
	Interval string `yaml:"interval,omitempty"`
	// Timeout is how long to wait for the check to complete (e.g., "3s").
	Timeout string `yaml:"timeout,omitempty"`
	// StartPeriod is the grace period for container initialization (e.g., "10s").
	StartPeriod string `yaml:"start_period,omitempty"`
	// Retries is the number of consecutive failures needed to mark unhealthy.
	Retries int `yaml:"retries,omitempty"`
}

// runConfig defines configuration for controlling user interactions during DAG runs.
type runConfig struct {
	DisableParamEdit bool `yaml:"disable_param_edit,omitempty"`  // Disable parameter editing when starting DAG
	DisableRunIdEdit bool `yaml:"disable_run_id_edit,omitempty"` // Disable custom run ID specification
}

// ssh defines the SSH configuration for the DAG.
type ssh struct {
	// User is the SSH user.
	User string `yaml:"user,omitempty"`
	// Host is the SSH host.
	Host string `yaml:"host,omitempty"`
	// Port is the SSH port (can be string or number).
	Port types.PortValue `yaml:"port,omitempty"`
	// Key is the path to the SSH private key.
	Key string `yaml:"key,omitempty"`
	// Password is the SSH password.
	Password string `yaml:"password,omitempty"`
	// StrictHostKey enables strict host key checking. Defaults to true if not specified.
	StrictHostKey *bool `yaml:"strict_host_key,omitempty"`
	// KnownHostFile is the path to the known_hosts file. Defaults to ~/.ssh/known_hosts.
	KnownHostFile string `yaml:"known_host_file,omitempty"`
	// Shell is the shell to use for remote command execution.
	// Supports string or array syntax (e.g., "bash -e" or ["bash", "-e"]).
	// If not specified, commands are executed directly without shell wrapping.
	Shell types.ShellValue `yaml:"shell,omitempty"`
	// Timeout is the connection timeout duration (e.g., "30s", "1m"). Defaults to 30s.
	Timeout string `yaml:"timeout,omitempty"`
	// Bastion is the jump host / bastion server configuration.
	Bastion *bastion `yaml:"bastion,omitempty"`
}

// bastion defines the bastion/jump host configuration.
type bastion struct {
	// Host is the bastion host address.
	Host string `yaml:"host,omitempty"`
	// Port is the bastion SSH port (can be string or number).
	Port types.PortValue `yaml:"port,omitempty"`
	// User is the bastion SSH user.
	User string `yaml:"user,omitempty"`
	// Key is the path to the SSH private key for the bastion.
	Key string `yaml:"key,omitempty"`
	// Password is the SSH password for the bastion.
	Password string `yaml:"password,omitempty"`
}

// secretRef defines a reference to an external secret.
type secretRef struct {
	// Name is the environment variable name (required).
	Name string `yaml:"name"`
	// Provider specifies the secret backend (required).
	Provider string `yaml:"provider"`
	// Key is the provider-specific identifier (required).
	Key string `yaml:"key"`
	// Options contains provider-specific configuration (optional).
	Options map[string]string `yaml:"options,omitempty"`
}

// Transformer transforms a spec field into output field(s).
// C is the context type, T is the input type.
type Transformer[C any, T any] interface {
	// Transform performs the transformation and sets field(s) on out
	Transform(ctx C, in T, out reflect.Value) error
}

// dagTransformer is a generic implementation that provides type safety
// for the builder function while satisfying the DAGTransformer interface.
type dagTransformer[T any] struct {
	fieldName string
	builder   func(ctx BuildContext, d *dag) (T, error)
}

func (t *dagTransformer[T]) Transform(ctx BuildContext, in *dag, out reflect.Value) error {
	v, err := t.builder(ctx, in)
	if err != nil {
		return err
	}
	field := out.FieldByName(t.fieldName)
	if field.IsValid() && field.CanSet() {
		field.Set(reflect.ValueOf(v))
	}
	return nil
}

// newTransformer creates a DAGTransformer for a single field transformation
func newTransformer[T any](fieldName string, builder func(BuildContext, *dag) (T, error)) Transformer[BuildContext, *dag] {
	return &dagTransformer[T]{
		fieldName: fieldName,
		builder:   builder,
	}
}

// transform wraps a DAGTransformer with its name for error reporting
type transform struct {
	name        string
	transformer Transformer[BuildContext, *dag]
}

// metadataTransformers are always run (for listing, scheduling, etc.)
var metadataTransformers = []transform{
	{"name", newTransformer("Name", buildName)},
	{"group", newTransformer("Group", buildGroup)},
	{"description", newTransformer("Description", buildDescription)},
	{"tags", newTransformer("Tags", buildTags)},
	// params must run BEFORE env so that env: values can reference ${param_name}
	{"params", newTransformer("Params", buildParams)},
	{"default_params", newTransformer("DefaultParams", buildDefaultParams)},
	{"param_defs", newTransformer("ParamDefs", buildParamDefs)},
	{"params_json", newTransformer("ParamsJSON", buildParamsJSON)},
	{"env", newTransformer("Env", buildEnvs)},
	{"schedule", newTransformer("Schedule", buildSchedule)},
	{"stop_schedule", newTransformer("StopSchedule", buildStopSchedule)},
	{"restart_schedule", newTransformer("RestartSchedule", buildRestartSchedule)},
	{"worker_selector", &workerSelectorTransformer{}},
	{"timeout", newTransformer("Timeout", buildTimeout)},
	{"delay", newTransformer("Delay", buildDelay)},
	{"restart_wait", newTransformer("RestartWait", buildRestartWait)},
	{"max_active_runs", newTransformer("MaxActiveRuns", buildMaxActiveRuns)},
	{"max_active_steps", newTransformer("MaxActiveSteps", buildMaxActiveSteps)},
	{"queue", newTransformer("Queue", buildQueue)},
	{"retry_policy", newTransformer("RetryPolicy", buildDAGRetryPolicy)},
	{"max_output_size", newTransformer("MaxOutputSize", buildMaxOutputSize)},
	{"skip_if_successful", newTransformer("SkipIfSuccessful", buildSkipIfSuccessful)},
	{"catchup_window", newTransformer("CatchupWindow", buildCatchupWindow)},
	{"overlap_policy", newTransformer("OverlapPolicy", buildOverlapPolicy)},
}

// fullTransformers are only run when building the full DAG (not metadata-only)
var fullTransformers = []transform{
	{"log_dir", newTransformer("LogDir", buildLogDir)},
	{"log_output", newTransformer("LogOutput", buildLogOutput)},
	{"mail_on", newTransformer("MailOn", buildMailOn)},
	{"run_config", newTransformer("RunConfig", buildRunConfig)},
	{"hist_retention_days", newTransformer("HistRetentionDays", buildHistRetentionDays)},
	{"max_clean_up_time_sec", newTransformer("MaxCleanUpTime", buildMaxCleanUpTime)},
	{"shell", newTransformer("Shell", buildShell)},
	{"shell_args", newTransformer("ShellArgs", buildShellArgs)},
	{"working_dir", newTransformer("WorkingDir", buildWorkingDir)},
	{"container", newTransformer("Container", buildContainer)},
	{"ssh", newTransformer("SSH", buildSSH)},
	{"llm", newTransformer("LLM", buildLLM)},
	{"secrets", newTransformer("Secrets", buildSecrets)},
	{"dotenv", newTransformer("Dotenv", buildDotenv)},
	{"smtp", newTransformer("SMTP", buildSMTPConfig)},
	{"error_mail", newTransformer("ErrorMail", buildErrMailConfig)},
	{"info_mail", newTransformer("InfoMail", buildInfoMailConfig)},
	{"wait_mail", newTransformer("WaitMail", buildWaitMailConfig)},
	{"preconditions", newTransformer("Preconditions", buildPreconditions)},
	{"otel", newTransformer("OTel", buildOTel)},
}

// runTransformers executes all transformers in the pipeline
func runTransformers(ctx BuildContext, spec *dag, result *core.DAG) core.ErrorList {
	var errs core.ErrorList
	out := reflect.ValueOf(result).Elem()

	// Always run metadata transformers
	for _, t := range metadataTransformers {
		if err := t.transformer.Transform(ctx, spec, out); err != nil {
			errs = append(errs, wrapTransformError(t.name, err))
		}
	}

	// Run full transformers only when not in metadata-only mode
	if !ctx.opts.Has(BuildFlagOnlyMetadata) {
		for _, t := range fullTransformers {
			if err := t.transformer.Transform(ctx, spec, out); err != nil {
				errs = append(errs, wrapTransformError(t.name, err))
			}
		}
	}

	return errs
}

// wrapTransformError wraps an error with the transformer name if it's not already a ValidationError
func wrapTransformError(name string, err error) error {
	var ve *core.ValidationError
	if errors.As(err, &ve) {
		return err
	}
	return core.NewValidationError(name, nil, err)
}

// build transforms the dag specification into a core.DAG.
func (d *dag) build(ctx BuildContext) (*core.DAG, error) {
	// Initialize with only Location (set from context, not spec)
	result := &core.DAG{
		Location: ctx.file,
	}

	// Initialize shared envScope state for thread-safe env var handling.
	// Start with OS environment as base layer.
	baseScope := eval.NewEnvScope(nil, true)

	// Pre-populate with build env from options (for retry with dotenv).
	// This allows YAML to reference env vars that were loaded from .env files
	// before the rebuild.
	buildEnv := make(map[string]string, len(ctx.opts.BuildEnv))
	maps.Copy(buildEnv, ctx.opts.BuildEnv)
	if len(buildEnv) > 0 {
		baseScope = baseScope.WithEntries(buildEnv, eval.EnvSourceDotEnv)
	}

	ctx.envScope = &envScopeState{
		scope:    baseScope,
		buildEnv: buildEnv,
	}
	ctx.paramsState = &paramsState{}

	// Run the transformer pipeline
	errs := runTransformers(ctx, d, result)

	// Add deprecation warning for max_active_runs on local queues.
	// Both max_active_runs > 1 (concurrency) and max_active_runs < 0 (queue bypass) are deprecated.
	if result.Queue == "" && (result.MaxActiveRuns > 1 || result.MaxActiveRuns < 0) {
		result.BuildWarnings = append(result.BuildWarnings, fmt.Sprintf(
			"max_active_runs=%d is deprecated for local queues and will be ignored. "+
				"Use a global queue with 'queue:' field for concurrency control.",
			result.MaxActiveRuns,
		))
	}

	// Collect schedule warnings (misleading step values like */33).
	for _, sched := range result.Schedule {
		result.BuildWarnings = append(result.BuildWarnings, sched.Warnings...)
	}
	for _, sched := range result.StopSchedule {
		result.BuildWarnings = append(result.BuildWarnings, sched.Warnings...)
	}
	for _, sched := range result.RestartSchedule {
		result.BuildWarnings = append(result.BuildWarnings, sched.Warnings...)
	}

	// Build handlers and steps directly (they need access to partially built result)
	if !ctx.opts.Has(BuildFlagOnlyMetadata) {
		if handlerOn, err := buildHandlers(ctx, d, result); err != nil {
			errs = append(errs, core.NewValidationError("handlers", nil, err))
		} else {
			result.HandlerOn = handlerOn
		}

		if steps, err := buildSteps(ctx, d, result); err != nil {
			errs = append(errs, core.NewValidationError("steps", nil, err))
		} else {
			result.Steps = steps
		}
	}

	// Validate steps
	if err := core.ValidateSteps(result); err != nil {
		errs = append(errs, err)
	}

	// Validate workerSelector compatibility with approval steps
	if len(result.WorkerSelector) > 0 && result.HasApprovalSteps() {
		errs = append(errs, core.NewValidationError(
			"worker_selector",
			result.WorkerSelector,
			fmt.Errorf("DAG with approval steps cannot be dispatched to workers"),
		))
	}

	// Validate name
	if result.Name != "" {
		if err := core.ValidateDAGName(result.Name); err != nil {
			errs = append(errs, core.NewValidationError("name", result.Name, err))
		}
	}

	if len(ctx.envScope.buildEnv) > 0 {
		result.PresolvedBuildEnv = maps.Clone(ctx.envScope.buildEnv)
	}

	if len(errs) > 0 {
		if ctx.opts.Has(BuildFlagAllowBuildErrors) {
			result.BuildErrors = errs
		} else {
			return nil, fmt.Errorf("failed to build DAG: %w", errs)
		}
	}

	return result, nil
}

// Builder functions - all return values instead of modifying result

func buildName(ctx BuildContext, d *dag) (string, error) {
	if ctx.opts.Name != "" && ctx.index == 0 {
		return strings.TrimSpace(ctx.opts.Name), nil
	}
	if name := strings.TrimSpace(d.Name); name != "" {
		return name, nil
	}
	// Fallback to filename without extension only for the main DAG (index 0)
	// Sub-DAGs in multi-DAG files must have explicit names
	if ctx.index == 0 {
		return defaultName(ctx.file), nil
	}
	return "", nil
}

func buildGroup(_ BuildContext, d *dag) (string, error) {
	return strings.TrimSpace(d.Group), nil
}

func buildDescription(_ BuildContext, d *dag) (string, error) {
	return strings.TrimSpace(d.Description), nil
}

func buildTimeout(_ BuildContext, d *dag) (time.Duration, error) {
	return time.Second * time.Duration(d.TimeoutSec), nil
}

func buildDelay(_ BuildContext, d *dag) (time.Duration, error) {
	return time.Second * time.Duration(d.DelaySec), nil
}

func buildRestartWait(_ BuildContext, d *dag) (time.Duration, error) {
	return time.Second * time.Duration(d.RestartWaitSec), nil
}

func buildTags(_ BuildContext, d *dag) (core.Tags, error) {
	if d.Tags.IsZero() {
		return nil, nil
	}
	var tags core.Tags
	for _, entry := range d.Tags.Entries() {
		if entry.Key() == "" {
			continue
		}
		tags = append(tags, core.Tag{
			Key:   strings.ToLower(strings.TrimSpace(entry.Key())),
			Value: strings.ToLower(strings.TrimSpace(entry.Value())),
		})
	}
	return tags, nil
}

func buildMaxActiveRuns(_ BuildContext, d *dag) (int, error) {
	if d.MaxActiveRuns != 0 {
		return d.MaxActiveRuns, nil
	}
	return 1, nil // Default
}

func buildMaxActiveSteps(_ BuildContext, d *dag) (int, error) {
	return d.MaxActiveSteps, nil
}

func buildQueue(_ BuildContext, d *dag) (string, error) {
	return strings.TrimSpace(d.Queue), nil
}

func buildDAGRetryPolicy(_ BuildContext, d *dag) (*core.DAGRetryPolicy, error) {
	if d.RetryPolicy == nil {
		return nil, nil
	}

	// Root DAG retry must be concrete when the DAG is loaded because scheduler
	// retry decisions evaluate the persisted DAG snapshot without re-resolving
	// retry expressions at runtime.
	limit, err := parseDAGRetryLimit(d.RetryPolicy.Limit)
	if err != nil {
		return nil, err
	}

	interval, intervalStr, err := parseDAGRetryInterval(d.RetryPolicy.IntervalSec)
	if err != nil {
		return nil, err
	}

	backoff, err := parseDAGRetryBackoff(d.RetryPolicy.Backoff)
	if err != nil {
		return nil, err
	}

	maxInterval, err := parseDAGRetryMaxInterval(d.RetryPolicy.MaxIntervalSec)
	if err != nil {
		return nil, err
	}

	return &core.DAGRetryPolicy{
		Limit:          limit,
		Interval:       interval,
		IntervalSecStr: intervalStr,
		Backoff:        backoff,
		MaxInterval:    maxInterval,
	}, nil
}

func buildMaxOutputSize(_ BuildContext, d *dag) (int, error) {
	return d.MaxOutputSize, nil
}

func buildSkipIfSuccessful(_ BuildContext, d *dag) (bool, error) {
	return d.SkipIfSuccessful, nil
}

func buildCatchupWindow(_ BuildContext, d *dag) (time.Duration, error) {
	if d.CatchupWindow == "" {
		return 0, nil
	}
	return core.ParseDuration(d.CatchupWindow)
}

func buildOverlapPolicy(_ BuildContext, d *dag) (core.OverlapPolicy, error) {
	return core.ParseOverlapPolicy(d.OverlapPolicy)
}

func buildLogDir(_ BuildContext, d *dag) (string, error) {
	return d.LogDir, nil
}

func buildLogOutput(_ BuildContext, d *dag) (core.LogOutputMode, error) {
	if d.LogOutput.IsZero() {
		// Return empty to allow inheritance from base config.
		// Default is applied in core.InitializeDefaults.
		return "", nil
	}
	return d.LogOutput.Mode(), nil
}

func buildRunConfig(_ BuildContext, d *dag) (*core.RunConfig, error) {
	if d.RunConfig == nil {
		return nil, nil
	}
	return &core.RunConfig{
		DisableParamEdit: d.RunConfig.DisableParamEdit,
		DisableRunIdEdit: d.RunConfig.DisableRunIdEdit,
	}, nil
}

func buildHistRetentionDays(_ BuildContext, d *dag) (int, error) {
	if d.HistRetentionDays != nil {
		return *d.HistRetentionDays, nil
	}
	return 0, nil
}

func buildMaxCleanUpTime(_ BuildContext, d *dag) (time.Duration, error) {
	if d.MaxCleanUpTimeSec != nil {
		return time.Second * time.Duration(*d.MaxCleanUpTimeSec), nil
	}
	return 0, nil
}

func buildEnvs(ctx BuildContext, d *dag) ([]string, error) {
	vars, err := loadVariablesFromEnvValue(ctx, d.Env)
	if err != nil {
		return nil, err
	}

	// Add vars to the shared envScope state so subsequent transformers can use it.
	// This replaces the old pattern of using os.Setenv which caused race conditions.
	if ctx.envScope != nil && len(vars) > 0 {
		ctx.envScope.scope = ctx.envScope.scope.WithEntries(vars, eval.EnvSourceDAGEnv)
		maps.Copy(ctx.envScope.buildEnv, vars)
	}

	var envs []string
	for k, v := range vars {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}
	return envs, nil
}

func buildSchedule(_ BuildContext, d *dag) ([]core.Schedule, error) {
	if d.Schedule.IsZero() {
		return nil, nil
	}
	return slices.Clone(d.Schedule.Starts()), nil
}

func buildStopSchedule(_ BuildContext, d *dag) ([]core.Schedule, error) {
	if d.Schedule.IsZero() {
		return nil, nil
	}
	return slices.Clone(d.Schedule.Stops()), nil
}

func buildRestartSchedule(_ BuildContext, d *dag) ([]core.Schedule, error) {
	if d.Schedule.IsZero() {
		return nil, nil
	}
	return slices.Clone(d.Schedule.Restarts()), nil
}

// paramsResult holds the result of parsing parameters
type paramsResult struct {
	Params        []string
	DefaultParams string
	ParamDefs     []core.ParamDef
	ParamsJSON    string // JSON representation of resolved params (original payload when provided as JSON)
}

func buildParams(ctx BuildContext, d *dag) ([]string, error) {
	result, err := parseParamsInternal(ctx, d)
	if err != nil {
		return nil, err
	}
	// Add resolved params to envScope so env: can reference ${param_name}
	if ctx.envScope != nil && len(result.Params) > 0 {
		paramVars := make(map[string]string, len(result.Params))
		for _, p := range result.Params {
			if k, v, ok := strings.Cut(p, "="); ok {
				paramVars[k] = v
			}
		}
		ctx.envScope.scope = ctx.envScope.scope.WithEntries(paramVars, eval.EnvSourceParam)
	}
	return result.Params, nil
}

func buildDefaultParams(ctx BuildContext, d *dag) (string, error) {
	result, err := parseParamsInternal(ctx, d)
	if err != nil {
		return "", err
	}
	return result.DefaultParams, nil
}

func buildParamDefs(ctx BuildContext, d *dag) ([]core.ParamDef, error) {
	result, err := parseParamsInternal(ctx, d)
	if err != nil {
		return nil, err
	}
	return result.ParamDefs, nil
}

func buildParamsJSON(ctx BuildContext, d *dag) (string, error) {
	result, err := parseParamsInternal(ctx, d)
	if err != nil {
		return "", err
	}
	return result.ParamsJSON, nil
}

// detectJSONParams checks if the input string is valid JSON and returns it if so.
// Returns empty string if the input is not JSON.
func detectJSONParams(input string) string {
	input = strings.TrimSpace(input)
	if (strings.HasPrefix(input, "{") && strings.HasSuffix(input, "}")) ||
		(strings.HasPrefix(input, "[") && strings.HasSuffix(input, "]")) {
		var js json.RawMessage
		if json.Unmarshal([]byte(input), &js) == nil {
			return input
		}
	}
	return ""
}

// buildResolvedParamsJSON returns a JSON representation of the resolved params.
// If the raw input was JSON, the original payload is returned to preserve structure.
func buildResolvedParamsJSON(paramPairs []paramPair, rawInput string) (string, error) {
	if rawJSON := detectJSONParams(rawInput); rawJSON != "" {
		return rawJSON, nil
	}
	return marshalParamPairs(paramPairs)
}

func parseDAGRetryInterval(v any) (time.Duration, string, error) {
	if v == nil {
		return 60 * time.Second, "", nil
	}
	interval, intervalStr, err := parseConcreteDAGRetryInt("retry_policy.interval_sec", v)
	if err != nil {
		return 0, "", err
	}
	return time.Second * time.Duration(interval), intervalStr, nil
}

func parseDAGRetryBackoff(v any) (float64, error) {
	backoff, err := parseBackoffValue(v, "retry_policy.backoff")
	if err != nil {
		return 0, core.NewValidationError("retry_policy.backoff", v, err)
	}
	return backoff, nil
}

func parseDAGRetryMaxInterval(v any) (time.Duration, error) {
	if v == nil {
		return time.Hour, nil
	}
	seconds, _, err := parseConcreteDAGRetryInt("retry_policy.max_interval_sec", v)
	if err != nil {
		return 0, err
	}
	return time.Second * time.Duration(seconds), nil
}

func parseDAGRetryLimit(v any) (int, error) {
	if v == nil {
		return 0, core.NewValidationError("retry_policy.limit", nil, fmt.Errorf("limit is required when retry_policy is specified"))
	}
	limit, _, err := parseConcreteDAGRetryInt("retry_policy.limit", v)
	if err != nil {
		return 0, err
	}
	return limit, nil
}

func parseConcreteDAGRetryInt(fieldName string, val any) (int, string, error) {
	switch v := val.(type) {
	case int:
		if v <= 0 {
			return 0, "", core.NewValidationError(fieldName, v, fmt.Errorf("%s must be > 0", retryFieldLabel(fieldName)))
		}
		return v, "", nil
	case int64:
		if v <= 0 {
			return 0, "", core.NewValidationError(fieldName, v, fmt.Errorf("%s must be > 0", retryFieldLabel(fieldName)))
		}
		if v > math.MaxInt {
			return 0, "", core.NewValidationError(fieldName, v, fmt.Errorf("value %d exceeds maximum int", v))
		}
		return int(v), "", nil
	case uint64:
		if v == 0 {
			return 0, "", core.NewValidationError(fieldName, v, fmt.Errorf("%s must be > 0", retryFieldLabel(fieldName)))
		}
		if v > math.MaxInt {
			return 0, "", core.NewValidationError(fieldName, v, fmt.Errorf("value %d exceeds maximum int", v))
		}
		return int(v), "", nil
	case string:
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return 0, "", core.NewValidationError(fieldName, v, fmt.Errorf("%s must be an integer or numeric string", retryFieldLabel(fieldName)))
		}
		if parsed <= 0 {
			return 0, "", core.NewValidationError(fieldName, v, fmt.Errorf("%s must be > 0", retryFieldLabel(fieldName)))
		}
		return parsed, v, nil
	default:
		return 0, "", core.NewValidationError(fieldName, val, fmt.Errorf("invalid type: %T", val))
	}
}

func retryFieldLabel(fieldName string) string {
	if idx := strings.LastIndex(fieldName, "."); idx >= 0 {
		return fieldName[idx+1:]
	}
	return fieldName
}

// marshalParamPairs converts the final param pairs into a JSON object string.
// Returns an empty string when there are no params to serialize.
func marshalParamPairs(paramPairs []paramPair) (string, error) {
	if len(paramPairs) == 0 {
		return "", nil
	}

	payload := make(map[string]string, len(paramPairs))
	for _, pair := range paramPairs {
		if pair.Name == "" {
			continue
		}
		payload[pair.Name] = pair.Value
	}

	if len(payload) == 0 {
		return "", nil
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal params to JSON: %w", err)
	}
	return string(data), nil
}

func parseParamsInternal(ctx BuildContext, d *dag) (*paramsResult, error) {
	if ctx.paramsState != nil && ctx.paramsState.cached {
		return ctx.paramsState.result, ctx.paramsState.err
	}

	result, err := buildDAGParamsResult(ctx, d)
	if ctx.paramsState != nil {
		ctx.paramsState.cached = true
		ctx.paramsState.result = result
		ctx.paramsState.err = err
	}
	return result, err
}

// workerSelectorTransformer is a custom transformer that sets both WorkerSelector and ForceLocal fields.
type workerSelectorTransformer struct{}

func (t *workerSelectorTransformer) Transform(ctx BuildContext, in *dag, out reflect.Value) error {
	ws, forceLocal, err := buildWorkerSelector(ctx, in)
	if err != nil {
		return err
	}

	if ws != nil {
		wsField := out.FieldByName("WorkerSelector")
		if wsField.IsValid() && wsField.CanSet() {
			wsField.Set(reflect.ValueOf(ws))
		}
	}

	if forceLocal {
		flField := out.FieldByName("ForceLocal")
		if flField.IsValid() && flField.CanSet() {
			flField.SetBool(true)
		}
	}

	return nil
}

func buildWorkerSelector(_ BuildContext, d *dag) (map[string]string, bool, error) {
	if d.WorkerSelector == nil {
		return nil, false, nil
	}

	switch v := d.WorkerSelector.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if strings.EqualFold(trimmed, "local") {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("unsupported worker_selector string value %q; the only allowed string value is \"local\"", trimmed)

	case map[string]string:
		if len(v) == 0 {
			return nil, false, nil
		}
		ret := make(map[string]string)
		for key, val := range v {
			ret[strings.TrimSpace(key)] = strings.TrimSpace(val)
		}
		return ret, false, nil

	case map[string]any:
		if len(v) == 0 {
			return nil, false, nil
		}
		ret := make(map[string]string)
		for key, val := range v {
			ret[strings.TrimSpace(key)] = strings.TrimSpace(fmt.Sprint(val))
		}
		return ret, false, nil

	case map[any]any:
		if len(v) == 0 {
			return nil, false, nil
		}
		ret := make(map[string]string)
		for key, val := range v {
			strKey, ok := key.(string)
			if !ok {
				return nil, false, fmt.Errorf("worker_selector keys must be strings, got %T", key)
			}
			ret[strings.TrimSpace(strKey)] = strings.TrimSpace(fmt.Sprint(val))
		}
		return ret, false, nil

	default:
		return nil, false, fmt.Errorf("worker_selector must be a map or \"local\", got %T", d.WorkerSelector)
	}
}

// shellResult holds both shell and args for internal use
type shellResult struct {
	Shell string
	Args  []string
}

func parseShellInternal(_ BuildContext, d *dag) (*shellResult, error) {
	if d.Shell.IsZero() {
		return &shellResult{Shell: cmdutil.GetShellCommand(""), Args: nil}, nil
	}

	// For array form, Command() returns first element, Arguments() returns rest
	if d.Shell.IsArray() {
		shell := d.Shell.Command()
		// Empty array should fall back to default shell
		if shell == "" {
			return &shellResult{Shell: cmdutil.GetShellCommand(""), Args: nil}, nil
		}
		// Shell expansion is deferred to runtime - see runtime/env.go Shell()
		args := d.Shell.Arguments()
		return &shellResult{Shell: shell, Args: args}, nil
	}

	// For string form, need to split command and args
	command := d.Shell.Command()
	if command == "" {
		return &shellResult{Shell: cmdutil.GetShellCommand(""), Args: nil}, nil
	}

	// Shell expansion is deferred to runtime - see runtime/env.go Shell()
	shell, args, err := cmdutil.SplitCommand(command)
	if err != nil {
		return nil, core.NewValidationError("shell", d.Shell.Value(), fmt.Errorf("failed to parse shell command: %w", err))
	}
	return &shellResult{Shell: strings.TrimSpace(shell), Args: args}, nil
}

func buildShell(ctx BuildContext, d *dag) (string, error) {
	result, err := parseShellInternal(ctx, d)
	if err != nil {
		return "", err
	}
	return result.Shell, nil
}

func buildShellArgs(ctx BuildContext, d *dag) ([]string, error) {
	result, err := parseShellInternal(ctx, d)
	if err != nil {
		return nil, err
	}
	return result.Args, nil
}

func buildWorkingDir(ctx BuildContext, d *dag) (string, error) {
	if d.WorkingDir != "" {
		return resolveWorkingDirPath(d.WorkingDir, ctx.file)
	}
	if ctx.opts.DefaultWorkingDir != "" {
		return ctx.opts.DefaultWorkingDir, nil
	}
	// Return empty to allow inheritance from base config.
	// Default is applied post-merge in loadDAG.
	return "", nil
}

// resolveWorkingDirPath resolves the working directory path at build time.
// Absolute paths, home dir (~), and variable ($) paths are stored as-is for runtime expansion.
// Relative paths are resolved against the DAG file location.
func resolveWorkingDirPath(wd, dagFile string) (string, error) {
	if filepath.IsAbs(wd) || strings.HasPrefix(wd, "~") || strings.HasPrefix(wd, "$") {
		return wd, nil
	}
	if dagFile != "" {
		return filepath.Join(filepath.Dir(dagFile), wd), nil
	}
	return wd, nil
}

// getDefaultWorkingDir returns the current working directory or user home as fallback.
func getDefaultWorkingDir() (string, error) {
	if dir, _ := os.Getwd(); dir != "" {
		return dir, nil
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	return dir, nil
}

func buildContainer(ctx BuildContext, d *dag) (*core.Container, error) {
	return buildContainerField(ctx, d.Container)
}

// buildContainerField handles both string and object forms of container field.
// String form: "container-name" -> exec into existing container
// Object form: {image: "...", ...} or {exec: "...", ...} -> create new or exec into existing
func buildContainerField(ctx BuildContext, raw any) (*core.Container, error) {
	if raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case string:
		// String mode: exec into existing container with defaults
		name := strings.TrimSpace(v)
		if name == "" {
			return nil, core.NewValidationError("container", nil,
				fmt.Errorf("container name cannot be empty"))
		}
		return &core.Container{
			Exec: name,
		}, nil

	case map[string]any:
		// Object mode: decode and validate
		var c container
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			Result:           &c,
			ErrorUnused:      true,
			WeaklyTypedInput: true,
			TagName:          "yaml",
		})
		if err != nil {
			return nil, core.NewValidationError("container", nil,
				fmt.Errorf("failed to create decoder: %w", err))
		}
		if err := decoder.Decode(v); err != nil {
			return nil, core.NewValidationError("container", nil,
				fmt.Errorf("failed to decode container: %w", withSnakeCaseKeyHint(err)))
		}
		return buildContainerFromSpec(ctx, &c)

	case *container:
		// Already decoded container struct (for backward compatibility)
		if v == nil {
			return nil, nil
		}
		return buildContainerFromSpec(ctx, v)

	default:
		return nil, core.NewValidationError("container", nil,
			fmt.Errorf("container must be a string or object, got %T", raw))
	}
}

// buildContainerFromSpec is a shared function that builds a core.Container from a container spec.
// It is used by both DAG-level and step-level container configuration.
func buildContainerFromSpec(_ BuildContext, c *container) (*core.Container, error) {
	// Validate mutual exclusivity
	if c.Exec != "" && c.Image != "" {
		return nil, core.NewValidationError("container", nil,
			fmt.Errorf("'exec' and 'image' are mutually exclusive"))
	}

	// Require one of exec or image
	if c.Exec == "" && c.Image == "" {
		return nil, core.NewValidationError("container", nil,
			fmt.Errorf("either 'exec' or 'image' must be specified"))
	}

	// Handle exec mode
	if c.Exec != "" {
		// Validate no incompatible fields in exec mode
		var invalidFields []string
		if c.Name != "" {
			invalidFields = append(invalidFields, "name")
		}
		if c.PullPolicy != nil {
			invalidFields = append(invalidFields, "pull_policy")
		}
		if len(c.Volumes) > 0 {
			invalidFields = append(invalidFields, "volumes")
		}
		if len(c.Ports) > 0 {
			invalidFields = append(invalidFields, "ports")
		}
		if c.Network != "" {
			invalidFields = append(invalidFields, "network")
		}
		if c.Platform != "" {
			invalidFields = append(invalidFields, "platform")
		}
		if c.Startup != "" {
			invalidFields = append(invalidFields, "startup")
		}
		if len(c.Command) > 0 {
			invalidFields = append(invalidFields, "command")
		}
		if c.WaitFor != "" {
			invalidFields = append(invalidFields, "wait_for")
		}
		if c.LogPattern != "" {
			invalidFields = append(invalidFields, "log_pattern")
		}
		if c.RestartPolicy != "" {
			invalidFields = append(invalidFields, "restart_policy")
		}
		if c.KeepContainer {
			invalidFields = append(invalidFields, "keep_container")
		}
		if c.Healthcheck != nil {
			invalidFields = append(invalidFields, "healthcheck")
		}

		if len(invalidFields) > 0 {
			return nil, core.NewValidationError("container", nil,
				fmt.Errorf("fields %v cannot be used with 'exec'", invalidFields))
		}

		// Collect raw env pairs without evaluation — evaluation is deferred
		// to runtime so that DAG-level env, params, and step outputs are in scope.
		envs, err := collectRawPairs(c.Env)
		if err != nil {
			return nil, core.NewValidationError("container.env", c.Env, err)
		}

		// Build exec-mode container
		return &core.Container{
			Exec:       strings.TrimSpace(c.Exec),
			User:       c.User,
			WorkingDir: c.WorkingDir,
			Env:        envs,
			Shell:      c.Shell,
		}, nil
	}

	// Handle image mode (existing behavior)
	pullPolicy, err := core.ParsePullPolicy(c.PullPolicy)
	if err != nil {
		return nil, core.NewValidationError("container.pull_policy", c.PullPolicy, err)
	}

	// Collect raw env pairs without evaluation — evaluation is deferred
	// to runtime so that DAG-level env, params, and step outputs are in scope.
	envs, err := collectRawPairs(c.Env)
	if err != nil {
		return nil, core.NewValidationError("container.env", c.Env, err)
	}

	// Parse healthcheck if provided
	var hc *core.Healthcheck
	if c.Healthcheck != nil {
		var err error
		hc, err = parseHealthcheck(c.Healthcheck)
		if err != nil {
			return nil, core.NewValidationError("container.healthcheck", c.Healthcheck, err)
		}
	}

	return &core.Container{
		Name:          strings.TrimSpace(c.Name),
		Image:         c.Image,
		PullPolicy:    pullPolicy,
		Env:           envs,
		Volumes:       c.Volumes,
		User:          c.User,
		WorkingDir:    c.WorkingDir,
		Platform:      c.Platform,
		Ports:         c.Ports,
		Network:       c.Network,
		KeepContainer: c.KeepContainer,
		Startup:       core.ContainerStartup(strings.ToLower(strings.TrimSpace(c.Startup))),
		Command:       c.Command,
		WaitFor:       core.ContainerWaitFor(strings.ToLower(strings.TrimSpace(c.WaitFor))),
		LogPattern:    c.LogPattern,
		RestartPolicy: strings.TrimSpace(c.RestartPolicy),
		Healthcheck:   hc,
		Shell:         c.Shell,
	}, nil
}

// parseHealthcheck converts a spec healthcheck to a core.Healthcheck with validation.
func parseHealthcheck(h *healthcheck) (*core.Healthcheck, error) {
	if h == nil {
		return nil, nil
	}

	// Validate test field
	if len(h.Test) == 0 {
		return nil, fmt.Errorf("test is required")
	}

	// First element must be a valid command type
	validPrefixes := map[string]bool{
		"NONE":      true,
		"CMD":       true,
		"CMD-SHELL": true,
	}
	if !validPrefixes[h.Test[0]] {
		return nil, fmt.Errorf("test must start with NONE, CMD, or CMD-SHELL, got %q", h.Test[0])
	}

	// NONE should be the only element
	if h.Test[0] == "NONE" && len(h.Test) > 1 {
		return nil, fmt.Errorf("NONE healthcheck should not have additional arguments")
	}

	// CMD and CMD-SHELL need at least one more element (the command)
	if (h.Test[0] == "CMD" || h.Test[0] == "CMD-SHELL") && len(h.Test) < 2 {
		return nil, fmt.Errorf("%s healthcheck requires a command", h.Test[0])
	}

	// Validate retries
	if h.Retries < 0 {
		return nil, fmt.Errorf("retries must be non-negative, got %d", h.Retries)
	}

	hc := &core.Healthcheck{
		Test:    h.Test,
		Retries: h.Retries,
	}

	// Parse duration strings
	if h.Interval != "" {
		d, err := time.ParseDuration(h.Interval)
		if err != nil {
			return nil, fmt.Errorf("invalid interval %q: %w", h.Interval, err)
		}
		hc.Interval = d
	}

	if h.Timeout != "" {
		d, err := time.ParseDuration(h.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout %q: %w", h.Timeout, err)
		}
		hc.Timeout = d
	}

	if h.StartPeriod != "" {
		d, err := time.ParseDuration(h.StartPeriod)
		if err != nil {
			return nil, fmt.Errorf("invalid start_period %q: %w", h.StartPeriod, err)
		}
		hc.StartPeriod = d
	}

	return hc, nil
}

func buildSSH(_ BuildContext, d *dag) (*core.SSHConfig, error) {
	if d.SSH == nil {
		return nil, nil
	}

	shell, shellArgs, err := parseSSHShell(d.SSH.Shell)
	if err != nil {
		return nil, err
	}

	return &core.SSHConfig{
		User:          d.SSH.User,
		Host:          d.SSH.Host,
		Port:          defaultPort(d.SSH.Port.String(), "22"),
		Key:           d.SSH.Key,
		Password:      d.SSH.Password,
		StrictHostKey: d.SSH.StrictHostKey == nil || *d.SSH.StrictHostKey,
		KnownHostFile: d.SSH.KnownHostFile,
		Shell:         shell,
		ShellArgs:     shellArgs,
		Timeout:       d.SSH.Timeout,
		Bastion:       buildBastionConfig(d.SSH.Bastion),
	}, nil
}

// parseSSHShell parses shell configuration from ShellValue.
func parseSSHShell(shellVal types.ShellValue) (string, []string, error) {
	if shellVal.IsZero() {
		return "", nil, nil
	}

	command := strings.TrimSpace(shellVal.Command())
	if command == "" {
		return "", nil, nil
	}

	if shellVal.IsArray() {
		return command, shellVal.Arguments(), nil
	}

	parsed, args, err := cmdutil.SplitCommand(command)
	if err != nil {
		return "", nil, core.NewValidationError("ssh.shell", shellVal.Value(), fmt.Errorf("failed to parse shell command: %w", err))
	}
	return strings.TrimSpace(parsed), args, nil
}

// buildBastionConfig builds bastion configuration from spec.
func buildBastionConfig(bastion *bastion) *core.BastionConfig {
	if bastion == nil {
		return nil
	}
	return &core.BastionConfig{
		Host:     bastion.Host,
		Port:     defaultPort(bastion.Port.String(), "22"),
		User:     bastion.User,
		Key:      bastion.Key,
		Password: bastion.Password,
	}
}

// defaultPort returns the port if non-empty, otherwise returns the default value.
func defaultPort(port, defaultVal string) string {
	if port == "" {
		return defaultVal
	}
	return port
}

func buildLLM(_ BuildContext, d *dag) (*core.LLMConfig, error) {
	if d.LLM == nil {
		return nil, nil
	}

	cfg := d.LLM

	// Validate provider if specified (optional at DAG level)
	if cfg.Provider != "" {
		validProviders := map[string]bool{
			"openai": true, "anthropic": true, "gemini": true,
			"openrouter": true, "local": true,
			// Aliases for local provider
			"ollama": true, "vllm": true, "llama": true,
		}
		if !validProviders[cfg.Provider] {
			return nil, core.NewValidationError("llm.provider", cfg.Provider,
				fmt.Errorf("invalid provider: must be one of openai, anthropic, gemini, openrouter, local (or aliases: ollama, vllm, llama)"))
		}
	}

	// Get model string or entries (optional at DAG level)
	var modelString string
	var models []core.ModelEntry

	if !cfg.Model.IsZero() {
		if cfg.Model.IsArray() {
			var err error
			models, err = convertModelEntries(cfg.Model.Entries())
			if err != nil {
				return nil, err
			}
		} else {
			modelString = cfg.Model.String()
		}
	}

	// Validate temperature range if specified
	if cfg.Temperature != nil {
		if *cfg.Temperature < 0.0 || *cfg.Temperature > 2.0 {
			return nil, core.NewValidationError("llm.temperature", *cfg.Temperature,
				fmt.Errorf("temperature must be between 0.0 and 2.0"))
		}
	}

	// Validate top_p range if specified
	if cfg.TopP != nil {
		if *cfg.TopP < 0.0 || *cfg.TopP > 1.0 {
			return nil, core.NewValidationError("llm.top_p", *cfg.TopP,
				fmt.Errorf("top_p must be between 0.0 and 1.0"))
		}
	}

	// Validate max_tokens if specified
	if cfg.MaxTokens != nil {
		if *cfg.MaxTokens < 1 {
			return nil, core.NewValidationError("llm.max_tokens", *cfg.MaxTokens,
				fmt.Errorf("max_tokens must be at least 1"))
		}
	}

	thinking, err := buildThinkingConfig(cfg.Thinking)
	if err != nil {
		return nil, err
	}

	return &core.LLMConfig{
		Provider:    cfg.Provider,
		Model:       modelString,
		Models:      models,
		System:      cfg.System,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
		TopP:        cfg.TopP,
		BaseURL:     cfg.BaseURL,
		APIKeyName:  cfg.APIKeyName,
		Stream:      cfg.Stream,
		Thinking:    thinking,
	}, nil
}

func buildSecrets(_ BuildContext, d *dag) ([]core.SecretRef, error) {
	if len(d.Secrets) == 0 {
		return nil, nil
	}
	return parseSecretRefs(d.Secrets)
}

func buildDotenv(_ BuildContext, d *dag) ([]string, error) {
	if d.Dotenv.IsZero() {
		return []string{".env"}, nil
	}
	return d.Dotenv.Values(), nil
}

func buildHandlers(ctx BuildContext, d *dag, result *core.DAG) (core.HandlerOn, error) {
	buildCtx := StepBuildContext{BuildContext: ctx, dag: result}
	var handlerOn core.HandlerOn

	defs, err := decodeDefaults(d.Defaults)
	if err != nil {
		return handlerOn, err
	}

	// buildHandler is a helper that builds a single handler step.
	buildHandler := func(s *step, name core.HandlerType) (*core.Step, error) {
		if s == nil {
			return nil, nil
		}
		s.Name = name.String()
		applyDefaults(s, defs, nil)
		return s.build(buildCtx)
	}

	if handlerOn.Init, err = buildHandler(d.HandlerOn.Init, core.HandlerOnInit); err != nil {
		return handlerOn, err
	}
	if handlerOn.Exit, err = buildHandler(d.HandlerOn.Exit, core.HandlerOnExit); err != nil {
		return handlerOn, err
	}
	if handlerOn.Success, err = buildHandler(d.HandlerOn.Success, core.HandlerOnSuccess); err != nil {
		return handlerOn, err
	}
	if handlerOn.Failure, err = buildHandler(d.HandlerOn.Failure, core.HandlerOnFailure); err != nil {
		return handlerOn, err
	}

	if handlerOn.Abort, err = buildHandler(d.HandlerOn.Abort, core.HandlerOnAbort); err != nil {
		return handlerOn, err
	}

	if handlerOn.Wait, err = buildHandler(d.HandlerOn.Wait, core.HandlerOnWait); err != nil {
		return handlerOn, err
	}

	return handlerOn, nil
}

func buildMailOn(_ BuildContext, d *dag) (*core.MailOn, error) {
	if d.MailOn == nil {
		return nil, nil
	}
	return &core.MailOn{
		Failure: d.MailOn.Failure,
		Success: d.MailOn.Success,
		Wait:    d.MailOn.Wait,
	}, nil
}

func buildSMTPConfig(_ BuildContext, d *dag) (*core.SMTPConfig, error) {
	if d.SMTP.IsZero() {
		return nil, nil
	}

	return &core.SMTPConfig{
		Host:     d.SMTP.Host,
		Port:     d.SMTP.Port.String(),
		Username: d.SMTP.Username,
		Password: d.SMTP.Password,
	}, nil
}

func buildErrMailConfig(_ BuildContext, d *dag) (*core.MailConfig, error) {
	return buildMailConfigInternal(d.ErrorMail)
}

func buildInfoMailConfig(_ BuildContext, d *dag) (*core.MailConfig, error) {
	return buildMailConfigInternal(d.InfoMail)
}

func buildWaitMailConfig(_ BuildContext, d *dag) (*core.MailConfig, error) {
	return buildMailConfigInternal(d.WaitMail)
}

func buildPreconditions(ctx BuildContext, d *dag) ([]*core.Condition, error) {
	return parsePrecondition(ctx, d.Preconditions)
}

func buildOTel(_ BuildContext, d *dag) (*core.OTelConfig, error) {
	if d.OTel == nil {
		return nil, nil
	}

	switch v := d.OTel.(type) {
	case map[string]any:
		config := &core.OTelConfig{}

		if enabled, ok := v["enabled"].(bool); ok {
			config.Enabled = enabled
		}
		if endpoint, ok := v["endpoint"].(string); ok {
			config.Endpoint = endpoint
		}
		if headers, ok := v["headers"].(map[string]any); ok {
			config.Headers = make(map[string]string)
			for key, val := range headers {
				if strVal, ok := val.(string); ok {
					config.Headers[key] = strVal
				}
			}
		}
		if insecure, ok := v["insecure"].(bool); ok {
			config.Insecure = insecure
		}
		if timeout, ok := v["timeout"].(string); ok {
			duration, err := time.ParseDuration(timeout)
			if err != nil {
				return nil, core.NewValidationError("otel.timeout", timeout, err)
			}
			config.Timeout = duration
		}
		if resource, ok := v["resource"].(map[string]any); ok {
			config.Resource = resource
		}

		return config, nil

	default:
		return nil, core.NewValidationError("otel", v, fmt.Errorf("otel must be a map"))
	}
}

func buildSteps(ctx BuildContext, d *dag, result *core.DAG) ([]core.Step, error) {
	buildCtx := StepBuildContext{BuildContext: ctx, dag: result}
	names := make(map[string]struct{})

	defs, err := decodeDefaults(d.Defaults)
	if err != nil {
		return nil, err
	}

	switch v := d.Steps.(type) {
	case nil:
		return nil, nil

	case []any:
		normalized := normalizeStepData(ctx, v)

		var builtSteps []*core.Step
		for i, raw := range normalized {
			switch v := raw.(type) {
			case map[string]any:
				st, err := buildStepFromRaw(buildCtx, i, v, names, defs)
				if err != nil {
					return nil, err
				}
				builtSteps = append(builtSteps, st)

			case []any:
				var normalizedNested = normalizeStepData(ctx, v)
				for _, nested := range normalizedNested {
					switch vv := nested.(type) {
					case map[string]any:
						st, err := buildStepFromRaw(buildCtx, i, vv, names, defs)
						if err != nil {
							return nil, err
						}
						builtSteps = append(builtSteps, st)

					default:
						return nil, core.NewValidationError("steps", raw, ErrInvalidStepData)
					}
				}

			default:
				return nil, core.NewValidationError("steps", raw, ErrInvalidStepData)
			}
		}

		var steps []core.Step
		for _, st := range builtSteps {
			steps = append(steps, *st)
		}
		// Transform router steps: inject preconditions into targets
		if err := transformRouterSteps(steps); err != nil {
			return nil, err
		}
		return steps, nil

	case map[string]any:
		stepsMap := make(map[string]step)
		md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			ErrorUnused: true,
			Result:      &stepsMap,
			DecodeHook:  TypedUnionDecodeHook(),
		})
		if err := md.Decode(v); err != nil {
			return nil, core.NewValidationError("steps", v, err)
		}

		var steps []core.Step
		for name, st := range stepsMap {
			st.Name = name
			names[st.Name] = struct{}{}
			rawStep, _ := v[name].(map[string]any)
			applyDefaults(&st, defs, rawStep)
			builtStep, err := st.build(buildCtx)
			if err != nil {
				return nil, err
			}
			steps = append(steps, *builtStep)
		}
		// Sort steps by name for deterministic output when built from a map.
		// Go map iteration is non-deterministic, which causes the SSE watcher's
		// JSON hash to change on every poll, triggering unnecessary broadcasts.
		slices.SortFunc(steps, func(a, b core.Step) int {
			return strings.Compare(a.Name, b.Name)
		})
		// Transform router steps: inject preconditions into targets
		if err := transformRouterSteps(steps); err != nil {
			return nil, err
		}
		return steps, nil

	default:
		return nil, core.NewValidationError("steps", v, ErrStepsMustBeArrayOrMap)
	}
}

// buildMailConfigInternal builds a core.MailConfig from the mail configuration.
func buildMailConfigInternal(def mailConfig) (*core.MailConfig, error) {
	if def.IsZero() {
		return nil, nil
	}

	// StringOrArray already parsed during YAML unmarshal
	rawAddresses := def.To.Values()

	// Trim whitespace and filter out empty entries
	var toAddresses []string
	for _, addr := range rawAddresses {
		trimmed := strings.TrimSpace(addr)
		if trimmed != "" {
			toAddresses = append(toAddresses, trimmed)
		}
	}

	return &core.MailConfig{
		From:       strings.TrimSpace(def.From),
		To:         toAddresses,
		Prefix:     strings.TrimSpace(def.Prefix),
		AttachLogs: def.AttachLogs,
	}, nil
}

// transformRouterSteps processes router-type steps and injects preconditions
// into their target steps. It modifies the steps slice in place.
func transformRouterSteps(steps []core.Step) error {
	// Build step index for lookup (using pointers to modify in place)
	stepIndex := make(map[string]*core.Step)
	for i := range steps {
		stepIndex[steps[i].Name] = &steps[i]
	}

	for i := range steps {
		if steps[i].Router == nil {
			continue
		}

		router := steps[i].Router
		routerName := steps[i].Name

		// Track targets to detect duplicates across routes
		seenTargets := make(map[string]string) // target -> first pattern that used it

		// For each route, inject precondition into target steps
		for _, route := range router.Routes {
			for _, targetName := range route.Targets {
				// Check for duplicate target
				if firstPattern, exists := seenTargets[targetName]; exists {
					return core.NewValidationError("routes", targetName,
						fmt.Errorf("router %q: step %q is targeted by multiple routes (%q and %q); each step can only be a target of one route",
							routerName, targetName, firstPattern, route.Pattern))
				}
				seenTargets[targetName] = route.Pattern

				target, ok := stepIndex[targetName]
				if !ok {
					return core.NewValidationError("routes", targetName,
						fmt.Errorf("router %q references non-existent step %q", routerName, targetName))
				}

				// Inject precondition: check if value matches pattern
				condition := &core.Condition{
					Condition: router.Value,
					Expected:  route.Pattern,
				}
				target.Preconditions = append(target.Preconditions, condition)

				// Add router as dependency if not already present
				if !slices.Contains(target.Depends, routerName) {
					target.Depends = append(target.Depends, routerName)
				}

				// Enable continueOn.skipped for proper flow
				target.ContinueOn.Skipped = true
			}
		}

		// Router itself allows downstream to continue
		steps[i].ContinueOn.Skipped = true
	}

	return nil
}
