// Package sftpx implements SFTP browsing and managed file transfers.
package sftpx

import (
	"regexp"
)

var partialPattern = regexp.MustCompile(`\.svpart-[0-9a-f]{8}$`)

type TransferState string

const (
	TransferQueued    TransferState = "queued"
	TransferRunning   TransferState = "running"
	TransferCompleted TransferState = "completed"
	TransferCancelled TransferState = "cancelled"
	TransferFailed    TransferState = "failed"
	TransferSkipped   TransferState = "skipped"
)

type ConflictChoice string

const (
	ConflictReplace   ConflictChoice = "replace"
	ConflictSkip      ConflictChoice = "skip"
	ConflictCancelAll ConflictChoice = "cancelAll"
)

type TransferRef struct {
	ID    string
	State TransferState
}

func PartialName(name, transferID string) string {
	if len(transferID) > 8 {
		transferID = transferID[:8]
	}
	return name + ".svpart-" + transferID
}

func IsPartialName(name string) bool {
	return partialPattern.MatchString(name)
}

func CanTransition(from, to TransferState) bool {
	switch from {
	case TransferQueued:
		return to == TransferRunning || to == TransferCancelled || to == TransferSkipped || to == TransferFailed
	case TransferRunning:
		return to == TransferCompleted || to == TransferCancelled || to == TransferFailed
	default:
		return false
	}
}

func PlanConflictChoice(batch []TransferRef, transferID string, choice ConflictChoice) map[string]TransferState {
	plan := make(map[string]TransferState)
	switch choice {
	case ConflictSkip:
		plan[transferID] = TransferSkipped
	case ConflictCancelAll:
		for _, transfer := range batch {
			if transfer.ID == transferID || transfer.State == TransferQueued {
				plan[transfer.ID] = TransferCancelled
			}
		}
	}
	return plan
}
