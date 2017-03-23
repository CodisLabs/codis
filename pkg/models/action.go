// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

<<<<<<< HEAD
import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/wandoulabs/go-zookeeper/zk"
	"github.com/wandoulabs/zkhelper"
)

type ActionType string

const (
	ACTION_TYPE_SERVER_GROUP_CHANGED ActionType = "group_changed"
	ACTION_TYPE_SERVER_GROUP_REMOVE  ActionType = "group_remove"
	ACTION_TYPE_SLOT_CHANGED         ActionType = "slot_changed"
	ACTION_TYPE_MULTI_SLOT_CHANGED   ActionType = "multi_slot_changed"
	ACTION_TYPE_SLOT_MIGRATE         ActionType = "slot_migrate"
	ACTION_TYPE_SLOT_PREMIGRATE      ActionType = "slot_premigrate"
)

=======
>>>>>>> CodisLabs/release3.1
const (
	ActionNothing   = ""
	ActionPending   = "pending"
	ActionPreparing = "preparing"
	ActionPrepared  = "prepared"
	ActionMigrating = "migrating"
	ActionFinished  = "finished"
	ActionSyncing   = "syncing"
)
