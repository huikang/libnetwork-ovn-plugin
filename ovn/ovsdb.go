package ovn

import (
	"errors"
	"fmt"

	"github.com/socketplane/libovsdb"
)

func (ovnnber *ovnnber) bridgeExists(portName string) (bool, error) {
	condition := libovsdb.NewCondition("name", "==", portName)
	selectOp := libovsdb.Operation{
		Op:    "select",
		Table: "Logical_Switch",
		Where: []interface{}{condition},
	}
	operations := []libovsdb.Operation{selectOp}
	reply, _ := ovnnber.ovsdb.Transact("OVN_Northbound", operations...)

	if len(reply) < len(operations) {
		return false, errors.New("Number of Replies should be atleast equal to number of Operations")
	}

	if reply[0].Error != "" {
		errMsg := fmt.Sprintf("Transaction Failed due to an error: %v", reply[0].Error)
		return false, errors.New(errMsg)
	}

	if len(reply[0].Rows) == 0 {
		return false, nil
	}
	return true, nil
}
