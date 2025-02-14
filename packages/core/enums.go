package core

type Role string

const (
	RoleAdmin      Role = "admin"
	RoleDispatcher Role = "dispatcher"
)

func ParseRole(s string) (Role, bool) {
	switch Role(s) {
	case RoleAdmin, RoleDispatcher:
		return Role(s), true
	}
	return "", false
}

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityUrgent Priority = "urgent"
)

func ParsePriority(s string) (Priority, bool) {
	switch Priority(s) {
	case PriorityLow, PriorityNormal, PriorityUrgent:
		return Priority(s), true
	}
	return "", false
}

type JobStatus string

const (
	StatusRequested  JobStatus = "requested"
	StatusScheduled  JobStatus = "scheduled"
	StatusInProgress JobStatus = "in_progress"
	StatusCompleted  JobStatus = "completed"
	StatusInvoiced   JobStatus = "invoiced"
	StatusCancelled  JobStatus = "cancelled"
)

var JobStatuses = []JobStatus{
	StatusRequested, StatusScheduled, StatusInProgress,
	StatusCompleted, StatusInvoiced, StatusCancelled,
}

func ParseJobStatus(s string) (JobStatus, bool) {
	for _, st := range JobStatuses {
		if JobStatus(s) == st {
			return st, true
		}
	}
	return "", false
}

type LineKind string

const (
	LineLabor LineKind = "labor"
	LinePart  LineKind = "part"
)
