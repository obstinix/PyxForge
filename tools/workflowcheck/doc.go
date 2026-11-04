// Package workflowcheck holds tests for the repository's GitHub Actions workflows. They run
// with the rest of the suite (go test ./...), so a workflow change that weakens a supply-chain
// rule fails CI before it is merged.
package workflowcheck
