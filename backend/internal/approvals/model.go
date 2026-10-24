package approvals

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Request statuses. PENDING waits for steps; APPROVED → EXECUTING → EXECUTED
// or FAILED once the last step is met; the rest are final without running.
const (
	StatusPending   = "PENDING"
	StatusApproved  = "APPROVED"
	StatusExecuting = "EXECUTING"
	StatusExecuted  = "EXECUTED"
	StatusFailed    = "FAILED"
	StatusRejected  = "REJECTED"
	StatusExpired   = "EXPIRED"
	StatusCancelled = "CANCELLED"
)

// Decisions an approver can take.
const (
	DecisionApprove = "APPROVE"
	DecisionReject  = "REJECT"
)

// Step modes: ANY needs `approvals` people holding any listed role (or the
// permission); ALL needs one distinct person for every listed role (and one
// holding the permission when it is set).
const (
	ModeAny = "ANY"
	ModeAll = "ALL"
)

// ConditionAmount is the only numeric condition field an action can offer today.
const ConditionAmount = "amount"

// Executor performs the real change from a stored payload, as the requester
// of record. The returned value is stored on the request as its result.
type Executor func(ctx context.Context, x Execution) (any, error)

// Describer turns a payload into the readable summary approvers see. It may
// refuse the change up front (e.g. a refund larger than what is left).
type Describer func(ctx context.Context, payload json.RawMessage) (Summary, error)

// Action is one approvable action in the catalogue (registered from the wiring).
type Action struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Group       string `json:"group"`
	// Permission is what the requester must hold to ask for the change at all.
	Permission string `json:"permission"`
	// Condition names the numeric field conditions may test ("amount"), or "".
	Condition string `json:"condition,omitempty"`
	// Scoped actions belong to a country, so a country condition applies.
	Scoped   bool      `json:"scoped"`
	Execute  Executor  `json:"-"`
	Describe Describer `json:"-"`
}

// Execution is what an executor receives.
type Execution struct {
	RequestID   string
	RequesterID string
	// Reason is the requester's reason with "(approved by …)" appended, so the
	// action's own audit entry names the approvers.
	Reason  string
	Payload json.RawMessage
}

// Change is one line of a readable diff.
type Change struct {
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// Summary is what approvers read: a sentence and the changed fields.
type Summary struct {
	Title   string   `json:"title"`
	Changes []Change `json:"changes"`
}

// Conditions narrow when a workflow applies; empty = always.
type Conditions struct {
	AmountMin *decimal.Decimal `json:"amount_min,omitempty"` // amount ≥ this
	AmountMax *decimal.Decimal `json:"amount_max,omitempty"` // amount ≤ this
	Countries []string         `json:"countries,omitempty"`  // request country is one of these
}

// Step is one approval stage; steps run in order.
type Step struct {
	Name       string   `json:"name"`
	Mode       string   `json:"mode"`
	Roles      []string `json:"roles"`
	Permission string   `json:"permission,omitempty"`
	Approvals  int      `json:"approvals"`
}

// Definition is a workflow's editable content.
type Definition struct {
	Conditions  Conditions `json:"conditions"`
	Steps       []Step     `json:"steps"`
	ExpireHours int        `json:"expire_hours"`
}

// Workflow is one action's workflow as stored (version 0 = never saved).
type Workflow struct {
	Action     string     `json:"action"`
	Enabled    bool       `json:"enabled"`
	Definition Definition `json:"definition"`
	Version    int        `json:"version"`
	UpdatedBy  *string    `json:"updated_by"`
	UpdatedAt  *time.Time `json:"updated_at"`
}

// WorkflowVersion is one saved definition in a workflow's history.
type WorkflowVersion struct {
	Version    int        `json:"version"`
	Enabled    bool       `json:"enabled"`
	Definition Definition `json:"definition"`
	CreatedBy  *string    `json:"created_by"`
	ByName     *string    `json:"by_name"`
	Reason     string     `json:"reason"`
	CreatedAt  time.Time  `json:"created_at"`
}

// WorkflowInput is what the editor saves.
type WorkflowInput struct {
	Enabled    bool       `json:"enabled"`
	Definition Definition `json:"definition"`
	Reason     string     `json:"reason"`
}

// Submission is a gated endpoint's change, offered to its workflow.
type Submission struct {
	Action      string
	RequesterID string
	Reason      string
	Country     string           // "" = not country-bound
	Amount      *decimal.Decimal // for amount conditions
	Target      string           // "trip:<id>", "payout:<id>"… for the audit and the inbox
	Payload     any
}

// Person is a requester or approver with a display name.
type Person struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Decision is one recorded approval or rejection.
type Decision struct {
	Step     int       `json:"step"`
	Approver Person    `json:"approver"`
	Decision string    `json:"decision"`
	AsRole   string    `json:"as_role"`
	Note     string    `json:"note"`
	At       time.Time `json:"at"`
}

// Request is one held-back change.
type Request struct {
	ID              string           `json:"id"`
	Action          string           `json:"action"`
	ActionLabel     string           `json:"action_label"`
	Status          string           `json:"status"`
	Requester       Person           `json:"requester"`
	Reason          string           `json:"reason"`
	Country         *string          `json:"country"`
	Amount          *decimal.Decimal `json:"amount"`
	Target          string           `json:"target"`
	Summary         Summary          `json:"summary"`
	Payload         json.RawMessage  `json:"payload"`
	Workflow        Definition       `json:"workflow"`
	WorkflowVersion int              `json:"workflow_version"`
	CurrentStep     int              `json:"current_step"`
	Decisions       []Decision       `json:"decisions"`
	CanDecide       bool             `json:"can_decide"`
	ExpiresAt       time.Time        `json:"expires_at"`
	CreatedAt       time.Time        `json:"created_at"`
	DecidedAt       *time.Time       `json:"decided_at"`
	ExecutedAt      *time.Time       `json:"executed_at"`
	Result          json.RawMessage  `json:"result,omitempty"`
	Error           *string          `json:"error"`
}

// Viewer is the staff member looking at or deciding requests.
type Viewer struct {
	ID string
	// Countries limits what they see; empty = every country (and country-less requests).
	Countries []string
}

// Grants are a person's staff roles and permissions, read fresh for a decision.
type Grants struct {
	Roles       []string
	Permissions []string
}

// Inbox boxes.
const (
	BoxPending = "pending" // waiting, in the viewer's countries
	BoxMine    = "mine"    // asked for by the viewer
	BoxDone    = "done"    // finished, in the viewer's countries
)

// Errors (codes in i18n).
var (
	ErrNotFound        = apperr.New("APPROVAL_NOT_FOUND", http.StatusNotFound, "")
	ErrUnknownAction   = apperr.New("APPROVAL_UNKNOWN_ACTION", http.StatusNotFound, "")
	ErrNotPending      = apperr.New("APPROVAL_NOT_PENDING", http.StatusConflict, "")
	ErrExpired         = apperr.New("APPROVAL_EXPIRED", http.StatusConflict, "")
	ErrSelfApproval    = apperr.New("APPROVAL_SELF", http.StatusForbidden, "")
	ErrNotApprover     = apperr.New("APPROVAL_NOT_APPROVER", http.StatusForbidden, "")
	ErrAlreadyDecided  = apperr.New("APPROVAL_ALREADY_DECIDED", http.StatusConflict, "")
	ErrNotRequester    = apperr.New("APPROVAL_NOT_REQUESTER", http.StatusForbidden, "")
	ErrNoteRequired    = apperr.New("APPROVAL_NOTE_REQUIRED", http.StatusBadRequest, "")
	ErrReasonRequired  = apperr.New("REASON_REQUIRED", http.StatusBadRequest, "")
	ErrWorkflowInvalid = apperr.New("APPROVAL_WORKFLOW_INVALID", http.StatusBadRequest, "")
)
