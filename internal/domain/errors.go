package domain

import (
	"errors"
	"fmt"
)

// ExitCode is the process exit status defined in section 7 of the design.
type ExitCode int

// Exit codes of gh board. main.go maps the command error to one of them.
const (
	ExitOK           ExitCode = 0 // success
	ExitUsage        ExitCode = 1 // usage error
	ExitNotFound     ExitCode = 2 // target not found or ambiguous
	ExitPolicy       ExitCode = 3 // refused by policy: transition, WIP, permission
	ExitDrift        ExitCode = 4 // a precondition drifted since it was read
	ExitPlanRequired ExitCode = 5 // the operation needs a plan and apply
	ExitApplyRefused ExitCode = 6 // apply refused: no terminal, expired, hash, actor
	ExitAPI          ExitCode = 7 // GitHub API error
)

// String names the exit code for messages and tests.
func (c ExitCode) String() string {
	switch c {
	case ExitOK:
		return "ok"
	case ExitUsage:
		return "usage"
	case ExitNotFound:
		return "not-found"
	case ExitPolicy:
		return "policy"
	case ExitDrift:
		return "drift"
	case ExitPlanRequired:
		return "plan-required"
	case ExitApplyRefused:
		return "apply-refused"
	case ExitAPI:
		return "api"
	default:
		return fmt.Sprintf("exit-%d", int(c))
	}
}

// Sentinel errors. Wrap them with fmt.Errorf("...: %w", Err...) so that
// CodeOf can map the chain to an exit code.
var (
	// ErrUsage marks a malformed argument or flag.
	ErrUsage = errors.New("usage error")
	// ErrNotFound marks a target that does not exist on the board.
	ErrNotFound = errors.New("target not found")
	// ErrAmbiguous marks a short reference that matches more than one item.
	ErrAmbiguous = errors.New("target is ambiguous")
	// ErrPolicy marks a write refused by a transition, WIP or permission rule.
	ErrPolicy = errors.New("refused by policy")
	// ErrDrift marks a precondition whose current value differs from the one read.
	ErrDrift = errors.New("drift detected")
	// ErrPlanRequired marks an operation that must go through plan and apply.
	ErrPlanRequired = errors.New("plan required")
	// ErrApplyRefused marks an apply refused before any write.
	ErrApplyRefused = errors.New("apply refused")
	// ErrAPI marks a failure reported by the GitHub API or the transport.
	ErrAPI = errors.New("github api error")
	// ErrNotImplemented is returned by every skeleton stub a builder replaces.
	ErrNotImplemented = errors.New("not implemented")
)

// Error carries an exit code together with its cause.
type Error struct {
	Code ExitCode
	Err  error
}

// NewError wraps err with an explicit exit code.
func NewError(code ExitCode, err error) *Error {
	return &Error{Code: code, Err: err}
}

// Errorf builds an Error with a formatted message; %w is honored.
func Errorf(code ExitCode, format string, args ...any) *Error {
	return &Error{Code: code, Err: fmt.Errorf(format, args...)}
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Err == nil {
		return e.Code.String()
	}
	return e.Err.Error()
}

// Unwrap exposes the cause to errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.Err }

// ExitCode returns the code carried by the error.
func (e *Error) ExitCode() ExitCode { return e.Code }

// sentinelCodes maps each sentinel to its exit code, in precedence order.
var sentinelCodes = []struct {
	err  error
	code ExitCode
}{
	{ErrApplyRefused, ExitApplyRefused},
	{ErrPlanRequired, ExitPlanRequired},
	{ErrDrift, ExitDrift},
	{ErrPolicy, ExitPolicy},
	{ErrNotFound, ExitNotFound},
	{ErrAmbiguous, ExitNotFound},
	{ErrAPI, ExitAPI},
	{ErrUsage, ExitUsage},
}

// CodeOf maps an error chain to the exit code of section 7: nil is 0, an
// explicit *Error wins, then the sentinel found in the chain, otherwise 1.
func CodeOf(err error) ExitCode {
	if err == nil {
		return ExitOK
	}
	var coded *Error
	if errors.As(err, &coded) {
		return coded.Code
	}
	for _, s := range sentinelCodes {
		if errors.Is(err, s.err) {
			return s.code
		}
	}
	return ExitUsage
}
