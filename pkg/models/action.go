package models

const (
	ActionNothing   = ""
	ActionPending   = "pending"
	ActionPreparing = "preparing"
	ActionPrepared  = "prepared"
	ActionMigrating = "migrating"
	ActionFinished  = "finished"
	ActionSyncing   = "syncing"
	ActionSyncError = "syncerror"
)
