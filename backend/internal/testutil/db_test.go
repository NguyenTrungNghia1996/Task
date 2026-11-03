package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupTestDB(t *testing.T) {
	db := SetupTestDB(t)
	assert.NotNil(t, db)

	org, user := CreateTestOrgAndUser(t, db, "admin", "system_admin")
	assert.NotEmpty(t, org.ID)
	assert.NotEmpty(t, user.ID)
	assert.Equal(t, "admin", user.Username)
}

