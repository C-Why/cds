package group

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-gorp/gorp"

	"github.com/ovh/cds/sdk"
	"github.com/ovh/cds/sdk/log"
)

// SharedInfraGroup is the group used to share infrastructure between projects
var (
	SharedInfraGroup *sdk.Group
	DefaultGroup     *sdk.Group
)

// CreateDefaultGroup creates a group 'public' where every user will be
func CreateDefaultGroup(db *gorp.DbMap, groupName string) error {
	query := `SELECT id FROM "group" where name = $1`
	var id int64
	if err := db.QueryRow(query, groupName).Scan(&id); err == sql.ErrNoRows {
		log.Debug("CreateDefaultGroup> create %s group in DB", groupName)
		query = `INSERT INTO "group" (name) VALUES ($1)`
		if _, err := db.Exec(query, groupName); err != nil {
			return err
		}
	}
	return nil
}

// AddAdminInGlobalGroup insert into new admin into global group as group admin
func AddAdminInGlobalGroup(db gorp.SqlExecutor, userID int64) error {
	query := `SELECT id FROM "group" where name = $1`
	var id int64
	if err := db.QueryRow(query, sdk.SharedInfraGroupName).Scan(&id); err != nil {
		return err
	}

	query = `INSERT INTO group_user (group_id, user_id, group_admin) VALUES ($1, $2, true)`
	if _, err := db.Exec(query, id, userID); err != nil {
		return err
	}
	return nil
}

// InitializeDefaultGroupName initializes sharedInfraGroup and Default Group
func InitializeDefaultGroupName(db gorp.SqlExecutor, defaultGrpName string) error {
	//Load the famous sharedInfraGroup
	var err error
	SharedInfraGroup, err = LoadByName(context.Background(), db, sdk.SharedInfraGroupName)
	if err != nil {
		return sdk.WrapError(err, "group.InitializeDefaultGroupName> Cannot load shared infra group")
	}

	if defaultGrpName != "" {
		DefaultGroup, err = LoadByName(context.Background(), db, defaultGrpName)
		if err != nil {
			return sdk.WrapError(err, "group.InitializeDefaultGroupName> Cannot load %s group", defaultGrpName)
		}
	}

	return nil
}

// IsDefaultGroupName returns true if groupName is the defaultGroupName
func IsDefaultGroupName(groupName string) bool {
	if DefaultGroup == nil {
		return false
	}
	return groupName == DefaultGroup.Name
}

// IsDefaultGroupID returns true if groupID is the defaultGroupID
func IsDefaultGroupID(groupID int64) bool {
	if DefaultGroup == nil {
		return false
	}
	return groupID == DefaultGroup.ID
}

// GetIDByNameInList find id related to the group name in a given group list
func GetIDByNameInList(groups []sdk.GroupPermission, groupName string) (int64, error) {
	for _, gr := range groups {
		if gr.Group.Name == groupName {
			return gr.Group.ID, nil
		}
	}

	return 0, fmt.Errorf("GetIDByNameInList> this group %s doesn't exist in this list", groupName)
}
